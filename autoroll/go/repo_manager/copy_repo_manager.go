package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	COPY_VERSION_HASH_FILE = "version.sha1"
)

var (
	// Use this function to instantiate a RepoManager. This is able to be
	// overridden for testing.
	NewCopyRepoManager func(context.Context, *CopyRepoManagerConfig, string, *gerrit.Gerrit, string, string, *http.Client, codereview.CodeReview, bool) (RepoManager, error) = newCopyRepoManager
)

type CopyEntry struct {
	SrcRelPath string `json:"srcRelPath"`
	DstRelPath string `json:"dstRelPath"`
}

// CopyRepoManagerConfig provides configuration for the copy
// RepoManager.
type CopyRepoManagerConfig struct {
	DepotToolsRepoManagerConfig

	// ChildRepo is the URL of the child repo.
	ChildRepo string `json:"childRepo"`

	// Optional fields.

	// Copies indicates which files and directories to copy from the
	// child repo into the parent repo. If not specified, the whole repo
	// is copied.
	Copies []CopyEntry `json:"copies,omitempty"`
}

// Validate the config.
func (c *CopyRepoManagerConfig) Validate() error {
	if c.ChildRepo == "" {
		return fmt.Errorf("ChildRepo is required.")
	}
	return c.DepotToolsRepoManagerConfig.Validate()
}

type copyRepoManager struct {
	*depotToolsRepoManager
	childRepoUrl string
	versionFile  string
	copies       []CopyEntry
}

// newCopyRepoManager returns a RepoManager instance which rolls a dependency
// which is copied directly into a subdir of the parent repo.
func newCopyRepoManager(ctx context.Context, c *CopyRepoManagerConfig, workdir string, g *gerrit.Gerrit, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	wd := path.Join(workdir, "repo_manager")
	drm, err := newDepotToolsRepoManager(ctx, c.DepotToolsRepoManagerConfig, wd, recipeCfgFile, serverURL, g, client, cr, local)
	if err != nil {
		return nil, err
	}
	childRepo, err := git.NewCheckout(ctx, c.ChildRepo, wd)
	if err != nil {
		return nil, err
	}
	drm.childRepo = childRepo
	rm := &copyRepoManager{
		depotToolsRepoManager: drm,
		childRepoUrl:          c.ChildRepo,
		versionFile:           path.Join(drm.childDir, COPY_VERSION_HASH_FILE),
		copies:                c.Copies,
	}
	return rm, nil
}

// See documentation for RepoManager interface.
func (rm *copyRepoManager) Update(ctx context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	// Sync the projects.
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()
	if err := rm.createAndSyncParent(ctx); err != nil {
		return nil, nil, nil, fmt.Errorf("Could not create and sync parent repo: %s", err)
	}

	// In this type of repo manager, the child repo is managed separately
	// from the parent.
	if err := rm.childRepo.Update(ctx); err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to update child repo: %s", err)
	}

	// Get the last roll revision.
	lastRollRevBytes, err := ioutil.ReadFile(rm.versionFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Failed to read %s: %s", rm.versionFile, err)
	}
	lastRollHash := strings.TrimSpace(string(lastRollRevBytes))
	details, err := rm.childRepo.Details(ctx, lastRollHash)
	if err != nil {
		return nil, nil, nil, err
	}
	lastRollRev := revision.FromLongCommit(rm.childRevLinkTmpl, details)

	// Get the tip-of-tree revision.
	tipRev, err := rm.getTipRev(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	// Find the not-rolled child repo commits.
	notRolledRevs, err := rm.getCommitsNotRolled(ctx, lastRollRev, tipRev)
	if err != nil {
		return nil, nil, nil, err
	}

	return lastRollRev, tipRev, notRolledRevs, nil
}

// See documentation for RepoManager interface.
func (rm *copyRepoManager) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()

	// Clean the checkout, get onto a fresh branch.
	if err := rm.cleanParent(ctx); err != nil {
		return 0, err
	}
	parentRepo := git.GitDir(rm.parentDir)
	if _, err := parentRepo.Git(ctx, "checkout", "-b", ROLL_BRANCH, "-t", fmt.Sprintf("origin/%s", rm.parentBranch), "-f"); err != nil {
		return 0, err
	}

	// Defer some more cleanup.
	defer func() {
		util.LogErr(rm.cleanParent(ctx))
	}()

	if !rm.local {
		if _, err := parentRepo.Git(ctx, "config", "user.name", rm.codereview.UserName()); err != nil {
			return 0, err
		}
		if _, err := parentRepo.Git(ctx, "config", "user.email", rm.codereview.UserEmail()); err != nil {
			return 0, err
		}
	}

	// Roll the dependency.
	if _, err := rm.childRepo.Git(ctx, "reset", "--hard", to.Id); err != nil {
		return 0, err
	}
	childFullPath := path.Join(rm.workdir, rm.childPath)
	childRelPath, err := filepath.Rel(parentRepo.Dir(), childFullPath)
	if err != nil {
		return 0, err
	}
	if _, err := parentRepo.Git(ctx, "rm", "-r", childRelPath); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(path.Dir(childFullPath), os.ModePerm); err != nil {
		return 0, err
	}
	if len(rm.copies) > 0 {
		for _, c := range rm.copies {
			src := path.Join(rm.childRepo.Dir(), c.SrcRelPath)
			dst := path.Join(parentRepo.Dir(), c.DstRelPath)
			dstDir := path.Dir(dst)
			if err := os.MkdirAll(dstDir, os.ModePerm); err != nil {
				return 0, err
			}
			if _, err := exec.RunCwd(ctx, rm.workdir, "cp", "-rT", src, dst); err != nil {
				return 0, err
			}
		}
	} else {
		if _, err := exec.RunCwd(ctx, rm.workdir, "cp", "-rT", rm.childRepo.Dir(), childFullPath); err != nil {
			return 0, err
		}
	}
	if err := os.RemoveAll(path.Join(childFullPath, ".git")); err != nil {
		return 0, err
	}
	if err := ioutil.WriteFile(rm.versionFile, []byte(to.Id), os.ModePerm); err != nil {
		return 0, err
	}
	if _, err := parentRepo.Git(ctx, "add", parentRepo.Dir()); err != nil {
		return 0, err
	}

	// Build the commit message.
	commitMsg, err := rm.buildCommitMsg(&CommitMsgVars{
		ChildPath:      rm.childPath,
		ChildRepo:      rm.childRepoUrl,
		CqExtraTrybots: cqExtraTrybots,
		Reviewers:      emails,
		Revisions:      rolling,
		RollingFrom:    from,
		RollingTo:      to,
		ServerURL:      rm.serverURL,
	})
	if err != nil {
		return 0, err
	}
	if _, err := parentRepo.Git(ctx, "commit", "-a", "-m", commitMsg); err != nil {
		return 0, err
	}

	// Run the pre-upload steps.
	for _, s := range rm.preUploadSteps {
		if err := s(ctx, nil, rm.httpClient, rm.parentDir); err != nil {
			return 0, fmt.Errorf("Failed pre-upload step: %s", err)
		}
	}

	// Upload the CL.
	gitExec, err := git.Executable(ctx)
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	uploadCmd := &exec.Command{
		Dir:     rm.parentDir,
		Env:     rm.depotToolsEnv,
		Name:    gitExec,
		Args:    []string{"cl", "upload", "--bypass-hooks", "--squash", "-f", "-v", "-v"},
		Timeout: 2 * time.Minute,
	}
	if dryRun {
		uploadCmd.Args = append(uploadCmd.Args, "--cq-dry-run")
	} else {
		uploadCmd.Args = append(uploadCmd.Args, "--use-commit-queue")
	}
	uploadCmd.Args = append(uploadCmd.Args, "--gerrit")
	if emails != nil && len(emails) > 0 {
		emailStr := strings.Join(emails, ",")
		uploadCmd.Args = append(uploadCmd.Args, "--send-mail", fmt.Sprintf("--tbrs=%s", emailStr))
	}
	uploadCmd.Args = append(uploadCmd.Args, "-m", commitMsg)

	sklog.Infof("Running command: git %s", strings.Join(uploadCmd.Args, " "))
	if _, err := exec.RunCommand(ctx, uploadCmd); err != nil {
		return 0, err
	}

	// Obtain the issue number.
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return 0, err
	}
	defer util.RemoveAll(tmp)
	jsonFile := path.Join(tmp, "issue.json")
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  rm.parentDir,
		Env:  rm.depotToolsEnv,
		Name: gitExec,
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

	// Mark the change as ready for review, if necessary.
	if err := rm.unsetWIP(ctx, nil, issue.Issue); err != nil {
		return 0, err
	}

	return issue.Issue, nil
}
