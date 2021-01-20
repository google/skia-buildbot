package repo_manager

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	FuchsiaSDKAndroidVersionFile = "sdk_id"
	FuchsiaSDKAndroidGenScript   = "scripts/update_fuchsia_sdk.py"
)

// FuchsiaSDKAndroidRepoManagerConfig provides configuration for
// FuchsiaSDKAndroidRepoManager.
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
	// Set some unused variables on the embedded RepoManager.
	br, err := config_vars.NewTemplate("N/A")
	if err != nil {
		panic(err)
	}
	c.ChildBranch = br
	c.ChildPath = "N/A"
	c.ChildRevLinkTmpl = "N/A"
	if err := c.FuchsiaSDKRepoManagerConfig.Validate(); err != nil {
		return err
	}
	// Unset the unused variables.
	c.ChildBranch = nil
	c.ChildPath = ""
	c.ChildRevLinkTmpl = ""

	if c.GenSdkBpRepo == "" {
		return errors.New("GenSdkBpRepo is required.")
	}
	if c.GenSdkBpBranch == "" {
		return errors.New("GenSdkBpBranch is required.")
	}
	return nil
}

// NewFuchsiaSDKAndroidRepoManager returns a RepoManager which rolls the Fuchsia
// SDK into Android. Unlike the fuchsiaSDKRepoManager, it actually unzips the
// contents of the SDK and checks them into the target repo. Additionally, it
// generates an Android.bp file.
func NewFuchsiaSDKAndroidRepoManager(ctx context.Context, c *FuchsiaSDKAndroidRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, serverURL string, authClient *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Create the child.
	childCfg := child.FuchsiaSDKConfig{
		IncludeMacSDK: false,
	}
	childRM, err := child.NewFuchsiaSDK(ctx, childCfg, authClient)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Create the parent.
	parentCfg := parent.GitCheckoutConfig{
		GitCheckoutConfig: git_common.GitCheckoutConfig{
			Branch:  c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentBranch,
			RepoURL: c.NoCheckoutRepoManagerConfig.CommonRepoManagerConfig.ParentRepo,
		},
		DependencyConfig: version_file_common.DependencyConfig{
			VersionFileConfig: version_file_common.VersionFileConfig{
				ID:   "FuchsiaSDK",
				Path: FuchsiaSDKAndroidVersionFile,
			},
		},
	}
	genSdkBpRepo, err := git.NewCheckout(ctx, c.GenSdkBpRepo, workdir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	createRoll := fuchsiaSDKAndroidRepoManagerCreateRollFunc(genSdkBpRepo, c.GenSdkBpBranch, workdir)
	uploadRoll := parent.GitCheckoutUploadGerritRollFunc(g)
	parentRM, err := parent.NewGitCheckout(ctx, parentCfg, reg, serverURL, workdir, cr.UserName(), cr.UserEmail(), nil, createRoll, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := gerrit_common.SetupGerrit(ctx, parentRM.Checkout.Checkout, g); err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}

// fuchsiaSDKAndroidRepoManagerCreateRollFunc returns a
// parent.GitilesLocalCreateRollFunc which rolls the Fuchsia SDK.
func fuchsiaSDKAndroidRepoManagerCreateRollFunc(genSdkBpRepo *git.Checkout, genSdkBpBranch, androidTop string) git_common.CreateRollFunc {
	return func(ctx context.Context, parentRepo *git.Checkout, from, to *revision.Revision, rolling []*revision.Revision, commitMsg string) (string, error) {
		// Sync the genSdkBpRepo.
		if err := genSdkBpRepo.UpdateBranch(ctx, genSdkBpBranch); err != nil {
			return "", skerr.Wrap(err)
		}
		genSdkBpRepoHash, err := genSdkBpRepo.RevParse(ctx, "HEAD")
		if err != nil {
			return "", skerr.Wrap(err)
		}

		// Log "git status" before updating the SDK.
		st, err := parentRepo.Git(ctx, "status")
		if err != nil {
			return "", skerr.Wrap(err)
		}
		sklog.Info(st)

		// Instead of simply rolling the version hash into a file, download and
		// unzip the SDK by running the update_fuchsia_sdk script, and commit
		// its contents.
		sklog.Infof("Running %s at %s", FuchsiaSDKAndroidGenScript, genSdkBpRepoHash)
		if _, err := exec.RunCommand(ctx, &exec.Command{
			Dir:        genSdkBpRepo.Dir(),
			Name:       "python",
			Env:        []string{fmt.Sprintf("ANDROID_BUILD_TOP=%s", androidTop)},
			Args:       []string{FuchsiaSDKAndroidGenScript, "--sdk_path", parentRepo.Dir()},
			InheritEnv: true,
			LogStdout:  true,
			LogStderr:  true,
			Verbose:    exec.Info,
		}); err != nil {
			return "", skerr.Wrap(err)
		}

		// Log "git status" after updating the SDK.
		st, err = parentRepo.Git(ctx, "status")
		if err != nil {
			return "", skerr.Wrap(err)
		}
		sklog.Info(st)

		// Commit. Use -A because we're just taking all of the changes produced
		// by FuchsiaSDKAndroidGenScript.
		if _, err := parentRepo.Git(ctx, "add", "-A"); err != nil {
			return "", skerr.Wrap(err)
		}
		if _, err := parentRepo.Git(ctx, "commit", "-m", commitMsg); err != nil {
			return "", skerr.Wrap(err)
		}
		hash, err := parentRepo.RevParse(ctx, "HEAD")
		if err != nil {
			return "", skerr.Wrap(err)
		}
		// Log "git status" after committing.
		st, err = parentRepo.Git(ctx, "status")
		if err != nil {
			return "", skerr.Wrap(err)
		}
		sklog.Info(st)
		return hash, nil
	}
}
