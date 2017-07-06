package repo_manager

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	MANIFEST_ROLL_BRANCH = "roll_branch"
	MANIFEST_FILE_NAME   = "userspace"
)

var (
	// Use this function to instantiate a RepoManager. This is able to be
	// overridden for testing.
	NewManifestRepoManager func(string, string, string, string, string, time.Duration, string, *gerrit.Gerrit) (RepoManager, error) = newManifestRepoManager
)

// manifestRepoManager is a struct used by Manifest AutoRoller for managing checkouts.
type manifestRepoManager struct {
	*depotToolsRepoManager
}

type Project struct {
	project    xml.Name `xml.Project`
	name       string   `xml:"name, attr"`
	path       string   `xml:"path, attr"`
	remote     string   `xml:"remote, attr"`
	gerrithost string   `xml:"gerrithost, attr"`
	githooks   string   `xml:"githooks, attr"`
}

type Manifests struct {
	manifest xml.Name `xml:"manifest"̀`
	projects Project  `xml:"projects"`
}

// newDEPSRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func newManifestRepoManager(workdir, parentRepo, parentBranch, childPath, childBranch string, frequency time.Duration, depot_tools string, g *gerrit.Gerrit) (RepoManager, error) {
	wd := path.Join(workdir, "repo_manager")
	parentBase := strings.TrimSuffix(path.Base(parentRepo), ".git")
	parentDir := path.Join(wd, parentBase)

	user, err := g.GetUserEmail()
	if err != nil {
		return nil, fmt.Errorf("Failed to determine Gerrit user: %s", err)
	}
	sklog.Infof("Repo Manager user: %s", user)

	mr := &manifestRepoManager{
		depotToolsRepoManager: &depotToolsRepoManager{
			commonRepoManager: &commonRepoManager{
				parentBranch: parentBranch,
				childDir:     path.Join(wd, childPath),
				childPath:    childPath,
				childRepo:    nil, // This will be filled in on the first update.
				childBranch:  childBranch,
				user:         user,
				workdir:      wd,
				g:            g,
			},
			depot_tools: depot_tools,
			gclient:     path.Join(depot_tools, "gclient"),
			parentDir:   parentDir,
			parentRepo:  parentRepo,
		},
	}
	if err := mr.update(); err != nil {
		return nil, err
	}
	go func() {
		for range time.Tick(frequency) {
			util.LogErr(mr.update())
		}
	}()
	return mr, nil
}

//// TODO(rmistry): Make this a util and move to gclientGetEnv
//// getEnv returns the environment used for most commands.
//func getEnv(depotTools string) []string {
//	return []string{
//		fmt.Sprintf("PATH=%s:%s", depotTools, os.Getenv("PATH")),
//		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
//		fmt.Sprintf("SKIP_GCE_AUTH_FOR_GIT=1"),
//	}
//}

// TODO(rmistry): Make this a util and move to gclientCleanParnet
// cleanParent forces the parent checkout into a clean state.
func (mr *manifestRepoManager) cleanParent() error {
	if _, err := exec.RunCwd(mr.parentDir, "git", "clean", "-d", "-f", "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(mr.parentDir, "git", "rebase", "--abort")
	if _, err := exec.RunCwd(mr.parentDir, "git", "checkout", fmt.Sprintf("origin/%s", mr.parentBranch), "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(mr.parentDir, "git", "branch", "-D", MANIFEST_ROLL_BRANCH)
	if _, err := exec.RunCommand(&exec.Command{
		Dir:  mr.workdir,
		Env:  getEnv(mr.depot_tools),
		Name: mr.gclient,
		Args: []string{"revert", "--nohooks"},
	}); err != nil {
		return err
	}
	return nil
}

// TODO(rmistry): Make sure that the childrepo is updated properly
// TODO(rmistry): Move majority of the below somewhere.
// update syncs code in the relevant repositories.
func (mr *manifestRepoManager) update() error {
	// Sync the projects.
	mr.repoMtx.Lock()
	defer mr.repoMtx.Unlock()

	// Create the working directory if needed.
	if _, err := os.Stat(mr.workdir); err != nil {
		if err := os.MkdirAll(mr.workdir, 0755); err != nil {
			return err
		}
	}

	if _, err := os.Stat(path.Join(mr.parentDir, ".git")); err == nil {
		if err := mr.cleanParent(); err != nil {
			return err
		}
		// Update the repo.
		if _, err := exec.RunCwd(mr.parentDir, "git", "fetch"); err != nil {
			return err
		}
		if _, err := exec.RunCwd(mr.parentDir, "git", "reset", "--hard", fmt.Sprintf("origin/%s", mr.parentBranch)); err != nil {
			return err
		}
	}

	if _, err := exec.RunCommand(&exec.Command{
		Dir:  mr.workdir,
		Env:  getEnv(mr.depot_tools),
		Name: mr.gclient,
		Args: []string{"config", mr.parentRepo},
	}); err != nil {
		return err
	}
	if _, err := exec.RunCommand(&exec.Command{
		Dir:  mr.workdir,
		Env:  getEnv(mr.depot_tools),
		Name: mr.gclient,
		Args: []string{"sync", "--nohooks"},
	}); err != nil {
		return err
	}

	// HERE HERE HERE
	// Create the child GitInfo if needed.
	var err error
	fmt.Println("ABOUT TO CREATE CHILD!!")
	if mr.childRepo == nil {
		mr.childRepo, err = git.NewCheckout(common.REPO_SKIA, mr.workdir)
		if err != nil {
			return err
		}
	}
	if err := mr.childRepo.Update(); err != nil {
		return err
	}
	fmt.Println("CREATED CHILD!!")

	// Get the last roll revision.
	lastRollRev, err := mr.getLastRollRev()
	if err != nil {
		return err
	}

	// Record child HEAD
	childHead, err := mr.getChildRepoHead()
	if err != nil {
		return err
	}
	mr.infoMtx.Lock()
	defer mr.infoMtx.Unlock()
	mr.lastRollRev = lastRollRev
	mr.childHead = childHead

	// TODO(rmistry): Remove the below.
	fmt.Println(lastRollRev)
	fmt.Println(childHead)
	fmt.Println(mr.childRepo)
	return nil
}

// ForceUpdate forces the repoManager to update.
func (mr *manifestRepoManager) ForceUpdate() error {
	return mr.update()
}

// TODO(rmistry): File a bug to make the below simpler by using some tool maybe.
// getLastRollRev returns the commit hash of the last-completed DEPS roll.
func (mr *manifestRepoManager) getLastRollRev() (string, error) {
	// Parse the manifest file to
	content, err := ioutil.ReadFile(filepath.Join(mr.parentDir, MANIFEST_FILE_NAME))
	if err != nil {
		return "", fmt.Errorf("Could not read from %s: %s", MANIFEST_FILE_NAME, err)
	}
	// fmt.Println(string(content))
	// TODO(rmistry): Skia should be configurable below!
	regex := regexp.MustCompile(`(?sm)skia(.*?)revision="(.*?)"`)
	m := regex.FindStringSubmatch(string(content))
	if m == nil {
		return "", fmt.Errorf("Could not find target revision from %s", MANIFEST_FILE_NAME)
	}
	return m[len(m)-1], nil
}

// getChildRepoHead returns the commit hash of the latest commit in the child repo.
func (mr *manifestRepoManager) getChildRepoHead() (string, error) {
	output, err := exec.RunCwd(mr.parentDir, "git", "ls-remote", common.REPO_SKIA, fmt.Sprintf("refs/heads/%s", mr.childBranch), "-1")
	if err != nil {
		return "", err
	}
	tokens := strings.Split(output, "\t")
	return tokens[0], nil
}

// CreateNewRoll creates and uploads a new DEPS roll to the given commit.
// Returns the issue number of the uploaded roll.
func (mr *manifestRepoManager) CreateNewRoll(strategy string, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	mr.repoMtx.Lock()
	defer mr.repoMtx.Unlock()

	// Clean the checkout, get onto a fresh branch.
	if err := mr.cleanParent(); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(mr.parentDir, "git", "checkout", "-b", DEPS_ROLL_BRANCH, "-t", fmt.Sprintf("origin/%s", mr.parentBranch), "-f"); err != nil {
		return 0, err
	}

	// Defer some more cleanup.
	defer func() {
		util.LogErr(mr.cleanParent())
	}()

	// Create the roll CL.

	// Determine what commit we're rolling to.
	cr := mr.childRepo
	commits, err := cr.RevList(fmt.Sprintf("%s..%s", mr.lastRollRev, mr.childHead))
	if err != nil {
		return 0, fmt.Errorf("Failed to list revisions: %s", err)
	}
	rollTo := mr.childHead
	if strategy == ROLL_STRATEGY_SINGLE {
		rollTo = commits[len(commits)-1]
		commits = commits[len(commits)-1:]
	}

	if _, err := exec.RunCwd(mr.parentDir, "git", "config", "user.name", mr.user); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(mr.parentDir, "git", "config", "user.email", mr.user); err != nil {
		return 0, err
	}

	// Update the manifest file.
	if err := mr.updateManifestFile(mr.lastRollRev, rollTo); err != nil {
		return 0, err
	}

	// Get list of changes.
	changeSummaries := []string{}
	for _, c := range commits {
		d, err := cr.Details(c)
		if err != nil {
			return 0, err
		}
		changeSummary := fmt.Sprintf("%s %s %s", d.Timestamp.Format("2006-01-02"), AUTHOR_EMAIL_RE.FindStringSubmatch(d.Author)[1], d.Subject)
		changeSummaries = append(changeSummaries, changeSummary)
	}

	// Create commit message.
	commitRange := fmt.Sprintf("%s..%s", mr.lastRollRev[:9], rollTo[:9])
	childRepoName := path.Base(mr.childDir)
	commitMsg := fmt.Sprintf(
		`Roll %s %s (%d commits)

https://%s.googlesource.com/%s.git/+log/%s

%s

`, mr.childPath, commitRange, len(commits), childRepoName, childRepoName, commitRange, strings.Join(changeSummaries, "\n"))

	fmt.Println("SLEEPING")
	//time.Sleep(2 * time.Minute)
	// Commit the change with the above message.
	if _, addErr := exec.RunCwd(mr.parentDir, "git", "add", MANIFEST_FILE_NAME); addErr != nil {
		return 0, fmt.Errorf("Failed to git add: %s", addErr)
	}
	if _, commitErr := exec.RunCwd(mr.parentDir, "git", "commit", "-m", commitMsg); commitErr != nil {
		return 0, fmt.Errorf("Failed to commit: %s", commitErr)
	}

	// Upload the CL to Gerrit.
	uploadCmd := &exec.Command{
		Dir:  mr.parentDir,
		Env:  getEnv(mr.depot_tools),
		Name: "git",
		Args: []string{"cl", "upload", "--bypass-hooks", "-f", "-v", "-v"},
	}
	if dryRun {
		uploadCmd.Args = append(uploadCmd.Args, "--cq-dry-run")
	} else {
		uploadCmd.Args = append(uploadCmd.Args, "--use-commit-queue")
	}
	uploadCmd.Args = append(uploadCmd.Args, "--gerrit")
	// TODO(rmistry): Do not add? may have to make a separate call for CR+2 and CQ+2??
	tbr := "\nTBR="
	if emails != nil && len(emails) > 0 {
		emailStr := strings.Join(emails, ",")
		tbr += emailStr
		uploadCmd.Args = append(uploadCmd.Args, "--send-mail", "--cc", emailStr)
	}
	commitMsg += tbr
	uploadCmd.Args = append(uploadCmd.Args, "-m", commitMsg)

	// Upload the CL.
	sklog.Infof("Running command: git %s", strings.Join(uploadCmd.Args, " "))
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
		Dir:  mr.parentDir,
		Env:  getEnv(mr.depot_tools),
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
	// return -1, fmt.Errorf("FAIL!!!!!!!!!!!!!!!!!!!!!!!!")
}

func (mr *manifestRepoManager) updateManifestFile(prevHash, newHash string) error {
	manifestFilePath := filepath.Join(mr.parentDir, MANIFEST_FILE_NAME)
	sklog.Infof("Updating %s from %s to %s", manifestFilePath, prevHash, newHash)
	content, err := ioutil.ReadFile(manifestFilePath)
	if err != nil {
		return fmt.Errorf("Could not read from %s: %s", manifestFilePath, err)
	}
	newContent := strings.Replace(string(content), prevHash, newHash, 1)
	if err := ioutil.WriteFile(manifestFilePath, []byte(newContent), os.ModePerm); err != nil {
		return fmt.Errorf("Could not write to %s: %s", manifestFilePath, err)
	}
	return nil
}

func (mr *manifestRepoManager) SendToGerritCQ(change *gerrit.ChangeInfo, comment string) error {
	return mr.g.SendToCQ(change, "")
}

func (mr *manifestRepoManager) SendToGerritDryRun(change *gerrit.ChangeInfo, comment string) error {
	return mr.g.SendToDryRun(change, "")
}
