package repo_manager

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/option"
)

const (
	FUCHSIA_SDK_ANDROID_VERSION_FILE    = "sdk_id"
	FUCHSIA_SDK_ANDROID_GEN_SCRIPT      = "scripts/update_fuchsia_sdk.py"
	TMPL_COMMIT_MSG_FUCHSIA_SDK_ANDROID = TMPL_COMMIT_MSG_FUCHSIA_SDK + "Exempt-From-Owner-Approval: The autoroll bot does not require owner approval."
)

type FuchsiaSDKAndroidRepoManagerConfig struct {
	FuchsiaSDKRepoManagerConfig
	GenSdkBpRepo   string `json:"genSdkBpRepo"`
	GenSdkBpBranch string `json:"genSdkBpBranch"`
}

// See documentation for RepoManagerConfig interface.
func (c *FuchsiaSDKAndroidRepoManagerConfig) NoCheckout() bool {
	return false
}

// See documentation for util.Validator interface.
func (c *FuchsiaSDKAndroidRepoManagerConfig) Validate() error {
	if err := c.FuchsiaSDKRepoManagerConfig.Validate(); err != nil {
		return err
	}
	if c.GenSdkBpRepo == "" {
		return errors.New("GenSdkBpRepo is required.")
	}
	if c.GenSdkBpBranch == "" {
		return errors.New("GenSdkBpBranch is required.")
	}
	return nil
}

// fuchsiaSDKAndroidRepoManager is a RepoManager which rolls the Fuchsia SDK
// into Android. Unlike the fuchsiaSDKRepoManager, it actually unzips the
// contents of the SDK and checks them into the target repo. Additionally, it
// generates an Android.bp file.
type fuchsiaSDKAndroidRepoManager struct {
	*fuchsiaSDKRepoManager
	androidTop     string
	genSdkBpRepo   *git.Checkout
	genSdkBpBranch string
	parentRepo     *git.Checkout
}

// Return a fuchsiaSDKAndroidRepoManager instance.
func NewFuchsiaSDKAndroidRepoManager(ctx context.Context, c *FuchsiaSDKAndroidRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, serverURL string, authClient *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	// We're not using the constructor for fuchsiaSDKRepoManager because we
	// need the NoCheckoutRepoManager to use the methods of this
	// implementation.
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(authClient))
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	fsrm := &fuchsiaSDKRepoManager{
		gcsClient:         gcsclient.New(storageClient, FUCHSIA_SDK_GS_BUCKET),
		gsBucket:          FUCHSIA_SDK_GS_BUCKET,
		gsLatestPathLinux: FUCHSIA_SDK_GS_LATEST_PATH_LINUX,
		gsLatestPathMac:   FUCHSIA_SDK_GS_LATEST_PATH_MAC,
		storageClient:     storageClient,
		versionFileLinux:  FUCHSIA_SDK_ANDROID_VERSION_FILE,
		versionFileMac:    "", // Ignored by this RepoManager.
	}
	parentRepo, err := git.NewCheckout(ctx, c.ParentRepo, workdir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	genSdkBpRepo, err := git.NewCheckout(ctx, c.GenSdkBpRepo, workdir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if c.CommitMsgTmpl == "" {
		c.CommitMsgTmpl = TMPL_COMMIT_MSG_FUCHSIA_SDK_ANDROID
	}
	rv := &fuchsiaSDKAndroidRepoManager{
		androidTop:            workdir,
		fuchsiaSDKRepoManager: fsrm,
		parentRepo:            parentRepo,
		genSdkBpRepo:          genSdkBpRepo,
		genSdkBpBranch:        c.GenSdkBpBranch,
	}
	ncrm, err := newNoCheckoutRepoManager(ctx, c.NoCheckoutRepoManagerConfig, reg, workdir, g, serverURL, authClient, cr, rv.createRoll, rv.updateHelper, local)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv.noCheckoutRepoManager = ncrm
	return rv, nil
}

// See documentation for RepoManager interface.
func (rm *fuchsiaSDKAndroidRepoManager) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	rm.baseCommitMtx.RLock()
	baseCommit := rm.baseCommit
	rm.baseCommitMtx.RUnlock()

	// We defer to the FuchsiaSDKRepoManager for Update(), so we need to
	// sync the local checkouts here.
	if err := rm.parentRepo.UpdateBranch(ctx, rm.parentBranch.String()); err != nil {
		return 0, skerr.Wrap(err)
	}
	if _, err := rm.parentRepo.Git(ctx, "reset", "--hard", baseCommit); err != nil {
		return 0, skerr.Wrap(err)
	}
	if err := rm.genSdkBpRepo.UpdateBranch(ctx, rm.genSdkBpBranch); err != nil {
		return 0, skerr.Wrap(err)
	}
	genSdkBpRepoHash, err := rm.genSdkBpRepo.RevParse(ctx, "HEAD")
	if err != nil {
		return 0, skerr.Wrap(err)
	}

	// Install the Gerrit Change-Id hook.
	out, err := rm.parentRepo.Git(ctx, "rev-parse", "--git-dir")
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	hookFile := filepath.Join(rm.parentRepo.Dir(), strings.TrimSpace(out), "hooks", "commit-msg")
	if _, err := os.Stat(hookFile); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(hookFile), os.ModePerm); err != nil {
			return 0, skerr.Wrap(err)
		}
		if err := rm.g.DownloadCommitMsgHook(ctx, hookFile); err != nil {
			return 0, skerr.Wrap(err)
		}
	} else if err != nil {
		return 0, skerr.Wrap(err)
	}

	// Log "git status" before updating the SDK.
	st, err := rm.parentRepo.Git(ctx, "status")
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	sklog.Info(st)

	// Instead of simply rolling the version hash into a file, download and
	// unzip the SDK by running the update_fuchsia_sdk script, and commit
	// its contents.
	sklog.Infof("Running %s at %s", FUCHSIA_SDK_ANDROID_GEN_SCRIPT, genSdkBpRepoHash)
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:        rm.genSdkBpRepo.Dir(),
		Name:       "python",
		Env:        []string{fmt.Sprintf("ANDROID_BUILD_TOP=%s", rm.androidTop)},
		Args:       []string{FUCHSIA_SDK_ANDROID_GEN_SCRIPT, "--sdk_path", rm.parentRepo.Dir()},
		InheritEnv: true,
		LogStdout:  true,
		LogStderr:  true,
		Verbose:    exec.Info,
	}); err != nil {
		return 0, skerr.Wrap(err)
	}

	// Log "git status" after updating the SDK.
	st, err = rm.parentRepo.Git(ctx, "status")
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	sklog.Info(st)

	// Create the commit message.
	msg, err := rm.buildCommitMsg(&parent.CommitMsgVars{
		CqExtraTrybots: cqExtraTrybots,
		Reviewers:      emails,
		RollingFrom:    from,
		RollingTo:      to,
		ServerURL:      rm.serverURL,
	})
	if err != nil {
		return 0, skerr.Wrap(err)
	}

	// Commit. Use -A because we're just taking all of the changes produced
	// by FUCHSIA_SDK_ANDROID_GEN_SCRIPT.
	if _, err := rm.parentRepo.Git(ctx, "add", "-A"); err != nil {
		return 0, skerr.Wrap(err)
	}
	if _, err := rm.parentRepo.Git(ctx, "commit", "-m", msg); err != nil {
		return 0, skerr.Wrap(err)
	}

	// Find the change ID in the commit message.
	out, err = rm.parentRepo.Git(ctx, "log", "-n1")
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	changeId, err := gerrit.ParseChangeId(out)
	if err != nil {
		return 0, skerr.Wrap(err)
	}

	// Upload CL.
	if _, err := rm.parentRepo.Git(ctx, "push", "origin", fmt.Sprintf("HEAD:refs/for/%s", rm.parentBranch)); err != nil {
		return 0, skerr.Wrap(err)
	}
	ci, err := rm.g.GetChange(ctx, changeId)
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	if err := gerrit_common.SetChangeLabels(ctx, rm.g, ci, emails, dryRun); err != nil {
		return 0, skerr.Wrap(err)
	}

	return ci.Issue, nil
}
