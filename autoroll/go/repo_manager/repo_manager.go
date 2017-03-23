package repo_manager

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
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

	SERVICE_ACCOUNT = "31977622648@project.gserviceaccount.com"

	UPSTREAM_REMOTE_NAME = "remote"

	REPO_BRANCH_NAME = "merge"
)

var (
	// Use this function to instantiate a RepoManager. This is able to be
	// overridden for testing.
	NewRepoManager func(string, string, string, time.Duration, string) (RepoManager, error) = NewDefaultRepoManager

	DEPOT_TOOLS_AUTH_USER_REGEX = regexp.MustCompile(fmt.Sprintf("Logged in to %s as ([\\w-]+).", autoroll.RIETVELD_URL))

	IGNORE_MERGE_CONFLICT_FILES = []string{"include/config/SkUserConfig.h"}

	FILES_GENERATED_BY_GN_TO_GP = []string{"include/config/SkUserConfig.h", "Android.bp"}
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
	//SendToCQ(*gerrit.ChangeInfo, comment string) error
	//SendToDryRun(*gerrit.ChangeInfo, comment string) error
}

type AndroidRepoManager struct {
	RepoManager
}

// repoManager is a struct used by AutoRoller for managing checkouts.
type repoManager struct {
	depot_tools             string
	gclient                 string
	infoMtx                 sync.RWMutex
	lastRollRev             string
	repoMtx                 sync.RWMutex
	rollDep                 string
	childDir                string
	childHead               string
	childPath               string
	childRepo               *gitinfo.GitInfo
	parentDir               string
	parentRepo              string
	user                    string
	workdir                 string
	repoToolPath            string
	gitCookieAuthDaemonPath string
}

type androidRepoManager struct {
	infoMtx     sync.RWMutex
	lastRollRev string
	repoMtx     sync.RWMutex
	childDir    string
	childHead   string
	childPath   string
	childRepo   *gitinfo.GitInfo
	//parentDir           string
	//parentRepo          string
	user                string
	workdir             string
	repoTool            string
	gitCookieAuthDaemon string
	api                 *gerrit.Gerrit
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

// rmistry
func NewAndroidRepoManager(workdir, childPath string, frequency time.Duration, repoTool, gitCookieAuthDaemon string) (RepoManager, error) {

	wd := path.Join(workdir, "android_repo")

	usr, err := user.Current()
	if err != nil {
		return nil, err
	}
	// TODO(rmistry): What should this be to work from desktop?
	//gitCookiePath := filepath.Join(usr.HomeDir, ".git-credential-cache", "cookie")
	gitCookiePath := filepath.Join(usr.HomeDir, ".gitcookies")
	// TODO(rmistry): Get from metadata.
	api, err := gerrit.NewGerrit("https://googleplex-android-review.googlesource.com", gitCookiePath, nil)
	if err != nil {
		return nil, err
	}
	api.TurnOnAuthenticatedGets()

	r := &androidRepoManager{
		childDir:            path.Join(wd, childPath),
		childPath:           childPath,
		childRepo:           nil, // This will be filled in on the first update.
		gitCookieAuthDaemon: gitCookieAuthDaemon,
		repoTool:            repoTool,
		user:                SERVICE_ACCOUNT,
		workdir:             wd,
		api:                 api,
	}

	// Put the git config --global color.ui true in the startup script?? and the other git config stuff??

	if err := r.update(); err != nil {
		return nil, err
	}
	go func() {
		for _ = range time.Tick(frequency) {
			// Authenticate before trying to update repo.
			if _, err := exec.RunCwd(gitCookieAuthDaemon); err != nil {
				util.LogErr(err)
			}
			// Update repo.
			util.LogErr(r.update())
		}
	}()
	return r, nil
}

// update syncs code in the relevant repositories.
func (r *androidRepoManager) update() error {
	// Sync the projects.
	r.repoMtx.Lock()
	defer r.repoMtx.Unlock()

	// Create the working directory if needed.
	if _, err := os.Stat(r.workdir); err != nil {
		if err := os.MkdirAll(r.workdir, 0755); err != nil {
			return err
		}
	}

	// TODO(rmistry): Get URL from metadata
	//fmt.Println("Skipping the init and sync cmds!")
	//fmt.Println("ABOUT TO RUN THE INIT AND SYNC CMDS")
	//if _, err := exec.RunCwd(r.workdir, r.repoTool, "init", "-u", "https://googleplex-android.googlesource.com/a/platform/manifest", "-g", "all,-notdefault,-darwin", "-b", "master"); err != nil {
	//	return err
	//}
	//if _, err := exec.RunCwd(r.workdir, r.repoTool, "sync", "-j32"); err != nil {
	//	return err
	//}

	// Create the child GitInfo if needed.
	if r.childRepo == nil {
		childRepo, err := gitinfo.NewGitInfo(r.childDir, false, false /* allBranches what should this be?*/)
		if err != nil {
			return err
		}
		r.childRepo = childRepo
	}

	// Fix the review to be "https://googleplex-android.googlesource.com/"
	// instead of "sso://googleplex-android which does not work outside prod.
	// TODO(rmistry): Get this from metadata as well.
	if _, err := exec.RunCwd(r.childRepo.Dir(), "git", "config", "remote.goog.review", "https://googleplex-android.googlesource.com/"); err != nil {
		return err
	}

	// Check to see whether there is an upstream yet.
	remoteOutput, err := exec.RunCwd(r.childRepo.Dir(), "git", "remote", "show")
	if err != nil {
		return err
	}
	if !strings.Contains(remoteOutput, UPSTREAM_REMOTE_NAME) {
		if _, err := exec.RunCwd(r.childRepo.Dir(), "git", "remote", "add", UPSTREAM_REMOTE_NAME, common.REPO_SKIA); err != nil {
			return err
		}
	}

	// Get the last roll revision.
	lastRollRev, err := r.getLastRollRev()
	if err != nil {
		return err
	}

	// Record child HEAD
	childHead, err := r.getChildRepoHead()
	if err != nil {
		return err
	}
	r.infoMtx.Lock()
	defer r.infoMtx.Unlock()
	r.lastRollRev = lastRollRev
	r.childHead = childHead

	fmt.Println("lastRollRev is:")
	fmt.Println(lastRollRev)
	fmt.Println("Child head is:")
	fmt.Println(childHead)
	return nil
}

// ForceUpdate forces the repoManager to update.
func (r *androidRepoManager) ForceUpdate() error {
	return r.update()
}

// getChildRepoHead returns the commit hash of the latest commit in the child repo.
func (r *androidRepoManager) getChildRepoHead() (string, error) {
	output, err := exec.RunCwd(r.childRepo.Dir(), "git", "ls-remote", UPSTREAM_REMOTE_NAME, "refs/heads/master", "-1")
	if err != nil {
		return "", err
	}
	tokens := strings.Split(output, "\t")
	return tokens[0], nil
}

// getLastRollRev returns the commit hash of the last-completed DEPS roll.
func (r *androidRepoManager) getLastRollRev() (string, error) {
	output, err := exec.RunCwd(r.childRepo.Dir(), "git", "log", "--pretty=format:%ae %H")
	if err != nil {
		return "", err
	}
	commitLines := strings.Split(output, "\n")
	indexWithMergeCommit := 0
	for i, commitLine := range commitLines {
		tokens := strings.Split(commitLine, " ")
		if tokens[0] == "31977622648@project.gserviceaccount.com" {
			indexWithMergeCommit = i
			break
		}
	}
	// The commit immediately before the merge commit will have a commit hash
	// that corresponds to the target repo.
	tokens := strings.Split(commitLines[indexWithMergeCommit+1], " ")
	return tokens[1], nil
}

// FullChildHash returns the full hash of the given short hash or ref in the
// child repo.
func (r *androidRepoManager) FullChildHash(shortHash string) (string, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	return r.childRepo.FullHash(shortHash)
}

// LastRollRev returns the last-rolled child commit.
func (r *androidRepoManager) LastRollRev() string {
	r.infoMtx.RLock()
	defer r.infoMtx.RUnlock()
	return r.lastRollRev
}

// RolledPast determines whether DEPS has rolled past the given commit.
func (r *androidRepoManager) RolledPast(hash string) (bool, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	return git.GitDir(r.childDir).IsAncestor(hash, r.lastRollRev)
}

// ChildHead returns the current child origin/master branch head.
func (r *androidRepoManager) ChildHead() string {
	r.infoMtx.RLock()
	defer r.infoMtx.RUnlock()
	return r.childHead
}

// abortMerge aborts the current merge in the child repo.
func (r *androidRepoManager) abortMerge() error {
	_, err := exec.RunCwd(r.childRepo.Dir(), "git", "merge", "--abort")
	return err
}

// abandonRepoBranch abandons the repo branch.
func (r *androidRepoManager) abandonRepoBranch() error {
	_, err := exec.RunCwd(r.childRepo.Dir(), "repo", "abandon", REPO_BRANCH_NAME)
	return err
}

// getChangeNumForHash returns the corresponding change number for the provided commit hash by querying Gerrit's search API.
func (r *androidRepoManager) getChangeForHash(hash string) (*gerrit.ChangeInfo, error) {
	issues, err := r.api.Search(1, gerrit.SearchCommit(hash))
	if err != nil {
		return nil, err
	}
	return r.api.GetIssueProperties(issues[0].Issue)
}

// setTopic sets a topic using the name of the child repo and the change number.
// Example: skia_merge_1234
func (r *androidRepoManager) setTopic(changeNum int64) error {
	topic := fmt.Sprintf("%s_merge_%d", path.Base(r.childDir), changeNum)
	return r.api.SetTopic(topic, changeNum)
}

// setLabelsToAutoLandChange sets the appropriate labels on the Gerrit change to auto-land.
// It uses the Gerrit REST API to set the following labels on the change:
// * Code-Review=2
// * Autosubmit=1
// * Presubmit-Ready=1
// The above labels will ensure that when/if TreeHugger completes successfully
// then the change will be automatically submitted.
func (r *androidRepoManager) setLabelsToAutoLandChange(change *gerrit.ChangeInfo) error {
	labelValues := map[string]interface{}{
		"Code-Review":     "2",
		"Autosubmit":      "1",
		"Presubmit-Ready": "1",
	}
	return r.api.SetReview(change, "Roller setting labels to auto-land change.", labelValues)
}

// CreateNewRoll creates and uploads a new Android roll to the given commit.
// Returns the change number of the uploaded roll.
func (r *androidRepoManager) CreateNewRoll(strategy string, emails []string, cqExtraTrybots string, dryRun, gerrit bool) (int64, error) {
	r.repoMtx.Lock()
	defer r.repoMtx.Unlock()

	//// Defer some cleanup.
	//defer func() {
	//	util.LogErr(r.cleanParent())
	//}()

	// Update the upstream remote.
	if _, err := exec.RunCwd(r.childDir, "git", "fetch", UPSTREAM_REMOTE_NAME); err != nil {
		return 0, err
	}

	// Create the roll CL.

	// Determine what commit we're rolling to.
	cr := r.childRepo
	commitRange := fmt.Sprintf("%s..%s", r.lastRollRev, r.childHead)
	commits, err := cr.RevList(commitRange)
	if err != nil {
		return 0, fmt.Errorf("Failed to list revisions: %s", err)
	}
	rollTo := r.childHead
	if strategy == ROLL_STRATEGY_SINGLE {
		rollTo = commits[len(commits)-1]
		commits = commits[len(commits)-1:]
	}

	// TODO(rmistry): TEMP TEMP TEMP
	//rollTo = "0c984a0af30989fe20b1f8af18867983a88c48b6" // 2 changes conflict here!
	//commits = []string{"0c984a0af30989fe20b1f8af18867983a88c48b6", "f49b1e0ad955c437675eae6e8bd64a2e0941e204"}

	// Start the merge.

	if _, err := exec.RunCwd(r.childDir, "git", "merge", rollTo, "--no-commit"); err != nil {
		// Check to see if this was a merge conflict with IGNORE_MERGE_CONFLICT_FILES.
		conflictsOutput, conflictsErr := exec.RunCwd(r.childDir, "git", "diff", "--name-only", "--diff-filter=U")
		fmt.Println(conflictsOutput)
		fmt.Println(conflictsErr)
		if conflictsErr != nil || conflictsOutput == "" {
			util.LogErr(conflictsErr)
			return 0, fmt.Errorf("Failed to roll to %s. Needs human investigation: %s", rollTo, err)
		}
		for _, conflict := range strings.Split(conflictsOutput, "\n") {
			if conflict == "" {
				continue
			}
			ignoreConflict := false
			for _, ignore := range IGNORE_MERGE_CONFLICT_FILES {
				if conflict == ignore {
					ignoreConflict = true
					sklog.Infof("Ignoring conflict in %s", conflict)
					break
				}
			}
			if !ignoreConflict {
				util.LogErr(r.abortMerge())
				return 0, fmt.Errorf("Failed to roll to %s. Conflicts in %s: %s", rollTo, conflictsOutput, err)
			}
		}
	}

	// Install GN.
	if _, syncErr := exec.RunCwd(r.childDir, "./bin/sync"); syncErr != nil {
		// Sync may return errors, but this is ok.
	}
	if _, fetchGNErr := exec.RunCwd(r.childDir, "./bin/fetch-gn"); fetchGNErr != nil {
		return 0, fmt.Errorf("Failed to install GN: %s", fetchGNErr)
	}

	// Generate and add files created by gn/gn_to_bp.py
	// TODO(rmistry): Use this when you test against closer to head.
	// if _, gnToBpErr := exec.RunCwd(r.childDir, "python", "-c", "from gn import gn_to_bp"); gnToBpErr != nil {
	if _, gnToBpErr := exec.RunCwd(r.childDir, "python", "-c", "import sys; sys.path.append('gn'); import gn_to_bp;"); gnToBpErr != nil {
		util.LogErr(r.abortMerge())
		return 0, fmt.Errorf("Failed to run gn_to_bp: %s", gnToBpErr)
	}
	for _, genFile := range FILES_GENERATED_BY_GN_TO_GP {
		if _, err := exec.RunCwd(r.childDir, "git", "add", genFile); err != nil {
			return 0, err
		}
	}

	// Create a new repo branch.
	if _, repoBranchErr := exec.RunCwd(r.childDir, "repo", "start", REPO_BRANCH_NAME, "."); repoBranchErr != nil {
		util.LogErr(r.abortMerge())
		return 0, fmt.Errorf("Failed to create repo branch: %s", repoBranchErr)
	}

	// Create commit message.
	commitMsg := fmt.Sprintf(
		`Merge latest Skia into master (%d commits)

https://skia.googlesource.com/skia.git/+log/%s

Test: Presubmit checks will test this change.
`, len(commits), commitRange)

	// Commit the change with the above message.
	if _, commitErr := exec.RunCwd(r.childDir, "git", "commit", "-m", commitMsg); commitErr != nil {
		util.LogErr(r.abandonRepoBranch())
		return 0, fmt.Errorf("Nothing to merge; did someone already merge %s?: %s", commitRange, commitErr)
	}

	// Bypass the repo upload prompt by setting autoupload config to true.
	if _, configErr := exec.RunCwd(r.childDir, "git", "config", "review.https://googleplex-android.googlesource.com/.autoupload", "true"); configErr != nil {
		util.LogErr(r.abandonRepoBranch())
		return 0, fmt.Errorf("Could not set autoupload config: %s", configErr)
	}

	// Upload the CL to Gerrit.
	emailStr := strings.Join(emails, ",")
	if _, uploadErr := exec.RunCwd(r.childDir, "repo", "upload", fmt.Sprintf("--re=%s", emailStr), "--verify"); uploadErr != nil {
		util.LogErr(r.abandonRepoBranch())
		return 0, fmt.Errorf("Could not upload to Gerrit: %s", uploadErr)
	}

	// Get latest hash to find Gerrit change number with.
	commitHashOutput, revParseErr := exec.RunCwd(r.childDir, "git", "rev-parse", "HEAD")
	if revParseErr != nil {
		util.LogErr(r.abandonRepoBranch())
		return 0, revParseErr
	}
	commitHash := strings.Split(commitHashOutput, "\n")[0]
	// We no longer need the local branch. Abandon the repo.
	util.LogErr(r.abandonRepoBranch())

	// Get the change number.
	change, err := r.getChangeForHash(commitHash)
	if err != nil {
		util.LogErr(r.abandonRepoBranch())
		return 0, err
	}
	// Set the topic of the merge change.
	if err := r.setTopic(change.Issue); err != nil {
		return 0, err
	}
	// Set the auto land labels.
	if err := r.setLabelsToAutoLandChange(change); err != nil {
		return 0, err
	}

	return change.Issue, nil
}

func (r *androidRepoManager) User() string {
	return r.user
}

// /////////////////////////////////////////////////////////////////////////////////////////

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
