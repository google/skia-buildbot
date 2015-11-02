package repo_manager

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/util"
)

const (
	DEPS_ROLL_BRANCH = "skia_roll"

	REPO_CHROMIUM = "https://chromium.googlesource.com/chromium/src.git"
	REPO_SKIA     = "https://skia.googlesource.com/skia.git"

	TMPL_CQ_INCLUDE_TRYBOTS = "CQ_INCLUDE_TRYBOTS=%s"
)

var (
	ISSUE_CREATED_REGEX = regexp.MustCompile(fmt.Sprintf("Issue created. URL: %s/(\\d+)", autoroll.RIETVELD_URL))
)

// RepoManager is a struct used by AutoRoller for managing checkouts.
type RepoManager struct {
	chromiumDir       string
	chromiumParentDir string
	lastRollRev       string
	mtx               sync.RWMutex
	skiaDir           string
	skiaHead          string
	skiaRepo          *gitinfo.GitInfo
}

// NewRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency. The cqExtraTrybots and
// emails lists are used when uploading roll CLs and may be changed through
// their respective setters.
func NewRepoManager(workdir string, frequency time.Duration) (*RepoManager, error) {
	chromiumParentDir := path.Join(workdir, "chromium")
	skiaDir := path.Join(workdir, "skia")
	skiaRepo, err := gitinfo.CloneOrUpdate(REPO_SKIA, skiaDir, true)
	if err != nil {
		return nil, err
	}
	r := &RepoManager{
		chromiumDir:       path.Join(chromiumParentDir, "src"),
		chromiumParentDir: chromiumParentDir,
		skiaDir:           skiaDir,
		skiaRepo:          skiaRepo,
	}
	if err := r.update(); err != nil {
		return nil, err
	}
	go func() {
		for _ = range time.Tick(frequency) {
			util.LogErr(r.update())
		}
	}()
	return r, nil
}

// update syncs code in the relevant repositories.
func (r *RepoManager) update() error {
	// Sync the projects.
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if err := r.skiaRepo.Update(true, true); err != nil {
		return err
	}

	if _, err := os.Stat(path.Join(r.chromiumDir, ".git")); err == nil {
		if err := r.cleanChromium(); err != nil {
			return err
		}
	}

	if _, err := exec.RunCwd(r.chromiumParentDir, "gclient", "config", REPO_CHROMIUM); err != nil {
		return err
	}
	if _, err := exec.RunCwd(r.chromiumParentDir, "gclient", "sync", "--nohooks"); err != nil {
		return err
	}

	// Get the last roll revision.
	lastRollRev, err := r.getLastRollRev()
	if err != nil {
		return err
	}
	r.lastRollRev = lastRollRev

	// Record Skia HEAD
	skiaHead, err := r.skiaRepo.FullHash("origin/master")
	if err != nil {
		return err
	}
	r.skiaHead = skiaHead
	return nil
}

// getLastRollRev returns the commit hash of the last-completed DEPS roll.
func (r *RepoManager) getLastRollRev() (string, error) {
	output, err := exec.RunCwd(r.chromiumDir, "gclient", "revinfo")
	if err != nil {
		return "", err
	}
	split := strings.Split(output, "\n")
	for _, s := range split {
		if strings.HasPrefix(s, "src/third_party/skia") {
			subs := strings.Split(s, "@")
			if len(subs) != 2 {
				return "", fmt.Errorf("Failed to parse output of `gclient revinfo`")
			}
			return subs[1], nil
		}
	}
	return "", fmt.Errorf("Failed to parse output of `gclient revinfo`")
}

// FullSkiaHash returns the full hash of the given short hash or ref in the
// Skia repo.
func (r *RepoManager) FullSkiaHash(shortHash string) (string, error) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.skiaRepo.FullHash(shortHash)
}

// LastRollRev returns the last-rolled Skia commit.
func (r *RepoManager) LastRollRev() string {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.lastRollRev
}

// RolledPast determines whether DEPS has rolled past the given commit.
func (r *RepoManager) RolledPast(hash string) bool {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	if _, err := exec.RunCwd(r.skiaDir, "git", "merge-base", "--is-ancestor", hash, r.lastRollRev); err != nil {
		return false
	}
	return true
}

// SkiaHead returns the current Skia origin/master branch head.
func (r *RepoManager) SkiaHead() string {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.skiaHead
}

// cleanChromium forces the Chromium checkout into a clean state.
func (r *RepoManager) cleanChromium() error {
	if _, err := exec.RunCwd(r.chromiumDir, "git", "clean", "-d", "-f", "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(r.chromiumDir, "git", "rebase", "--abort")
	if _, err := exec.RunCwd(r.chromiumDir, "git", "checkout", "origin/master", "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(r.chromiumDir, "git", "branch", "-D", DEPS_ROLL_BRANCH)
	if _, err := exec.RunCwd(r.chromiumDir, "gclient", "revert", "--nohooks"); err != nil {
		return err
	}
	return nil
}

// CreateNewRoll creates and uploads a new DEPS roll to the given commit.
// Returns the issue number of the uploaded roll.
func (r *RepoManager) CreateNewRoll(emails, cqExtraTrybots []string, dryRun bool) (int64, error) {
	to := r.SkiaHead()

	// Clean the checkout, get onto a fresh branch.
	if err := r.cleanChromium(); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(r.chromiumDir, "git", "checkout", "-b", DEPS_ROLL_BRANCH, "-t", "origin/master", "-f"); err != nil {
		return 0, err
	}

	// Defer some more cleanup.
	defer func() {
		util.LogErr(r.cleanChromium())
	}()

	// Create the roll CL.
	if _, err := exec.RunCwd(r.chromiumDir, "roll-dep", "src/third_party/skia", to); err != nil {
		return 0, err
	}
	// Build the commit message, starting with the message provided by roll-dep.
	commitMsg, err := exec.RunCwd(r.chromiumDir, "git", "log", "-n1", "--format=%B", "HEAD")
	if err != nil {
		return 0, err
	}
	if cqExtraTrybots != nil && len(cqExtraTrybots) > 0 {
		commitMsg += "\n" + fmt.Sprintf(TMPL_CQ_INCLUDE_TRYBOTS, strings.Join(cqExtraTrybots, ","))
	}
	uploadCmd := []string{"git", "cl", "upload", "--bypass-hooks", "-f"}
	if dryRun {
		uploadCmd = append(uploadCmd, "--cq-dry-run")
	} else {
		uploadCmd = append(uploadCmd, "--use-commit-queue")
	}
	tbr := "\nTBR="
	if emails != nil && len(emails) > 0 {
		emailStr := strings.Join(emails, ",")
		tbr += emailStr
		uploadCmd = append(uploadCmd, "--send-mail", "--cc", emailStr)
	}
	commitMsg += tbr
	uploadCmd = append(uploadCmd, "-m", commitMsg)

	// Upload the CL.
	uploadOutput, err := exec.RunCwd(r.chromiumDir, uploadCmd...)
	if err != nil {
		return 0, err
	}
	issues := ISSUE_CREATED_REGEX.FindStringSubmatch(uploadOutput)
	if len(issues) != 2 {
		return 0, fmt.Errorf("Failed to find newly-uploaded issue number!")
	}
	return strconv.ParseInt(issues[1], 10, 64)
}
