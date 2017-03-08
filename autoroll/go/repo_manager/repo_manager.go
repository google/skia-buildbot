package repo_manager

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/util"
)

const (
	DEPS_ROLL_BRANCH = "roll_branch"

	GCLIENT  = "gclient"
	ROLL_DEP = "roll-dep"

	ROLL_STRATEGY_BATCH  = "batch"
	ROLL_STRATEGY_SINGLE = "single"

	TMPL_CQ_INCLUDE_TRYBOTS = "CQ_INCLUDE_TRYBOTS=%s"
)

var (
	// Use this function to instantiate a RepoManager. This is able to be
	// overridden for testing.
	NewRepoManager func(string, string, string, time.Duration, string) (RepoManager, error) = NewDefaultRepoManager

	DEPOT_TOOLS_AUTH_USER_REGEX = regexp.MustCompile(fmt.Sprintf("Logged in to %s as ([\\w-]+).", autoroll.RIETVELD_URL))
)

// issueJson is the structure of "git cl issue --json"
type issueJson struct {
	Issue    int64  `json:"issue"`
	IssueUrl string `json:"issue_url"`
}

// RepoManager is used by AutoRoller for managing checkouts.
type RepoManager interface {
	ForceUpdate() error
	FullChildHash(string) (string, error)
	LastRollRev() string
	RolledPast(string) (bool, error)
	ChildHead() string
	CreateNewRoll(string, []string, string, bool, bool) (int64, error)
	User() string
}

// repoManager is a struct used by AutoRoller for managing checkouts.
type repoManager struct {
	depot_tools string
	gclient     string
	infoMtx     sync.RWMutex
	lastRollRev string
	repoMtx     sync.RWMutex
	rollDep     string
	childDir    string
	childHead   string
	childPath   string
	childRepo   *gitinfo.GitInfo
	parentDir   string
	parentRepo  string
	user        string
	workdir     string
}

// getEnv returns the environment used for most commands.
func getEnv(depotTools string) []string {
	return []string{
		fmt.Sprintf("PATH=%s:%s", depotTools, os.Getenv("PATH")),
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
		fmt.Sprintf("SKIP_GCE_AUTH_FOR_GIT=1"),
	}
}

// getDepotToolsUser returns the authorized depot tools user.
func getDepotToolsUser(depotTools string) (string, error) {
	output, err := exec.RunCommand(&exec.Command{
		Env:  getEnv(depotTools),
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
func NewDefaultRepoManager(workdir, parentRepo, childPath string, frequency time.Duration, depot_tools string) (RepoManager, error) {
	gclient := GCLIENT
	rollDep := ROLL_DEP
	if depot_tools != "" {
		gclient = path.Join(depot_tools, gclient)
		rollDep = path.Join(depot_tools, rollDep)
	}

	wd := path.Join(workdir, "repo_manager")
	parentBase := strings.TrimSuffix(path.Base(parentRepo), ".git")
	parentDir := path.Join(wd, parentBase)

	user, err := getDepotToolsUser(depot_tools)
	if err != nil {
		return nil, fmt.Errorf("Failed to determine depot tools user: %s", err)
	}

	r := &repoManager{
		depot_tools: depot_tools,
		gclient:     gclient,
		rollDep:     rollDep,
		childDir:    path.Join(wd, childPath),
		childPath:   childPath,
		childRepo:   nil, // This will be filled in on the first update.
		parentDir:   parentDir,
		parentRepo:  parentRepo,
		user:        user,
		workdir:     wd,
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
	r.repoMtx.Lock()
	defer r.repoMtx.Unlock()

	// Create the working directory if needed.
	if _, err := os.Stat(r.workdir); err != nil {
		if err := os.MkdirAll(r.workdir, 0755); err != nil {
			return err
		}
	}

	if _, err := os.Stat(path.Join(r.parentDir, ".git")); err == nil {
		if err := r.cleanParent(); err != nil {
			return err
		}
		// Update the repo.
		if _, err := exec.RunCwd(r.parentDir, "git", "fetch"); err != nil {
			return err
		}
		if _, err := exec.RunCwd(r.parentDir, "git", "reset", "--hard", "origin/master"); err != nil {
			return err
		}
	}

	if _, err := exec.RunCommand(&exec.Command{
		Dir:  r.workdir,
		Env:  getEnv(r.depot_tools),
		Name: r.gclient,
		Args: []string{"config", r.parentRepo},
	}); err != nil {
		return err
	}
	if _, err := exec.RunCommand(&exec.Command{
		Dir:  r.workdir,
		Env:  getEnv(r.depot_tools),
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

	// Record child HEAD
	childHead, err := r.childRepo.FullHash("origin/master")
	if err != nil {
		return err
	}
	r.infoMtx.Lock()
	defer r.infoMtx.Unlock()
	r.lastRollRev = lastRollRev
	r.childHead = childHead
	return nil
}

// ForceUpdate forces the repoManager to update.
func (r *repoManager) ForceUpdate() error {
	return r.update()
}

// getLastRollRev returns the commit hash of the last-completed DEPS roll.
func (r *repoManager) getLastRollRev() (string, error) {
	output, err := exec.RunCwd(r.parentDir, r.gclient, "revinfo")
	if err != nil {
		return "", err
	}
	split := strings.Split(output, "\n")
	for _, s := range split {
		if strings.HasPrefix(s, r.childPath) {
			subs := strings.Split(s, "@")
			if len(subs) != 2 {
				return "", fmt.Errorf("Failed to parse output of `gclient revinfo`:\n\n%s\n", output)
			}
			return subs[1], nil
		}
	}
	return "", fmt.Errorf("Failed to parse output of `gclient revinfo`:\n\n%s\n", output)
}

// FullChildHash returns the full hash of the given short hash or ref in the
// child repo.
func (r *repoManager) FullChildHash(shortHash string) (string, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	return r.childRepo.FullHash(shortHash)
}

// LastRollRev returns the last-rolled child commit.
func (r *repoManager) LastRollRev() string {
	r.infoMtx.RLock()
	defer r.infoMtx.RUnlock()
	return r.lastRollRev
}

// RolledPast determines whether DEPS has rolled past the given commit.
func (r *repoManager) RolledPast(hash string) (bool, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	return git.GitDir(r.childDir).IsAncestor(hash, r.lastRollRev)
}

// ChildHead returns the current child origin/master branch head.
func (r *repoManager) ChildHead() string {
	r.infoMtx.RLock()
	defer r.infoMtx.RUnlock()
	return r.childHead
}

// cleanParent forces the parent checkout into a clean state.
func (r *repoManager) cleanParent() error {
	if _, err := exec.RunCwd(r.parentDir, "git", "clean", "-d", "-f", "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(r.parentDir, "git", "rebase", "--abort")
	if _, err := exec.RunCwd(r.parentDir, "git", "checkout", "origin/master", "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(r.parentDir, "git", "branch", "-D", DEPS_ROLL_BRANCH)
	if _, err := exec.RunCommand(&exec.Command{
		Dir:  r.workdir,
		Env:  getEnv(r.depot_tools),
		Name: r.gclient,
		Args: []string{"revert", "--nohooks"},
	}); err != nil {
		return err
	}
	return nil
}

// CreateNewRoll creates and uploads a new DEPS roll to the given commit.
// Returns the issue number of the uploaded roll.
func (r *repoManager) CreateNewRoll(strategy string, emails []string, cqExtraTrybots string, dryRun, gerrit bool) (int64, error) {
	r.repoMtx.Lock()
	defer r.repoMtx.Unlock()

	// Clean the checkout, get onto a fresh branch.
	if err := r.cleanParent(); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(r.parentDir, "git", "checkout", "-b", DEPS_ROLL_BRANCH, "-t", "origin/master", "-f"); err != nil {
		return 0, err
	}

	// Defer some more cleanup.
	defer func() {
		util.LogErr(r.cleanParent())
	}()

	// Create the roll CL.

	// Determine what commit we're rolling to.
	cr := r.childRepo
	commits, err := cr.RevList(fmt.Sprintf("%s..%s", r.lastRollRev, r.childHead))
	if err != nil {
		return 0, fmt.Errorf("Failed to list revisions: %s", err)
	}
	rollTo := r.childHead
	if strategy == ROLL_STRATEGY_SINGLE {
		rollTo = commits[len(commits)-1]
		commits = commits[len(commits)-1:]
	}

	if _, err := exec.RunCwd(r.parentDir, "git", "config", "user.name", autoroll.ROLL_AUTHOR); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(r.parentDir, "git", "config", "user.email", autoroll.ROLL_AUTHOR); err != nil {
		return 0, err
	}

	// Find Chromium bugs.
	bugs := []string{}
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

	// Run roll-dep.
	args := []string{r.childPath, "--roll-to", rollTo}
	if len(bugs) > 0 {
		args = append(args, "--bug", strings.Join(bugs, ","))
	}
	sklog.Infof("Running command: roll-dep %s", strings.Join(args, " "))
	if _, err := exec.RunCommand(&exec.Command{
		Dir:  r.parentDir,
		Env:  getEnv(r.depot_tools),
		Name: r.rollDep,
		Args: args,
	}); err != nil {
		return 0, err
	}
	// Build the commit message, starting with the message provided by roll-dep.
	commitMsg, err := exec.RunCwd(r.parentDir, "git", "log", "-n1", "--format=%B", "HEAD")
	if err != nil {
		return 0, err
	}
	commitMsg += `
Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

If the roll is causing failures, see:
http://www.chromium.org/developers/tree-sheriffs/sheriff-details-chromium#TOC-Failures-due-to-DEPS-rolls

`
	if cqExtraTrybots != "" {
		commitMsg += "\n" + fmt.Sprintf(TMPL_CQ_INCLUDE_TRYBOTS, cqExtraTrybots)
	}
	uploadCmd := &exec.Command{
		Dir:  r.parentDir,
		Env:  getEnv(r.depot_tools),
		Name: "git",
		Args: []string{"cl", "upload", "--bypass-hooks", "-f"},
	}
	if dryRun {
		uploadCmd.Args = append(uploadCmd.Args, "--cq-dry-run")
	} else {
		uploadCmd.Args = append(uploadCmd.Args, "--use-commit-queue")
	}
	if gerrit {
		uploadCmd.Args = append(uploadCmd.Args, "--gerrit")
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
	if _, err := exec.RunCommand(uploadCmd); err != nil {
		return 0, err
	}

	// Obtain the issue number.
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return 0, err
	}
	defer util.RemoveAll(tmp)
	jsonFile := path.Join(tmp, "issue.json")
	if _, err := exec.RunCommand(&exec.Command{
		Dir:  r.parentDir,
		Env:  getEnv(r.depot_tools),
		Name: "git",
		Args: []string{"cl", "issue", fmt.Sprintf("--json=%s", jsonFile)},
	}); err != nil {
		return 0, err
	}
	f, err := os.Open(jsonFile)
	if err != nil {
		return 0, err
	}
	var issue issueJson
	if err := json.NewDecoder(f).Decode(&issue); err != nil {
		return 0, err
	}
	return issue.Issue, nil
}

func (r *repoManager) User() string {
	return r.user
}
