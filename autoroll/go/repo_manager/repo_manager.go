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
	DEPS_ROLL_BRANCH = "roll_branch"

	GCLIENT  = "gclient"
	ROLL_DEP = "roll-dep"

	REPO_CHROMIUM = "https://chromium.googlesource.com/chromium/src.git"

	TMPL_CQ_INCLUDE_TRYBOTS = "CQ_INCLUDE_TRYBOTS=%s"
)

var (
	ISSUE_CREATED_REGEX = regexp.MustCompile(fmt.Sprintf("Issue created. URL: %s/(\\d+)", autoroll.RIETVELD_URL))

	// Use this function to instantiate a RepoManager. This is able to be
	// overridden for testing.
	NewRepoManager func(string, string, time.Duration, string) (RepoManager, error) = NewDefaultRepoManager

	DEPOT_TOOLS_AUTH_USER_REGEX = regexp.MustCompile(fmt.Sprintf("Logged in to %s as ([\\w-]+).", autoroll.RIETVELD_URL))
)

// RepoManager is used by AutoRoller for managing checkouts.
type RepoManager interface {
	ForceUpdate() error
	FullChildHash(string) (string, error)
	LastRollRev() string
	RolledPast(string) bool
	ChildHead() string
	CreateNewRoll([]string, string, bool) (int64, error)
	User() string
}

// repoManager is a struct used by AutoRoller for managing checkouts.
type repoManager struct {
	chromiumDir       string
	chromiumParentDir string
	depot_tools       string
	gclient           string
	lastRollRev       string
	mtx               sync.RWMutex
	rollDep           string
	childDir          string
	childHead         string
	childPath         string
	childRepo         *gitinfo.GitInfo
	user              string
}

// getDepotToolsUser returns the authorized depot tools user.
func getDepotToolsUser(depotTools string) (string, error) {
	output, err := exec.RunCommand(&exec.Command{
		Env:  []string{fmt.Sprintf("PATH=%s:%s", depotTools, os.Getenv("PATH"))},
		Name: path.Join(depotTools, "depot-tools-auth"),
		Args: []string{"info", autoroll.RIETVELD_URL},
	})
	if err != nil {
		return "", err
	}
	m := DEPOT_TOOLS_AUTH_USER_REGEX.FindStringSubmatch(output)
	if len(m) != 2 {
		return "", fmt.Errorf("Unable to parse the output of depot-tools-auth.")
	}
	return m[1], nil
}

// NewDefaultRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func NewDefaultRepoManager(workdir, childPath string, frequency time.Duration, depot_tools string) (RepoManager, error) {
	gclient := GCLIENT
	rollDep := ROLL_DEP
	if depot_tools != "" {
		gclient = path.Join(depot_tools, gclient)
		rollDep = path.Join(depot_tools, rollDep)
	}

	chromiumParentDir := path.Join(workdir, "chromium")
	chromiumDir := path.Join(chromiumParentDir, "src")

	user, err := getDepotToolsUser(depot_tools)
	if err != nil {
		return nil, fmt.Errorf("Failed to determine depot tools user: %s", err)
	}

	r := &repoManager{
		chromiumDir:       chromiumDir,
		chromiumParentDir: chromiumParentDir,
		depot_tools:       depot_tools,
		gclient:           gclient,
		rollDep:           rollDep,
		childDir:          path.Join(chromiumParentDir, childPath),
		childPath:         childPath,
		childRepo:         nil, // This will be filled in on the first update.
		user:              user,
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
func (r *repoManager) update() error {
	// Sync the projects.
	r.mtx.Lock()
	defer r.mtx.Unlock()

	// Create the chromium parent directory if needed.
	if _, err := os.Stat(r.chromiumParentDir); err != nil {
		if err := os.MkdirAll(r.chromiumParentDir, 0755); err != nil {
			return err
		}
	}

	if _, err := os.Stat(path.Join(r.chromiumDir, ".git")); err == nil {
		if err := r.cleanChromium(); err != nil {
			return err
		}
	}

	if _, err := exec.RunCommand(&exec.Command{
		Dir:  r.chromiumParentDir,
		Env:  []string{fmt.Sprintf("PATH=%s:%s", r.depot_tools, os.Getenv("PATH"))},
		Name: r.gclient,
		Args: []string{"config", REPO_CHROMIUM},
	}); err != nil {
		return err
	}
	if _, err := exec.RunCommand(&exec.Command{
		Dir:  r.chromiumParentDir,
		Env:  []string{fmt.Sprintf("PATH=%s:%s", r.depot_tools, os.Getenv("PATH"))},
		Name: r.gclient,
		Args: []string{"sync", "--nohooks"},
	}); err != nil {
		return err
	}

	// Create the child GitInfo if needed.
	if r.childRepo == nil {
		childRepo, err := gitinfo.NewGitInfo(r.childDir, false, true)
		if err != nil {
			return err
		}
		r.childRepo = childRepo
	}

	// Get the last roll revision.
	lastRollRev, err := r.getLastRollRev()
	if err != nil {
		return err
	}
	r.lastRollRev = lastRollRev

	// Record child HEAD
	childHead, err := r.childRepo.FullHash("origin/master")
	if err != nil {
		return err
	}
	r.childHead = childHead
	return nil
}

// ForceUpdate forces the repoManager to update.
func (r *repoManager) ForceUpdate() error {
	return r.update()
}

// getLastRollRev returns the commit hash of the last-completed DEPS roll.
func (r *repoManager) getLastRollRev() (string, error) {
	output, err := exec.RunCwd(r.chromiumDir, r.gclient, "revinfo")
	if err != nil {
		return "", err
	}
	split := strings.Split(output, "\n")
	for _, s := range split {
		if strings.HasPrefix(s, r.childPath) {
			subs := strings.Split(s, "@")
			if len(subs) != 2 {
				return "", fmt.Errorf("Failed to parse output of `gclient revinfo`")
			}
			return subs[1], nil
		}
	}
	return "", fmt.Errorf("Failed to parse output of `gclient revinfo`")
}

// FullChildHash returns the full hash of the given short hash or ref in the
// child repo.
func (r *repoManager) FullChildHash(shortHash string) (string, error) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.childRepo.FullHash(shortHash)
}

// LastRollRev returns the last-rolled child commit.
func (r *repoManager) LastRollRev() string {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.lastRollRev
}

// RolledPast determines whether DEPS has rolled past the given commit.
func (r *repoManager) RolledPast(hash string) bool {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	if _, err := exec.RunCwd(r.childDir, "git", "merge-base", "--is-ancestor", hash, r.lastRollRev); err != nil {
		return false
	}
	return true
}

// ChildHead returns the current child origin/master branch head.
func (r *repoManager) ChildHead() string {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.childHead
}

// cleanChromium forces the Chromium checkout into a clean state.
func (r *repoManager) cleanChromium() error {
	if _, err := exec.RunCwd(r.chromiumDir, "git", "clean", "-d", "-f", "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(r.chromiumDir, "git", "rebase", "--abort")
	if _, err := exec.RunCwd(r.chromiumDir, "git", "checkout", "origin/master", "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(r.chromiumDir, "git", "branch", "-D", DEPS_ROLL_BRANCH)
	if _, err := exec.RunCommand(&exec.Command{
		Dir:  r.chromiumDir,
		Env:  []string{fmt.Sprintf("PATH=%s:%s", r.depot_tools, os.Getenv("PATH"))},
		Name: r.gclient,
		Args: []string{"revert", "--nohooks"},
	}); err != nil {
		return err
	}
	return nil
}

// CreateNewRoll creates and uploads a new DEPS roll to the given commit.
// Returns the issue number of the uploaded roll.
func (r *repoManager) CreateNewRoll(emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

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
	if _, err := exec.RunCwd(r.chromiumDir, "git", "config", "user.name", autoroll.ROLL_AUTHOR); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(r.chromiumDir, "git", "config", "user.email", autoroll.ROLL_AUTHOR); err != nil {
		return 0, err
	}

	// Find Chromium bugs.
	bugs := []string{}
	cr := r.childRepo
	commits, err := cr.RevList(fmt.Sprintf("%s..%s", r.lastRollRev, r.childHead))
	if err != nil {
		return 0, fmt.Errorf("Failed to list revisions: %s", err)
	}
	for _, c := range commits {
		d, err := cr.Details(c, false)
		if err != nil {
			return 0, fmt.Errorf("Failed to obtain commit details: %s", err)
		}
		b := util.BugsFromCommitMsg(d.Body)
		for _, bug := range b[util.PROJECT_CHROMIUM] {
			bugs = append(bugs, bug)
		}
	}

	args := []string{r.childPath, r.childHead}
	for _, bug := range bugs {
		args = append(args, "--bug", bug)
	}
	if _, err := exec.RunCommand(&exec.Command{
		Dir:  r.chromiumDir,
		Env:  []string{fmt.Sprintf("PATH=%s:%s", r.depot_tools, os.Getenv("PATH"))},
		Name: r.rollDep,
		Args: []string{r.childPath, r.childHead},
	}); err != nil {
		return 0, err
	}
	// Build the commit message, starting with the message provided by roll-dep.
	commitMsg, err := exec.RunCwd(r.chromiumDir, "git", "log", "-n1", "--format=%B", "HEAD")
	if err != nil {
		return 0, err
	}
	if cqExtraTrybots != "" {
		commitMsg += "\n" + fmt.Sprintf(TMPL_CQ_INCLUDE_TRYBOTS, cqExtraTrybots)
	}
	uploadCmd := &exec.Command{
		Dir:  r.chromiumDir,
		Env:  []string{fmt.Sprintf("PATH=%s:%s", r.depot_tools, os.Getenv("PATH"))},
		Name: "git",
		Args: []string{"cl", "upload", "--bypass-hooks", "-f"},
	}
	if dryRun {
		uploadCmd.Args = append(uploadCmd.Args, "--cq-dry-run")
	} else {
		uploadCmd.Args = append(uploadCmd.Args, "--use-commit-queue")
	}
	tbr := "\nTBR="
	if emails != nil && len(emails) > 0 {
		emailStr := strings.Join(emails, ",")
		tbr += emailStr
		uploadCmd.Args = append(uploadCmd.Args, "--send-mail", "--cc", emailStr)
	}
	commitMsg += tbr
	uploadCmd.Args = append(uploadCmd.Args, "-m", commitMsg)

	// Upload the CL.
	uploadOutput, err := exec.RunCommand(uploadCmd)
	if err != nil {
		return 0, err
	}
	issues := ISSUE_CREATED_REGEX.FindStringSubmatch(uploadOutput)
	if len(issues) != 2 {
		return 0, fmt.Errorf("Failed to find newly-uploaded issue number!")
	}
	return strconv.ParseInt(issues[1], 10, 64)
}

func (r *repoManager) User() string {
	return r.user
}
