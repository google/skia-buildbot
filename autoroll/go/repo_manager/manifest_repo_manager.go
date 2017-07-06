package repo_manager

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	MANIFEST_FILE_NAME = "userspace"
)

var (
	// Use this function to instantiate a RepoManager. This is able to be
	// overridden for testing.
	NewManifestRepoManager func(string, string, string, string, string, string, *gerrit.Gerrit, string) (RepoManager, error) = newManifestRepoManager
)

// manifestRepoManager is a struct used by Manifest AutoRoller for managing checkouts.
type manifestRepoManager struct {
	*depotToolsRepoManager
}

// newManifestRepoManager returns a RepoManager instance which operates in the
// given working directory and updates at the given frequency.
func newManifestRepoManager(workdir, parentRepo, parentBranch, childPath, childBranch string, depot_tools string, g *gerrit.Gerrit, strategy string) (RepoManager, error) {
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
				strategy:     strategy,
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
	// TODO(borenet): This update can be extremely expensive. Consider
	// moving it out of the startup critical path.
	return mr, mr.Update()
}

// Update syncs code in the relevant repositories.
func (mr *manifestRepoManager) Update() error {
	// Sync the projects.
	mr.repoMtx.Lock()
	defer mr.repoMtx.Unlock()

	if err := mr.createAndSyncParent(); err != nil {
		return fmt.Errorf("Could not create and sync parent repo: %s", err)
	}

	// Create the child GitInfo if needed.
	var err error
	if mr.childRepo == nil {
		mr.childRepo, err = git.NewCheckout(common.REPO_SKIA, mr.workdir)
		if err != nil {
			return err
		}
	}
	if err := mr.childRepo.Update(); err != nil {
		return err
	}

	// Get the last roll revision.
	lastRollRev, err := mr.getLastRollRev()
	if err != nil {
		return err
	}

	// Get the next roll revision.
	childHead, err := mr.childRepo.FullHash(fmt.Sprintf("origin/%s", mr.childBranch))
	if err != nil {
		return err
	}
	var nextRollRev string
	if mr.strategy == ROLL_STRATEGY_SINGLE {
		commits, err := mr.childRepo.RevList(fmt.Sprintf("%s..%s", lastRollRev, childHead))
		if err != nil {
			return fmt.Errorf("Failed to list revisions: %s", err)
		}
		if len(commits) == 0 {
			nextRollRev = lastRollRev
		} else {
			nextRollRev = commits[len(commits)-1]
		}
	} else {
		nextRollRev = childHead
	}
	mr.infoMtx.Lock()
	defer mr.infoMtx.Unlock()
	mr.lastRollRev = lastRollRev
	mr.nextRollRev = nextRollRev
	return nil
}

// getLastRollRev returns the commit hash of the last-completed DEPS roll.
// TODO(rmistry): File a bug to Fuchsia infra to make the below simpler by
//                add a tool similar to roll-dep.
func (mr *manifestRepoManager) getLastRollRev() (string, error) {
	// Parse the manifest file to extract the child repo revision.
	content, err := ioutil.ReadFile(filepath.Join(mr.parentDir, MANIFEST_FILE_NAME))
	if err != nil {
		return "", fmt.Errorf("Could not read from %s: %s", MANIFEST_FILE_NAME, err)
	}
	childRepoName := path.Base(mr.childDir)
	regex := regexp.MustCompile(fmt.Sprintf(`(?sm)%s(.*?)revision="(.*?)"`, childRepoName))
	m := regex.FindStringSubmatch(string(content))
	if m == nil {
		return "", fmt.Errorf("Could not find target revision from %s", MANIFEST_FILE_NAME)
	}
	return m[len(m)-1], nil
}

// CreateNewRoll creates and uploads a new DEPS roll to the given commit.
// Returns the issue number of the uploaded roll.
func (mr *manifestRepoManager) CreateNewRoll(from, to string, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	mr.repoMtx.Lock()
	defer mr.repoMtx.Unlock()

	// Clean the checkout, get onto a fresh branch.
	if err := mr.cleanParent(); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(mr.parentDir, "git", "checkout", "-b", ROLL_BRANCH, "-t", fmt.Sprintf("origin/%s", mr.parentBranch), "-f"); err != nil {
		return 0, err
	}

	// Defer some more cleanup.
	defer func() {
		util.LogErr(mr.cleanParent())
	}()

	// Create the roll CL.
	cr := mr.childRepo
	commits, err := cr.RevList(fmt.Sprintf("%s..%s", from, to))
	if err != nil {
		return 0, fmt.Errorf("Failed to list revisions: %s", err)
	}

	if _, err := exec.RunCwd(mr.parentDir, "git", "config", "user.name", mr.user); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(mr.parentDir, "git", "config", "user.email", mr.user); err != nil {
		return 0, err
	}

	// Update the manifest file.
	if err := mr.updateManifestFile(mr.lastRollRev, to); err != nil {
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
	commitRange := fmt.Sprintf("%s..%s", from[:9], to[:9])
	childRepoName := path.Base(mr.childDir)
	commitMsg := fmt.Sprintf(
		`Roll %s %s (%d commits)

https://%s.googlesource.com/%s.git/+log/%s

%s

`, mr.childPath, commitRange, len(commits), childRepoName, childRepoName, commitRange, strings.Join(changeSummaries, "\n"))

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
		Env:  mr.GetEnvForDepotTools(),
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
		Env:  mr.GetEnvForDepotTools(),
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
