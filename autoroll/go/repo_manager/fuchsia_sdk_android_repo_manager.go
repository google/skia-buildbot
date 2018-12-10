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
	"strings"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

const (
	ANDROID_BP                       = "Android.bp"
	FUCHSIA_SDK_ANDROID_VERSION_FILE = "hash"
	FUCHSIA_UTIL                     = "fuchsia_util.py"
	GEN_SDK_BP                       = "gen_sdk_bp.py"
	GEN_SDK_BP_DIR                   = "scripts"
)

type FuchsiaSDKAndroidRepoManagerConfig struct {
	FuchsiaSDKRepoManagerConfig
	GenSdkBpRepo string `json:"genSdkBpRepo"`
}

func (c *FuchsiaSDKAndroidRepoManagerConfig) Validate() error {
	if err := c.FuchsiaSDKRepoManagerConfig.Validate(); err != nil {
		return err
	}
	if c.GenSdkBpRepo == "" {
		return errors.New("GenSdkBpRepo is required.")
	}
	return nil
}

// fuchsiaSDKAndroidRepoManager is a RepoManager which rolls the Fuchsia SDK
// into Android. Unlike the fuchsiaSDKRepoManager, it actually unzips the
// contents of the SDK and checks them into the target repo. Additionally, it
// generates an Android.bp file.
type fuchsiaSDKAndroidRepoManager struct {
	*fuchsiaSDKRepoManager
	arm          *androidRepoManager
	genSdkBpRepo *git.Checkout
	parentRepo   *git.Checkout
}

// Return a fuchsiaSDKAndroidRepoManager instance.
func NewFuchsiaSDKAndroidRepoManager(ctx context.Context, c *FuchsiaSDKAndroidRepoManagerConfig, workdir string, g gerrit.GerritInterface, serverURL, gitcookiesPath string, authClient *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	// We're not using the constructor for fuchsiaSDKRepoManager because we
	// need the NoCheckoutRepoManager to use the methods of this
	// implementation.
	if err := c.Validate(); err != nil {
		return nil, err
	}
	androidConfig := &AndroidRepoManagerConfig{
		CommonRepoManagerConfig: c.CommonRepoManagerConfig,
	}
	androidConfig.ParentBranch = "master"
	androidRM, err := NewAndroidRepoManager(ctx, androidConfig, workdir, g, serverURL, "<unused>", authClient, cr, local)
	if err != nil {
		return nil, err
	}
	arm := androidRM.(*androidRepoManager)
	arm.SetStrategy(strategy.StrategyRemoteHead(arm.childBranch, UPSTREAM_REMOTE_NAME, arm.childRepo))
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(authClient))
	if err != nil {
		return nil, err
	}

	fsrm := &fuchsiaSDKRepoManager{
		gcsClient:        gcs.NewGCSClient(storageClient, FUCHSIA_SDK_GS_BUCKET),
		gsBucket:         FUCHSIA_SDK_GS_BUCKET,
		storageClient:    storageClient,
		versionFileLinux: FUCHSIA_SDK_ANDROID_VERSION_FILE,
		versionFileMac:   "", // Ignored by this RepoManager.
	}
	parentRepo, err := git.NewCheckout(ctx, c.ParentRepo, workdir)
	if err != nil {
		return nil, err
	}
	genSdkBpRepo, err := git.NewCheckout(ctx, c.GenSdkBpRepo, workdir)
	if err != nil {
		return nil, err
	}
	rv := &fuchsiaSDKAndroidRepoManager{
		fuchsiaSDKRepoManager: fsrm,
		arm:                   arm,
		parentRepo:            parentRepo,
		genSdkBpRepo:          genSdkBpRepo,
	}
	ncrm, err := newNoCheckoutRepoManager(ctx, c.NoCheckoutRepoManagerConfig, workdir, g, serverURL, gitcookiesPath, authClient, cr, rv.buildCommitMessage, rv.updateHelper, local)
	if err != nil {
		return nil, err
	}
	rv.noCheckoutRepoManager = ncrm
	return rv, nil
}

// See documentation for noCheckoutRepoManagerUpdateHelperFunc.
func (rm *fuchsiaSDKAndroidRepoManager) updateHelper(ctx context.Context, strat strategy.NextRollStrategy, parentRepo *gitiles.Repo, baseCommit string) (string, string, int, map[string]string, error) {
	sklog.Info("Updating Android checkout...")
	if err := rm.arm.updateAndroidCheckout(ctx); err != nil {
		return "", "", 0, nil, err
	}

	sklog.Info("Finding next roll rev...")
	lastRollRev, nextRollRev, commitsNotRolled, _, err := rm.fuchsiaSDKRepoManager.updateHelper(ctx, strat, parentRepo, baseCommit)
	if err != nil {
		return "", "", 0, nil, err
	}

	if err := rm.parentRepo.Update(ctx); err != nil {
		return "", "", 0, nil, err
	}
	// Sync the parentRepo to baseCommit.
	if _, err := rm.parentRepo.Git(ctx, "reset", "--hard", baseCommit); err != nil {
		return "", "", 0, nil, err
	}
	if err := rm.genSdkBpRepo.Update(ctx); err != nil {
		return "", "", 0, nil, err
	}
	if _, err := rm.genSdkBpRepo.Git(ctx, "checkout", fmt.Sprintf("origin/%s", rm.parentBranch)); err != nil {
		return "", "", 0, nil, err
	}
	sklog.Info("Reading old file contents...")
	oldContents, err := fileutil.ReadAllFilesRecursive(rm.parentRepo.Dir())
	if err != nil {
		return "", "", 0, nil, err
	}
	for f, _ := range oldContents {
		if strings.HasPrefix(f, ".git/") {
			delete(oldContents, f)
		}
	}

	// Instead of simply rolling the version hash into a file, download and
	// unzip the SDK, and commit its contents.
	sdkGsPath := FUCHSIA_SDK_GS_PATH + "/linux-amd64/" + nextRollRev
	sklog.Infof("Downloading SDK from %s...", sdkGsPath)
	scratchDir, err := ioutil.TempDir("", "scratchDir")
	if err != nil {
		return "", "", 0, nil, err
	}
	defer util.RemoveAll(scratchDir)
	sdkZip := path.Join(scratchDir, "sdk.zip")
	if err := util.WithWriteFile(sdkZip, func(w io.Writer) error {
		r, err := rm.gcsClient.FileReader(ctx, sdkGsPath)
		if err != nil {
			return err
		}
		defer util.Close(r)
		_, err = io.Copy(w, r)
		return err
	}); err != nil {
		return "", "", 0, nil, err
	}

	// Unzip the SDK.
	sklog.Info("Unzipping SDK...")
	destDir, err := ioutil.TempDir("", "dest")
	if err != nil {
		return "", "", 0, nil, err
	}
	defer util.RemoveAll(destDir)
	if _, err := exec.RunCwd(ctx, destDir, "tar", "-zxvf", sdkZip); err != nil {
		return "", "", 0, nil, err
	}

	// Run the gen_sdk_bp.py script.
	genSdkBp := path.Join(rm.genSdkBpRepo.Dir(), GEN_SDK_BP_DIR, GEN_SDK_BP)
	sklog.Infof("Running %s...", genSdkBp)
	env := []string{fmt.Sprintf("ANDROID_BUILD_TOP=%s", rm.arm.workdir)}
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  destDir,
		Name: "python",
		Args: []string{genSdkBp},
		Env:  env,
	}); err != nil {
		return "", "", 0, nil, err
	}
	src := path.Join(rm.arm.workdir, "external", "fuchsia_sdk", ANDROID_BP)
	dst := path.Join(destDir, ANDROID_BP)
	if err := os.Rename(src, dst); err != nil {
		return "", "", 0, nil, err
	}

	// Determine the contents of the next roll.
	sklog.Info("Reading new file contents...")
	newContents, err := fileutil.ReadAllFilesRecursive(destDir)
	if err != nil {
		return "", "", 0, nil, err
	}
	nextRollContents := make(map[string]string, len(newContents))
	for f, contents := range newContents {
		if !bytes.Equal(oldContents[f], contents) {
			nextRollContents[f] = string(contents)
		}
	}
	for f, _ := range oldContents {
		if _, ok := newContents[f]; !ok {
			nextRollContents[f] = ""
		}
	}

	// Lastly, include the SDK version hash file.
	nextRollContents[rm.versionFileLinux] = nextRollRev
	sklog.Infof("Next roll modifies %d files.", len(nextRollContents))
	return lastRollRev, nextRollRev, commitsNotRolled, nextRollContents, nil
}
