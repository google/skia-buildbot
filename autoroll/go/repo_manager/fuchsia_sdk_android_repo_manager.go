package repo_manager

import (
	"context"
	"fmt"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	fuchsiaSDKAndroidVersionFile = "sdk_id"
	fuchsiaSDKAndroidGenScript   = "scripts/update_fuchsia_sdk.py"
)

// NewFuchsiaSDKAndroidRepoManager returns a RepoManager which rolls the Fuchsia
// SDK into Android. Unlike the fuchsiaSDKRepoManager, it actually unzips the
// contents of the SDK and checks them into the target repo. Additionally, it
// generates an Android.bp file.
func NewFuchsiaSDKAndroidRepoManager(ctx context.Context, c *config.FuchsiaSDKAndroidRepoManagerConfig, reg *config_vars.Registry, workdir, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Create the child.
	childRM, err := child.NewFuchsiaSDK(ctx, c.Child, client)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Create the parent.
	g, ok := cr.Client().(gerrit.GerritInterface)
	if !ok {
		return nil, skerr.Fmt("FuchsiaSDKAndroidRepoManager must use Gerrit for code review.")
	}
	genSdkBpRepo, err := git.NewCheckout(ctx, c.GenSdkBpRepo, workdir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	createRoll := fuchsiaSDKAndroidRepoManagerCreateRollFunc(genSdkBpRepo, c.GenSdkBpBranch, workdir)
	uploadRoll := parent.GitCheckoutUploadGerritRollFunc(g)
	parentRM, err := parent.NewGitCheckout(ctx, c.Parent, reg, workdir, cr, nil, createRoll, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := gerrit_common.SetupGerrit(ctx, parentRM.Checkout.Checkout, g); err != nil {
		return nil, skerr.Wrap(err)
	}
	return &parentChildRepoManager{
		Child:  childRM,
		Parent: parentRM,
	}, nil
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
		sklog.Infof("Running %s at %s", fuchsiaSDKAndroidGenScript, genSdkBpRepoHash)
		if _, err := exec.RunCommand(ctx, &exec.Command{
			Dir:        genSdkBpRepo.Dir(),
			Name:       "python",
			Env:        []string{fmt.Sprintf("ANDROID_BUILD_TOP=%s", androidTop)},
			Args:       []string{fuchsiaSDKAndroidGenScript, "--sdk_path", parentRepo.Dir()},
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
