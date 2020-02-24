package repo_manager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tar"
	"google.golang.org/api/option"
)

const (
	ANDROID_BP                       = "Android.bp"
	FUCHSIA_SDK_ANDROID_VERSION_FILE = "sdk_id"
	GEN_SDK_BP                       = "gen_sdk_bp.py"
	GEN_SDK_BP_DIR                   = "scripts"
	SDK_DEST_PATH                    = "prebuilts/fuchsia_sdk"

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
	arm            *androidRepoManager
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
	androidConfig := &AndroidRepoManagerConfig{
		CommonRepoManagerConfig: c.CommonRepoManagerConfig,
	}
	androidRM, err := NewAndroidRepoManager(ctx, androidConfig, reg, workdir, g, serverURL, "<unused>", authClient, cr, local)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	arm := androidRM.(*androidRepoManager)
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(authClient))
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	fsrm := &fuchsiaSDKRepoManager{
		gcsClient:         gcsclient.New(storageClient, FUCHSIA_SDK_GS_BUCKET),
		gsBucket:          FUCHSIA_SDK_GS_BUCKET,
		gsLatestPathLinux: FUCHSIA_SDK_GS_LATEST_PATH_LINUX,
		gsLatestPathMac:   FUCHSIA_SDK_GS_LATEST_PATH_MAC,
		gsListPath:        FUCHSIA_SDK_GS_PATH,
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
		fuchsiaSDKRepoManager: fsrm,
		arm:                   arm,
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

// See documentation for noCheckoutRepoManagerUpdateHelperFunc.
func (rm *fuchsiaSDKAndroidRepoManager) updateHelper(ctx context.Context, parentRepo *gitiles.Repo, baseCommit string) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	sklog.Info("Updating Android checkout...")
	if err := rm.arm.updateAndroidCheckout(ctx); err != nil {
		return nil, nil, nil, skerr.Wrap(err)
	}

	sklog.Info("Finding next roll rev...")
	lastRollRev, tipRev, notRolledRevs, err := rm.fuchsiaSDKRepoManager.updateHelper(ctx, parentRepo, baseCommit)
	if err != nil {
		return nil, nil, nil, skerr.Wrap(err)
	}

	if err := rm.parentRepo.UpdateBranch(ctx, rm.parentBranch.String()); err != nil {
		return nil, nil, nil, skerr.Wrap(err)
	}
	return lastRollRev, tipRev, notRolledRevs, nil
}

// See documentation for noCheckoutRepoManagerCreateRollHelperFunc.
func (rm *fuchsiaSDKAndroidRepoManager) createRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, serverURL, cqExtraTrybots string, emails []string, baseCommit string) (string, map[string]string, error) {
	// Sync the parentRepo to baseCommit.
	if _, err := rm.parentRepo.Git(ctx, "reset", "--hard", baseCommit); err != nil {
		return "", nil, skerr.Wrap(err)
	}
	if err := rm.genSdkBpRepo.UpdateBranch(ctx, rm.genSdkBpBranch); err != nil {
		return "", nil, skerr.Wrap(err)
	}
	if _, err := rm.genSdkBpRepo.Git(ctx, "checkout", fmt.Sprintf("origin/%s", rm.genSdkBpBranch)); err != nil {
		return "", nil, skerr.Wrap(err)
	}
	genSdkBpRepoHash, err := rm.genSdkBpRepo.RevParse(ctx, "HEAD")
	if err != nil {
		return "", nil, skerr.Wrap(err)
	}
	sklog.Info("Reading old file contents...")
	oldContents, err := fileutil.ReadAllFilesRecursive(rm.parentRepo.Dir(), []string{".git"})
	if err != nil {
		return "", nil, skerr.Wrap(err)
	}

	// Instead of simply rolling the version hash into a file, download and
	// unzip the SDK, and commit its contents.
	sdkDestPath := path.Join(rm.arm.workdir, SDK_DEST_PATH)
	if err := os.RemoveAll(sdkDestPath); err != nil {
		sklog.Warningf("Failed to remove SDK dest path %s: %s", sdkDestPath, err)
	}
	if err := os.MkdirAll(sdkDestPath, os.ModePerm); err != nil {
		return "", nil, skerr.Wrap(err)
	}
	sdkGsPath := rm.gsListPath + "/linux-amd64/" + to.Id
	sklog.Infof("Downloading SDK from %s...", sdkGsPath)
	newContents := map[string][]byte{}
	r, err := rm.gcsClient.FileReader(ctx, sdkGsPath)
	if err != nil {
		return "", nil, skerr.Wrap(err)
	}
	if err := tar.ReadGzipArchive(r, func(filename string, r io.Reader) error {
		b, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}
		newContents[filename] = b
		filePath := path.Join(sdkDestPath, filename)
		dir := path.Dir(filePath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		return ioutil.WriteFile(filePath, b, os.ModePerm)
	}); err != nil {
		return "", nil, fmt.Errorf("Failed to read archive: %s", err)
	}

	// Run the gen_sdk_bp.py script.
	genSdkBp := path.Join(rm.genSdkBpRepo.Dir(), GEN_SDK_BP_DIR, GEN_SDK_BP)
	sklog.Infof("Running %s at %s", genSdkBp, genSdkBpRepoHash)
	env := []string{fmt.Sprintf("ANDROID_BUILD_TOP=%s", rm.arm.workdir)}
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:        rm.genSdkBpRepo.Dir(),
		Name:       "python",
		Args:       []string{genSdkBp},
		Env:        env,
		InheritEnv: true,
		LogStdout:  true,
		LogStderr:  true,
	}); err != nil {
		return "", nil, skerr.Wrap(err)
	}
	src := path.Join(rm.arm.workdir, rm.childPath, ANDROID_BP)
	b, err := ioutil.ReadFile(src)
	if err != nil {
		return "", nil, skerr.Wrap(err)
	}
	newContents[ANDROID_BP] = b

	// Determine the contents of the next roll.
	nextRollContents := make(map[string]string, len(newContents))
	for f, contents := range newContents {
		if !bytes.Equal(oldContents[f], contents) {
			nextRollContents[f] = string(contents)
		}
	}
	for f := range oldContents {
		if _, ok := newContents[f]; !ok {
			nextRollContents[f] = ""
		}
	}

	// Lastly, include the SDK version hash file.
	nextRollContents[rm.versionFileLinux] = to.Id
	sklog.Infof("Next roll modifies %d files.", len(nextRollContents))

	// Create the commit message.
	msg, err := rm.buildCommitMsg(&CommitMsgVars{
		CqExtraTrybots: cqExtraTrybots,
		Reviewers:      emails,
		RollingFrom:    from,
		RollingTo:      to,
		ServerURL:      rm.serverURL,
	})
	if err != nil {
		return "", nil, skerr.Wrap(err)
	}

	return msg, nextRollContents, nil
}
