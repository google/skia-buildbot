package repo_manager

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/android_skia_checkout"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	UPSTREAM_REMOTE_NAME = "remote"
	REPO_BRANCH_NAME     = "merge"
)

var (
	AUTHOR_EMAIL_RE = regexp.MustCompile(".* \\((.*)\\)")

	DELETE_MERGE_CONFLICT_FILES = []string{android_skia_checkout.SkUserConfigRelPath}
)

// ProjectMetadataFileConfig provides configuration for METADATA files in the Android repo.
type ProjectMetadataFileConfig struct {
	FilePath    string `json:"filePath"`
	Name        string `json:"projectName"`
	Description string `json:"projectDescription"`
	HomePage    string `json:"projectHomePage"`
	GitURL      string `json:"projectGitURL"`
	LicenseType string `json:"projectLicenseType"`
}

// Validate implements util.Validator.
func (c *ProjectMetadataFileConfig) Validate() error {
	if c.FilePath == "" || c.Name == "" || c.Description == "" || c.HomePage == "" || c.GitURL == "" || c.LicenseType == "" {
		return errors.New("All parts of ProjectMetadataFileConfig are required")
	}
	return nil
}

// ProjectMetadataFileConfigToProto converts a ProjectMetadataFileConfig to a
// config.AndroidRepoManagerConfig_ProjectMetadataFileConfig.
func ProjectMetadataFileConfigToProto(cfg *ProjectMetadataFileConfig) *config.AndroidRepoManagerConfig_ProjectMetadataFileConfig {
	if cfg == nil {
		return nil
	}
	return &config.AndroidRepoManagerConfig_ProjectMetadataFileConfig{
		FilePath:    cfg.FilePath,
		Name:        cfg.Name,
		Description: cfg.Description,
		HomePage:    cfg.HomePage,
		GitUrl:      cfg.GitURL,
		LicenseType: cfg.LicenseType,
	}
}

// ProtoToProjectMetadataFileConfig converts a
// config.AndroidRepoManagerConfig_ProjectMetadataFileConfig to a
// ProjectMetadataFileConfig.
func ProtoToProjectMetadataFileConfig(cfg *config.AndroidRepoManagerConfig_ProjectMetadataFileConfig) *ProjectMetadataFileConfig {
	if cfg == nil {
		return nil
	}
	return &ProjectMetadataFileConfig{
		FilePath:    cfg.FilePath,
		Name:        cfg.Name,
		Description: cfg.Description,
		HomePage:    cfg.HomePage,
		GitURL:      cfg.GitUrl,
		LicenseType: cfg.LicenseType,
	}
}

// AndroidRepoManagerConfig provides configuration for the Android RepoManager.
type AndroidRepoManagerConfig struct {
	CommonRepoManagerConfig
	*ProjectMetadataFileConfig `json:"projectMetadataFileConfig,omitempty"`
	ChildRepoURL               string `json:"childRepoURL"`
}

// Validate implements util.Validator.
func (c *AndroidRepoManagerConfig) Validate() error {
	if err := c.CommonRepoManagerConfig.Validate(); err != nil {
		return err
	}
	if c.ProjectMetadataFileConfig != nil {
		if err := c.ProjectMetadataFileConfig.Validate(); err != nil {
			return err
		}
	}
	if c.ChildRepoURL == "" {
		return errors.New("childRepoURL must be specified")
	}
	return nil
}

// ValidStrategies implements roller.RepoManagerConfig.
func (c *AndroidRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
		strategy.ROLL_STRATEGY_N_BATCH,
	}
}

// AndroidRepoManagerConfigToProto converts an AndroidRepoManagerConfig to a
// config.AndroidRepoManagerConfig.
func AndroidRepoManagerConfigToProto(cfg *AndroidRepoManagerConfig) *config.AndroidRepoManagerConfig {
	return &config.AndroidRepoManagerConfig{
		ChildRepoUrl:     cfg.ChildRepoURL,
		ChildBranch:      cfg.ChildBranch.RawTemplate(),
		ChildPath:        cfg.ChildPath,
		ParentRepoUrl:    cfg.ParentRepo,
		ParentBranch:     cfg.ParentBranch.RawTemplate(),
		ChildRevLinkTmpl: cfg.ChildRevLinkTmpl,
		ChildSubdir:      cfg.ChildSubdir,
		PreUploadSteps:   parent.PreUploadStepsToProto(cfg.PreUploadSteps),
		Metadata:         ProjectMetadataFileConfigToProto(cfg.ProjectMetadataFileConfig),
	}
}

// ProtoToAndroidRepoManagerConfig converts a config.AndroidRepoManagerConfig to
// an AndroidRepoManagerConfig.
func ProtoToAndroidRepoManagerConfig(cfg *config.AndroidRepoManagerConfig) (*AndroidRepoManagerConfig, error) {
	childBranch, err := config_vars.NewTemplate(cfg.ChildBranch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentBranch, err := config_vars.NewTemplate(cfg.ParentBranch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &AndroidRepoManagerConfig{
		CommonRepoManagerConfig: CommonRepoManagerConfig{
			ChildBranch:      childBranch,
			ChildPath:        cfg.ChildPath,
			ParentBranch:     parentBranch,
			ParentRepo:       cfg.ParentRepoUrl,
			ChildRevLinkTmpl: cfg.ChildRevLinkTmpl,
			ChildSubdir:      cfg.ChildSubdir,
			PreUploadSteps:   parent.ProtoToPreUploadSteps(cfg.PreUploadSteps),
		},
		ProjectMetadataFileConfig: ProtoToProjectMetadataFileConfig(cfg.Metadata),
		ChildRepoURL:              cfg.ChildRepoUrl,
	}, nil
}

// androidRepoManager is a struct used by Android AutoRoller for managing
// checkouts.
type androidRepoManager struct {
	*commonRepoManager
	androidRemoteName string
	childRepoURL      string
	parentRepoURL     string
	repoToolPath      string

	projectMetadataFileConfig *ProjectMetadataFileConfig
}

// NewAndroidRepoManager returns an androidRepoManager instance.
func NewAndroidRepoManager(ctx context.Context, c *AndroidRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, serverURL, serviceAccount string, client *http.Client, cr codereview.CodeReview, isInternal, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	user, err := user.Current()
	if err != nil {
		return nil, err
	}
	repoToolDir := path.Join(user.HomeDir, "bin")
	repoToolPath := path.Join(repoToolDir, "repo")
	if _, err := os.Stat(repoToolDir); err != nil {
		if err := os.MkdirAll(repoToolDir, 0755); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(repoToolPath); err != nil {
		// Download the repo tool.
		if _, err := exec.RunCwd(ctx, repoToolDir, "wget", "https://storage.googleapis.com/git-repo-downloads/repo", "-O", repoToolPath); err != nil {
			return nil, err
		}
		// Make the repo tool executable.
		if _, err := exec.RunCwd(ctx, repoToolDir, "chmod", "a+x", repoToolPath); err != nil {
			return nil, err
		}
	}

	wd := path.Join(workdir, "android_repo")
	crm, err := newCommonRepoManager(ctx, c.CommonRepoManagerConfig, reg, wd, serverURL, g, client, cr, local)
	if err != nil {
		return nil, err
	}
	androidRemoteName := "aosp"
	if isInternal {
		androidRemoteName = "goog"
	}
	r := &androidRepoManager{
		commonRepoManager:         crm,
		androidRemoteName:         androidRemoteName,
		parentRepoURL:             g.GetRepoUrl(),
		repoToolPath:              repoToolPath,
		projectMetadataFileConfig: c.ProjectMetadataFileConfig,
		childRepoURL:              c.ChildRepoURL,
	}
	return r, nil
}

// Helper function for updating the Android checkout.
func (r *androidRepoManager) updateAndroidCheckout(ctx context.Context) error {
	// Create the working directory if needed.
	if _, err := os.Stat(r.workdir); err != nil {
		if err := os.MkdirAll(r.workdir, 0755); err != nil {
			return err
		}
	}

	// Run repo init and sync commands.
	if _, err := exec.RunCwd(ctx, r.workdir, r.repoToolPath, "init", "-u", fmt.Sprintf("%s/a/platform/manifest", r.parentRepoURL), "-g", "all,-notdefault,-darwin", "-b", r.parentBranch.String()); err != nil {
		return err
	}
	// Sync only the child path and the repohooks directory (needed to upload changes).
	if _, err := exec.RunCwd(ctx, r.workdir, r.repoToolPath, "sync", "--force-sync", r.childPath, "tools/repohooks", "-j32"); err != nil {
		return err
	}

	// Set color.ui=true so that the repo tool does not prompt during upload.
	if _, err := r.childRepo.Git(ctx, "config", "color.ui", "true"); err != nil {
		return err
	}

	// Fix the review config to a URL which will work outside prod.
	if _, err := r.childRepo.Git(ctx, "config", fmt.Sprintf("remote.%s.review", r.androidRemoteName), fmt.Sprintf("%s/", r.parentRepoURL)); err != nil {
		return err
	}

	// Check to see whether there is an upstream yet.
	remoteOutput, err := r.childRepo.Git(ctx, "remote", "show")
	if err != nil {
		return err
	}
	if !strings.Contains(remoteOutput, UPSTREAM_REMOTE_NAME) {
		if _, err := r.childRepo.Git(ctx, "remote", "add", UPSTREAM_REMOTE_NAME, r.childRepoURL); err != nil {
			return err
		}
	}

	// Update the remote to make sure that all new branches are available.
	if _, err := r.childRepo.Git(ctx, "remote", "update", UPSTREAM_REMOTE_NAME, "--prune"); err != nil {
		return err
	}
	return nil
}

// See documentation for RepoManager interface.
func (r *androidRepoManager) Update(ctx context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	// Sync the projects.
	r.repoMtx.Lock()
	defer r.repoMtx.Unlock()
	if err := r.updateAndroidCheckout(ctx); err != nil {
		return nil, nil, nil, err
	}

	// Get the last roll revision.
	lastRollRev, err := r.getLastRollRev(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	// Get the tip-of-tree revision.
	tipRev, err := r.getTipRev(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	// Find the not-rolled child repo commits.
	notRolledRevs, err := r.getCommitsNotRolled(ctx, lastRollRev, tipRev)
	if err != nil {
		return nil, nil, nil, err
	}

	return lastRollRev, tipRev, notRolledRevs, nil
}

// getLastRollRev returns the last-completed DEPS roll Revision.
func (r *androidRepoManager) getLastRollRev(ctx context.Context) (*revision.Revision, error) {
	output, err := r.childRepo.Git(ctx, "merge-base", fmt.Sprintf("refs/remotes/remote/%s", r.childBranch), fmt.Sprintf("refs/remotes/%s/%s", r.androidRemoteName, r.parentBranch))
	if err != nil {
		return nil, err
	}
	details, err := r.childRepo.Details(ctx, strings.TrimRight(output, "\n"))
	if err != nil {
		return nil, err
	}
	return revision.FromLongCommit(r.childRevLinkTmpl, details), nil
}

// abortMerge aborts the current merge in the child repo.
func (r *androidRepoManager) abortMerge(ctx context.Context) error {
	_, err := r.childRepo.Git(ctx, "merge", "--abort")
	return err
}

// abandonRepoBranch abandons the repo branch.
func (r *androidRepoManager) abandonRepoBranch(ctx context.Context) error {
	_, err := exec.RunCwd(ctx, r.childRepo.Dir(), r.repoToolPath, "abandon", REPO_BRANCH_NAME)
	return err
}

// getChangeNumForHash returns the corresponding change number for the provided commit hash by querying Gerrit's search API.
func (r *androidRepoManager) getChangeForHash(hash string) (*gerrit.ChangeInfo, error) {
	issues, err := r.g.Search(context.TODO(), 1, false, gerrit.SearchCommit(hash))
	if err != nil {
		return nil, err
	}
	return r.g.GetIssueProperties(context.TODO(), issues[0].Issue)
}

// setTopic sets a topic using the name of the child repo and the change number.
// Example: skia_merge_1234
func (r *androidRepoManager) setTopic(changeNum int64) error {
	topic := fmt.Sprintf("%s_merge_%d", path.Base(r.childDir), changeNum)
	return r.g.SetTopic(context.TODO(), topic, changeNum)
}

// See documentation for RepoManager interface.
func (r *androidRepoManager) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, dryRun bool, commitMsg string) (int64, error) {
	r.repoMtx.Lock()
	defer r.repoMtx.Unlock()

	parentBranch := r.parentBranch.String()

	// Update the upstream remote.
	if _, err := r.childRepo.Git(ctx, "fetch", UPSTREAM_REMOTE_NAME); err != nil {
		return 0, err
	}

	// Create the roll CL.

	// Start the merge.
	mergeTarget := to.Id
	if strings.HasPrefix(to.Id, gerrit.CHANGE_REF_PREFIX) {
		if err := r.childRepo.FetchRefFromRepo(ctx, r.childRepoURL, to.Id); err != nil {
			return 0, fmt.Errorf("Failed to fetch ref in %s: %s", r.childRepo.Dir(), err)
		}
		mergeTarget = "FETCH_HEAD"
	}
	if _, err := r.childRepo.Git(ctx, "merge", mergeTarget, "--no-commit"); err != nil {
		// Check to see if this was a merge conflict with ignoreMergeConflictFiles and deleteMergeConflictFiles.
		conflictsOutput, conflictsErr := r.childRepo.Git(ctx, "diff", "--name-only", "--diff-filter=U")
		if conflictsErr != nil || conflictsOutput == "" {
			util.LogErr(conflictsErr)
			return 0, fmt.Errorf("Failed to roll to %s. Needs human investigation: %s", to, err)
		}
		for _, conflict := range strings.Split(conflictsOutput, "\n") {
			if conflict == "" {
				continue
			}
			ignoreConflict := false
			for _, del := range DELETE_MERGE_CONFLICT_FILES {
				if conflict == del {
					_, resetErr := r.childRepo.Git(ctx, "reset", "--", del)
					util.LogErr(resetErr)
					_, delErr := exec.RunCwd(ctx, r.childDir, "rm", del)
					util.LogErr(delErr)
					ignoreConflict = true
					sklog.Infof("Deleting %s due to merge conflict", conflict)
					break
				}
			}
			if !ignoreConflict {
				util.LogErr(r.abortMerge(ctx))
				return 0, fmt.Errorf("Failed to roll to %s. Conflicts in %s: %s", to, conflictsOutput, err)
			}
		}
	}

	if r.projectMetadataFileConfig != nil {
		// Populate the METADATA file.
		d := time.Now()
		metadataContents := fmt.Sprintf(`name: "%s"
description: "%s"
third_party {
  url {
    type: HOMEPAGE
    value: "%s"
  }
  url {
    type: GIT
    value: "%s"
  }
  version: "%s"
  license_type: %s
  last_upgrade_date {
    year: %d
    month: %d
    day: %d
  }
}
`, r.projectMetadataFileConfig.Name, r.projectMetadataFileConfig.Description, r.projectMetadataFileConfig.HomePage, r.projectMetadataFileConfig.GitURL, to.Id, r.projectMetadataFileConfig.LicenseType, d.Year(), d.Month(), d.Day())

		metadataFilePath := filepath.Join(r.workdir, r.projectMetadataFileConfig.FilePath)
		if err := ioutil.WriteFile(metadataFilePath, []byte(metadataContents), os.ModePerm); err != nil {
			return 0, fmt.Errorf("Error when writing to %s: %s", metadataFilePath, err)
		}
		if _, addGifErr := r.childRepo.Git(ctx, "add", metadataFilePath); addGifErr != nil {
			return 0, addGifErr
		}
	}

	// Run the pre-upload steps.
	for _, s := range r.preUploadSteps {
		if err := s(ctx, nil, r.httpClient, r.workdir); err != nil {
			util.LogErr(r.abortMerge(ctx))
			return 0, fmt.Errorf("Failed pre-upload step: %s", err)
		}
	}

	// Create a new repo branch.
	if _, repoBranchErr := exec.RunCwd(ctx, r.childDir, r.repoToolPath, "start", REPO_BRANCH_NAME, "."); repoBranchErr != nil {
		util.LogErr(r.abortMerge(ctx))
		return 0, fmt.Errorf("Failed to create repo branch: %s", repoBranchErr)
	}

	// If the parent branch is not the main branch then:
	// Add all authors of merged changes to the email list. We do not do this
	// for the main branch because developers would get spammed due to multiple
	// rolls a day. Release branch rolls run rarely and developers should be
	// aware that their changes are being rolled there.
	rollEmails := []string{}
	rollEmails = append(rollEmails, emails...)
	if parentBranch != git.DefaultBranch {
		for _, c := range rolling {
			// Extract out the email if it is a Googler.
			if strings.HasSuffix(c.Author, "@google.com") {
				rollEmails = append(rollEmails, c.Author)
			}
		}
		sort.Strings(rollEmails)
	}

	// Commit the change with the above message.
	if _, commitErr := r.childRepo.Git(ctx, "commit", "-m", commitMsg); commitErr != nil {
		util.LogErr(r.abandonRepoBranch(ctx))
		return 0, fmt.Errorf("Nothing to merge; did someone already merge %s..%s?: %s", from, to, commitErr)
	}

	// Bypass the repo upload prompt by setting autoupload config to true.
	// Strip "-review" from the upload URL else autoupload does not work.
	uploadUrl := strings.Replace(r.parentRepoURL, "-review", "", 1)
	if _, configErr := r.childRepo.Git(ctx, "config", fmt.Sprintf("review.%s/.autoupload", uploadUrl), "true"); configErr != nil {
		util.LogErr(r.abandonRepoBranch(ctx))
		return 0, fmt.Errorf("Could not set autoupload config: %s", configErr)
	}

	// Upload the CL to Gerrit.
	uploadArgs := []string{"upload", "--no-verify"}
	if rollEmails != nil && len(rollEmails) > 0 {
		uploadArgs = append(uploadArgs, fmt.Sprintf("--re=%s", strings.Join(rollEmails, ",")))
	}
	uploadCommand := &exec.Command{
		Name: r.repoToolPath,
		Args: uploadArgs,
		Dir:  r.childDir,
		// The below is to bypass the blocking
		// "ATTENTION: You are uploading an unusually high number of commits."
		// prompt which shows up when a merge contains more than 5 commits.
		Stdin: strings.NewReader("yes"),
	}
	if _, uploadErr := exec.RunCommand(ctx, uploadCommand); uploadErr != nil {
		util.LogErr(r.abandonRepoBranch(ctx))
		return 0, fmt.Errorf("Could not upload to Gerrit: %s", uploadErr)
	}

	// Get latest hash to find Gerrit change number with.
	commitHashOutput, revParseErr := r.childRepo.Git(ctx, "rev-parse", "HEAD")
	if revParseErr != nil {
		util.LogErr(r.abandonRepoBranch(ctx))
		return 0, revParseErr
	}
	commitHash := strings.Split(commitHashOutput, "\n")[0]
	// We no longer need the local branch. Abandon the repo.
	util.LogErr(r.abandonRepoBranch(ctx))

	// Get the change number.
	change, err := r.getChangeForHash(commitHash)
	if err != nil {
		util.LogErr(r.abandonRepoBranch(ctx))
		return 0, err
	}
	// Set the topic of the merge change.
	if err := r.setTopic(change.Issue); err != nil {
		return 0, err
	}

	// Set labels.
	labels := r.g.Config().SetCqLabels
	if dryRun {
		labels = r.g.Config().SetDryRunLabels
	}
	labels = gerrit.MergeLabels(labels, r.g.Config().SelfApproveLabels)
	if err = r.g.SetReview(ctx, change, "Roller setting labels to auto-land change.", labels, rollEmails); err != nil {
		// Only throw exception here if parentBranch is the main branch. This is
		// because other branches will not have permissions setup for the
		// bot to run CR+2.
		if parentBranch != git.DefaultBranch {
			sklog.Warningf("Could not set labels on %d: %s", change.Issue, err)
			sklog.Warningf("Not throwing error because branch %q is not %q", parentBranch, git.DefaultBranch)
		} else {
			return 0, err
		}
	}

	// Mark the change as ready for review, if necessary.
	if err := gerrit_common.UnsetWIP(ctx, r.g, change, 0); err != nil {
		return 0, err
	}

	return change.Issue, nil
}

func (r *androidRepoManager) getTipRev(ctx context.Context) (*revision.Revision, error) {
	// "ls-remote" can get stuck indefinitely if GoB is having problems. Call it with a timeout.
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel() // Releases resources if "ls-remote" completes before timeout.
	output, err := r.childRepo.Git(ctxWithTimeout, "ls-remote", UPSTREAM_REMOTE_NAME, fmt.Sprintf("refs/heads/%s", r.childBranch), "-1")
	if err != nil {
		return nil, err
	}
	hash := strings.Split(output, "\t")[0]
	details, err := r.childRepo.Details(ctx, hash)
	if err != nil {
		return nil, err
	}
	return revision.FromLongCommit(r.childRevLinkTmpl, details), nil
}
