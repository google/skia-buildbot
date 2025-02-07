package repo_manager

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/android_skia_checkout"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/oauth2/google"
)

const (
	// androidUpstreamRemoteName is the name of the remote used for Android
	// rollers.
	androidUpstreamRemoteName = "remote"
	// androidRepoBranchName is the name of the branch used for Android rollers.
	androidRepoBranchName = "merge"

	// Android does not allow self+2. As a workaround, we use a second account
	// to approve our CLs.
	autoApproverKeyProject = "skia-infra-public"
)

var (
	androidDeleteMergeConflictFiles = []string{
		android_skia_checkout.SkUserConfigRelPath,
		// Android does not allow remote submodules (b/189557997).
		".gitmodules",
		// Temporary workaround for swiftshader->android roller till
		// b/198431779 is resolved.
		"third_party/angle/angle",
	}
)

// androidRepoManager is a struct used by Android AutoRoller for managing
// checkouts.
type androidRepoManager struct {
	androidRemoteName         string
	autoApproverGerrit        gerrit.GerritInterface
	childRepoURL              string
	defaultBugProject         string
	parentRepoURL             string
	repoToolPath              string
	includeAuthorsAsReviewers bool

	projectMetadataFileConfig *config.AndroidRepoManagerConfig_ProjectMetadataFileConfig

	childBranch      *config_vars.Template
	childDir         string
	childPath        string
	childRepo        git.Checkout
	childRevLinkTmpl string
	g                gerrit.GerritInterface
	httpClient       *http.Client
	parentBranch     *config_vars.Template
	preUploadSteps   []parent.PreUploadStep
	repoMtx          sync.RWMutex
	workdir          string
}

// NewAndroidRepoManager returns an androidRepoManager instance.
func NewAndroidRepoManager(ctx context.Context, c *config.AndroidRepoManagerConfig, reg *config_vars.Registry, workdir string, serverURL, serviceAccount string, client *http.Client, cr codereview.CodeReview, isInternal, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	user, err := user.Current()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	repoToolDir := path.Join(user.HomeDir, "bin")
	repoToolPath := path.Join(repoToolDir, "repo")
	if _, err := os.Stat(repoToolDir); err != nil {
		if err := os.MkdirAll(repoToolDir, 0755); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	if _, err := os.Stat(repoToolPath); err != nil {
		// Download the repo tool.
		if _, err := exec.RunCwd(ctx, repoToolDir, "wget", "https://storage.googleapis.com/git-repo-downloads/repo", "-O", repoToolPath); err != nil {
			return nil, skerr.Wrap(err)
		}
		// Make the repo tool executable.
		if _, err := exec.RunCwd(ctx, repoToolDir, "chmod", "a+x", repoToolPath); err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	childDir := path.Join(workdir, c.ChildPath)
	if c.ChildSubdir != "" {
		childDir = path.Join(workdir, c.ChildSubdir, c.ChildPath)
	}
	childRepo := git.CheckoutDir(childDir)

	if _, err := os.Stat(workdir); err == nil {
		if err := git.DeleteLockFiles(ctx, workdir); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	preUploadSteps, err := parent.GetPreUploadSteps(c.PreUploadSteps, c.PreUploadCommands)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childBranch, err := config_vars.NewTemplate(c.ChildBranch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := reg.Register(childBranch); err != nil {
		return nil, skerr.Wrap(err)
	}
	parentBranch, err := config_vars.NewTemplate(c.ParentBranch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := reg.Register(parentBranch); err != nil {
		return nil, skerr.Wrap(err)
	}

	androidRemoteName := "aosp"
	if isInternal {
		androidRemoteName = "goog"
	}
	g, ok := cr.Client().(gerrit.GerritInterface)
	if !ok {
		return nil, skerr.Fmt("AndroidRepoManager must use Gerrit for code review.")
	}

	var autoApproverGerrit gerrit.GerritInterface
	if !local && c.AutoApproverSecret != "" {
		secretClient, err := secret.NewClient(ctx)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		autoApproverKey, err := secretClient.Get(ctx, autoApproverKeyProject, c.AutoApproverSecret, secret.VersionLatest)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to retrieve secret %s", c.AutoApproverSecret)
		} else {
			autoApproverCreds, err := google.CredentialsFromJSON(ctx, []byte(autoApproverKey), gerrit.AuthScope)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			autoApproverClient := httputils.DefaultClientConfig().WithTokenSource(autoApproverCreds.TokenSource).With2xxOnly().WithRetry4XX().Client()
			autoApproverGerritConfig := cr.Client().(*gerrit.Gerrit).Config()
			autoApproverGerritURL := cr.Client().(*gerrit.Gerrit).Url(0)
			autoApproverGerrit, err = gerrit.NewGerritWithConfig(autoApproverGerritConfig, autoApproverGerritURL, autoApproverClient)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
		}
	}

	r := &androidRepoManager{
		androidRemoteName:         androidRemoteName,
		autoApproverGerrit:        autoApproverGerrit,
		parentRepoURL:             g.GetRepoUrl(),
		repoToolPath:              repoToolPath,
		projectMetadataFileConfig: c.Metadata,
		childRepoURL:              c.ChildRepoUrl,
		defaultBugProject:         c.DefaultBugProject,
		includeAuthorsAsReviewers: c.IncludeAuthorsAsReviewers,

		childBranch:      childBranch,
		childDir:         childDir,
		childPath:        c.ChildPath,
		childRepo:        childRepo,
		childRevLinkTmpl: c.ChildRevLinkTmpl,
		g:                g,
		httpClient:       client,
		parentBranch:     parentBranch,
		preUploadSteps:   preUploadSteps,
		workdir:          workdir,
	}
	return r, nil
}

// GetRevision implements RepoManager.
func (r *androidRepoManager) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	r.repoMtx.RLock()
	defer r.repoMtx.RUnlock()
	details, err := r.childRepo.Details(ctx, id)
	if err != nil {
		return nil, err
	}
	return revision.FromLongCommit(r.childRevLinkTmpl, r.defaultBugProject, details), nil
}

// Helper function for updating the Android checkout.
func (r *androidRepoManager) updateAndroidCheckout(ctx context.Context) error {
	// Create the working directory if needed.
	if _, err := os.Stat(r.workdir); err != nil {
		if err := os.MkdirAll(r.workdir, 0755); err != nil {
			return skerr.Wrapf(err, "failed to create workdir %s", r.workdir)
		}
	}

	// Create the temp directory if needed.
	tmp := os.Getenv("TMPDIR")
	if tmp != "" {
		if _, err := os.Stat(tmp); err != nil {
			if err := os.MkdirAll(tmp, 0755); err != nil {
				return skerr.Wrapf(err, "failed to create TMPDIR %s", tmp)
			}
		}
	}

	// Run repo init and sync commands.
	initCmd := []string{"python3", r.repoToolPath, "init", "-u", fmt.Sprintf("%s/a/platform/manifest", r.parentRepoURL), "-g", "all,-notdefault,-darwin", "-b", r.parentBranch.String()}
	if _, err := exec.RunCwd(ctx, r.workdir, initCmd...); err != nil {
		sklog.Warningf("repo init error: %s", err)
		// Try deleting .repo in the workdir and re-initing (skbug.com/13867).
		repoDirPath := path.Join(r.workdir, ".repo")
		if err := os.RemoveAll(repoDirPath); err != nil {
			return skerr.Wrapf(err, "Could not delete %s before attempting a reinit", repoDirPath)
		}
		if _, err := exec.RunCwd(ctx, r.workdir, initCmd...); err != nil {
			return err
		}
	}

	// Sync only the child path and the repohooks directory (needed to upload changes).
	const repoHooksDir = "tools/repohooks"
	syncCmd := []string{"python3", r.repoToolPath, "sync", "--force-sync", r.childPath, repoHooksDir, "-j32"}
	if _, err := exec.RunCwd(ctx, r.workdir, syncCmd...); err != nil {
		sklog.Warningf("repo sync error: %s", err)

		// Try deleting childDir and re-syncing (skbug.com/12955).
		if err := os.RemoveAll(r.childDir); err != nil {
			return skerr.Wrapf(err, "Could not delete %s before attempting a resync", r.childDir)
		}
		sklog.Info("Retrying sync after deleting %s", r.childDir)
		if _, err := exec.RunCwd(ctx, r.workdir, syncCmd...); err != nil {
			sklog.Warningf("repo sync error after deleting %s: %s", r.childDir, err)

			// Try deleting tools/repohooks and re-syncing (skbug.com/12955).
			if err := os.RemoveAll(path.Join(r.workdir, repoHooksDir)); err != nil {
				return skerr.Wrapf(err, "Could not delete %s before attempting a resync", repoHooksDir)
			}
			if _, err := exec.RunCwd(ctx, r.workdir, syncCmd...); err != nil {
				sklog.Warningf("repo sync error after deleting %s: %s", repoHooksDir, err)

				// Try deleting .repo in the workdir and re-initing and re-syncing (skbug.com/12146).
				repoDirPath := path.Join(r.workdir, ".repo")
				if err := os.RemoveAll(repoDirPath); err != nil {
					return skerr.Wrapf(err, "Could not delete %s before attempting a resync", repoDirPath)
				}
				if _, err := exec.RunCwd(ctx, r.workdir, initCmd...); err != nil {
					return err
				}
				sklog.Info("Retrying sync after deleting %s", repoDirPath)
				if _, err := exec.RunCwd(ctx, r.workdir, syncCmd...); err != nil {
					return err
				}
			}
		}
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
	if !strings.Contains(remoteOutput, androidUpstreamRemoteName) {
		if _, err := r.childRepo.Git(ctx, "remote", "add", androidUpstreamRemoteName, r.childRepoURL); err != nil {
			return err
		}
	}

	// Update the remote to make sure that all new branches are available.
	if _, err := r.childRepo.Git(ctx, "remote", "update", androidUpstreamRemoteName, "--prune"); err != nil {
		return err
	}
	return nil
}

// Update implements RepoManager.
func (r *androidRepoManager) Update(ctx context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	// Sync the projects.
	r.repoMtx.Lock()
	defer r.repoMtx.Unlock()
	if err := r.updateAndroidCheckout(ctx); err != nil {
		return nil, nil, nil, skerr.Wrapf(err, "failed to update Android checkout")
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
	notRolledRevs, err := r.LogRevisions(ctx, lastRollRev, tipRev)
	if err != nil {
		return nil, nil, nil, err
	}

	return lastRollRev, tipRev, notRolledRevs, nil
}

// LogRevisions implements RepoManager.
func (r *androidRepoManager) LogRevisions(ctx context.Context, from, to *revision.Revision) ([]*revision.Revision, error) {
	if from.Id == to.Id {
		return []*revision.Revision{}, nil
	}
	commits, err := r.childRepo.RevList(ctx, "--first-parent", git.LogFromTo(from.Id, to.Id))
	if err != nil {
		return nil, err
	}
	revs := make([]*vcsinfo.LongCommit, 0, len(commits))
	for _, c := range commits {
		detail, err := r.childRepo.Details(ctx, c)
		if err != nil {
			return nil, err
		}
		revs = append(revs, detail)
	}
	return revision.FromLongCommits(r.childRevLinkTmpl, r.defaultBugProject, revs), nil
}

// getLastRollRev returns the last-completed DEPS roll Revision.
func (r *androidRepoManager) getLastRollRev(ctx context.Context) (*revision.Revision, error) {
	output, err := r.childRepo.Git(ctx, "merge-base", fmt.Sprintf("refs/remotes/remote/%s", r.childBranch), fmt.Sprintf("refs/remotes/%s/%s", r.androidRemoteName, r.parentBranch))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	details, err := r.childRepo.Details(ctx, strings.TrimRight(output, "\n"))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return revision.FromLongCommit(r.childRevLinkTmpl, r.defaultBugProject, details), nil
}

// abortMerge aborts the current merge in the child repo.
func (r *androidRepoManager) abortMerge(ctx context.Context) error {
	_, err := r.childRepo.Git(ctx, "merge", "--abort")
	return err
}

// abandonRepoBranchAndCleanup abandons the repo branch and cleans up the local
// checkout to make sure there are no leftover untracked files/directories.
func (r *androidRepoManager) abandonRepoBranchAndCleanup(ctx context.Context) error {
	if _, err := exec.RunCwd(ctx, r.childRepo.Dir(), "python3", r.repoToolPath, "abandon", androidRepoBranchName); err != nil {
		sklog.Errorf("Failed to abandon merge; ignoring: %s", err)
	}
	if _, err := r.childRepo.Git(ctx, "reset", "--hard", "HEAD"); err != nil {
		return err
	}
	if _, err := r.childRepo.Git(ctx, "clean", "-d", "-f", "-f"); err != nil {
		return err
	}
	return nil
}

// getChangeNumForHash returns the corresponding change number for the provided commit hash by querying Gerrit's search API.
func (r *androidRepoManager) getChangeForHash(hash string) (*gerrit.ChangeInfo, error) {
	issues, err := r.g.Search(context.TODO(), 1, false, gerrit.SearchCommit(hash))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if len(issues) == 0 {
		return nil, skerr.Fmt("Could not find any issues that match the commit hash %s", hash)
	}
	return r.g.GetIssueProperties(context.TODO(), issues[0].Issue)
}

// See documentation for RepoManager interface.
func (r *androidRepoManager) CreateNewRoll(ctx context.Context, from *revision.Revision, to *revision.Revision, rolling []*revision.Revision, emails []string, dryRun bool, commitMsg string) (int64, error) {
	r.repoMtx.Lock()
	defer r.repoMtx.Unlock()

	// Update the upstream remote.
	if _, err := r.childRepo.Git(ctx, "fetch", androidUpstreamRemoteName); err != nil {
		return 0, err
	}

	// Create the roll CL.

	// Start the merge.
	mergeTarget := to.Id
	squash := false
	if strings.HasPrefix(to.Id, gerrit.ChangeRefPrefix) {
		if err := r.childRepo.FetchRefFromRepo(ctx, r.childRepoURL, to.Id); err != nil {
			return 0, skerr.Wrapf(err, "Failed to fetch ref in %s: %s", r.childRepo.Dir(), err)
		}
		mergeTarget = "FETCH_HEAD"
		// To avoid having Android automatically upload the unsubmitted changes of
		// the canary user, squash the merge commit.
		// For regular rolls not squashing the merge is preferable because
		// individual changes have already been replicated over and that means
		// that they will not be uploaded again in Android.
		squash = true
	}
	mergeCmds := []string{"merge", mergeTarget, "--no-commit"}
	if squash {
		mergeCmds = append(mergeCmds, "--squash")
	}
	_, mergeErr := r.childRepo.Git(ctx, mergeCmds...)
	if mergeErr != nil {
		sklog.Errorf("git merge failed with:\n%s", mergeErr)
	}

	// Android does not allow remote dependencies to have submodule directories (b/189557997)
	// .gitmodules will be removed as part of androidDeleteMergeConflictFiles, so delete the directories here.
	modOutput, modErr := exec.RunCwd(ctx, r.childDir, "bash", "-c", "git ls-files -s | grep ^160000 | awk '{ print $4; }' | awk '{ system(\"git rm -r \"$1) }'")
	sklog.Infof("Output of submodule removal cmd: %s", modOutput)
	util.LogErr(modErr)

	if mergeErr != nil {
		// Check to see if this was a merge conflict with ignoreMergeConflictFiles and deleteMergeConflictFiles.
		conflictsOutput, conflictsErr := r.childRepo.Git(ctx, "diff", "--name-only", "--diff-filter=U")
		if conflictsErr != nil || (modOutput == "" && conflictsOutput == "") {
			util.LogErr(conflictsErr)
			return 0, skerr.Wrapf(mergeErr, "failed to roll to %s. Needs human investigation: %s", to, mergeErr)
		}
		for _, conflict := range strings.Split(conflictsOutput, "\n") {
			if conflict == "" {
				continue
			}
			ignoreConflict := false
			for _, del := range androidDeleteMergeConflictFiles {
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
				return 0, skerr.Wrapf(mergeErr, "failed to roll to %s. Conflicts in %s: %s", to, conflictsOutput, mergeErr)
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
`, r.projectMetadataFileConfig.Name, r.projectMetadataFileConfig.Description, r.projectMetadataFileConfig.HomePage, r.projectMetadataFileConfig.GitUrl, to.Id, r.projectMetadataFileConfig.LicenseType, d.Year(), d.Month(), d.Day())

		metadataFilePath := filepath.Join(r.workdir, r.projectMetadataFileConfig.FilePath)
		if err := os.WriteFile(metadataFilePath, []byte(metadataContents), os.ModePerm); err != nil {
			return 0, skerr.Wrapf(err, "Error when writing to %s: %s", metadataFilePath, err)
		}
		if _, addGifErr := r.childRepo.Git(ctx, "add", metadataFilePath); addGifErr != nil {
			return 0, addGifErr
		}
	}

	// Run the pre-upload steps.
	for _, s := range r.preUploadSteps {
		if err := s(ctx, nil, r.httpClient, r.workdir, from, to); err != nil {
			util.LogErr(r.abortMerge(ctx))
			return 0, skerr.Wrapf(err, "Failed pre-upload step: %s", err)
		}
	}

	// The pre-upload step may reintroduce submodule directories, remove them
	// to compensate the effect.
	modOutput, modErr = exec.RunCwd(ctx, r.childDir, "bash", "-c", "git ls-files -s | grep ^160000 | awk '{ print $4; }' | awk '{ system(\"git rm -f --cached \"$1) }'")
	sklog.Infof("Output of submodule removal cmd (after preUploadSteps): %s", modOutput)
	util.LogErr(modErr)

	// Create a new repo branch.
	if _, repoBranchErr := exec.RunCwd(ctx, r.childDir, "python3", r.repoToolPath, "start", androidRepoBranchName, "."); repoBranchErr != nil {
		util.LogErr(r.abortMerge(ctx))
		return 0, skerr.Wrapf(repoBranchErr, "failed to create repo branch")
	}

	rollEmails := []string{}
	rollEmails = append(rollEmails, emails...)
	if r.includeAuthorsAsReviewers {
		// Add all authors of merged changes to the email list. We do this only
		// for some rollers because developers would get spammed due to multiple
		// rolls a day. Release branch rolls run rarely and developers should be
		// aware that their changes are being rolled there.
		for _, c := range rolling {
			// Extract out the email if it is a Googler.
			if strings.HasSuffix(c.Author, "@google.com") {
				rollEmails = append(rollEmails, c.Author)
			}
		}
		sort.Strings(rollEmails)
	}

	// Commit the change with the above message.
	if _, commitErr := r.childRepo.Git(ctx, "commit", "-a", "-m", commitMsg); commitErr != nil {
		util.LogErr(r.abandonRepoBranchAndCleanup(ctx))
		return 0, skerr.Wrapf(commitErr, "nothing to merge; did someone already merge %s..%s?", from, to)
	}

	// Bypass the repo upload prompt by setting autoupload config to true.
	// Strip "-review" from the upload URL else autoupload does not work.
	uploadUrl := strings.Replace(r.parentRepoURL, "-review", "", 1)
	if _, configErr := r.childRepo.Git(ctx, "config", fmt.Sprintf("review.%s/.autoupload", uploadUrl), "true"); configErr != nil {
		util.LogErr(r.abandonRepoBranchAndCleanup(ctx))
		return 0, skerr.Wrapf(configErr, "could not set autoupload config")
	}

	// Check for untracked files and fail if we find any.
	output, err := r.childRepo.Git(ctx, "ls-files", "--exclude-standard", "--others")
	if err != nil {
		util.LogErr(r.abandonRepoBranchAndCleanup(ctx))
		return 0, skerr.Wrapf(err, "failed to check for untracked files")
	}
	output = strings.TrimSpace(output)
	if len(output) > 0 {
		untrackedFiles := strings.Split(output, "\n")
		msg := "found untracked files:"
		for _, f := range untrackedFiles {
			msg += fmt.Sprintf("\n- %s", f)
		}
		util.LogErr(r.abandonRepoBranchAndCleanup(ctx))
		return 0, skerr.Fmt(msg)
	}

	// Upload the CL to Gerrit.
	uploadArgs := []string{r.repoToolPath, "upload", "--no-verify", "--yes",
		"-o", "banned-words~skip"}
	if rollEmails != nil && len(rollEmails) > 0 {
		uploadArgs = append(uploadArgs, fmt.Sprintf("--re=%s", strings.Join(rollEmails, ",")))
	}
	uploadCommand := &exec.Command{
		Name: "python3",
		Args: uploadArgs,
		Dir:  r.childDir,
		// The below is to bypass the blocking
		// "ATTENTION: You are uploading an unusually high number of commits."
		// prompt which shows up when a merge contains more than 5 commits.
		// TODO(borenet): Is this necessary with "--yes"?
		Stdin: strings.NewReader("yes"),
	}
	if uploadOutput, uploadErr := exec.RunCommand(ctx, uploadCommand); uploadErr != nil {
		util.LogErr(r.abandonRepoBranchAndCleanup(ctx))
		return 0, skerr.Wrapf(uploadErr, "could not upload to Gerrit")
	} else {
		sklog.Info(uploadOutput)
	}

	// Get latest hash to find Gerrit change number with.
	commitHashOutput, revParseErr := r.childRepo.Git(ctx, "rev-parse", "HEAD")
	if revParseErr != nil {
		util.LogErr(r.abandonRepoBranchAndCleanup(ctx))
		return 0, revParseErr
	}
	commitHash := strings.Split(commitHashOutput, "\n")[0]
	// We no longer need the local branch. Abandon the repo.
	util.LogErr(r.abandonRepoBranchAndCleanup(ctx))

	// Get the change number.
	change, err := r.getChangeForHash(commitHash)
	if err != nil {
		util.LogErr(r.abandonRepoBranchAndCleanup(ctx))
		return 0, err
	}
	// Set the topic of the merge change. By default use the name of the child
	// repo and the change number. Example: skia_merge_1234
	topicName := fmt.Sprintf("%s_merge_%d", path.Base(r.childDir), change.Issue)
	if to.ExternalChangeId != "" {
		topicName = to.ExternalChangeId
	}
	if err := r.g.SetTopic(ctx, topicName, change.Issue); err != nil {
		return 0, err
	}

	// Set labels.
	labels := gerrit.MergeLabels(r.g.Config().SelfApproveLabels, r.g.Config().SetCqLabels)
	if dryRun {
		labels = r.g.Config().SetDryRunLabels
	}
	if err = r.g.SetReview(ctx, change, "Roller setting labels to auto-land change.", labels, rollEmails, "", nil, "", 0, nil); err != nil {
		return 0, err
	}

	// Mark the change as ready for review, if necessary.
	if err := gerrit_common.UnsetWIP(ctx, r.g, change, 0); err != nil {
		return 0, err
	}

	// Use the second account to auto-approve the CL from the first account.
	if r.autoApproverGerrit != nil && !dryRun {
		if err := r.autoApproverGerrit.SetReview(ctx, change, "Auto-approving AutoRoll CL", r.g.Config().SelfApproveLabels, nil, "", nil, "", 0, nil); err != nil {
			return 0, skerr.Wrap(err)
		}
	}
	return change.Issue, nil
}

func (r *androidRepoManager) getTipRev(ctx context.Context) (*revision.Revision, error) {
	// "ls-remote" can get stuck indefinitely if GoB is having problems. Call it with a timeout.
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel() // Releases resources if "ls-remote" completes before timeout.
	output, err := r.childRepo.Git(ctxWithTimeout, "ls-remote", androidUpstreamRemoteName, fmt.Sprintf("refs/heads/%s", r.childBranch), "-1")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	hash := strings.Split(output, "\t")[0]
	details, err := r.childRepo.Details(ctx, hash)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return revision.FromLongCommit(r.childRevLinkTmpl, r.defaultBugProject, details), nil
}
