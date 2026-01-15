package parent

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	autoroll_git_common "go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/github_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	GClient = "gclient.py"
)

// NewDEPSLocal returns a Parent which uses a local checkout and DEPS to manage
// dependencies.
func NewDEPSLocal(ctx context.Context, c *config.DEPSLocalParentConfig, client *http.Client, serverURL, workdir string, cr codereview.CodeReview, childBranch string, uploadRoll autoroll_git_common.UploadRollFunc, applyExternalChangeFunc autoroll_git_common.ApplyExternalChangeFunc) (*GitCheckoutParent, error) {
	// Validation.
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}

	gitPath, err := git.Executable(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Set up depot tools.
	depotTools, err := depot_tools.GetDepotTools(ctx, workdir)
	if err != nil {
		return nil, skerr.Wrapf(err, "setting up depot tools in %s", workdir)
	}
	depotToolsEnv := append(depot_tools.Env(depotTools), "SKIP_GCE_AUTH_FOR_GIT=1")
	depotToolsEnv = append(depotToolsEnv, "GIT_TRACE=1")
	for i, envVar := range depotToolsEnv {
		split := strings.SplitN(envVar, "=", 2)
		if len(split) == 2 && split[0] == "PATH" {
			pathEnvVarValue := fmt.Sprintf("%s:%s", filepath.Dir(gitPath), split[1])
			depotToolsEnv[i] = fmt.Sprintf("PATH=%s", pathEnvVarValue)
			if err := os.Setenv("PATH", pathEnvVarValue); err != nil {
				return nil, skerr.Wrap(err)
			}
		}
	}

	// Create the checkout dir.
	parentCheckoutDir := workdir
	if c.ParentSubdir != "" {
		parentCheckoutDir = filepath.Join(workdir, c.ParentSubdir)
	}
	if err := os.MkdirAll(parentCheckoutDir, os.ModePerm); err != nil {
		return nil, skerr.Wrapf(err, "failed to create parent checkout dir %q", parentCheckoutDir)
	}

	gclientCmd := []string{filepath.Join(depotTools, GClient)}
	// gclient requires the use of vpython3 to bring in needed dependencies.
	vpythonBinary := "vpython3"
	gclient := func(ctx context.Context, cmd ...string) error {
		args := append(gclientCmd, cmd...)
		sklog.Infof("Running: %s %s", vpythonBinary, strings.Join(args, " "))
		_, err := exec.RunCommand(ctx, &exec.Command{
			Dir:        parentCheckoutDir,
			Env:        depotToolsEnv,
			Name:       vpythonBinary,
			Args:       args,
			InheritEnv: true,
		})
		return skerr.Wrap(err)
	}
	sync := func(ctx context.Context, extraArgs ...string) error {
		args := []string{"sync", "--delete_unversioned_trees", "--force", "--reset", "-v"}
		if !c.RunHooks {
			args = append(args, "--nohooks")
		}
		if len(extraArgs) > 0 {
			args = append(args, extraArgs...)
		}
		if err := gclient(ctx, args...); err != nil {
			return skerr.Wrap(err)
		}
		return skerr.Wrap(gclient(ctx, "recurse", "--no-progress", "-j1", gitPath, "clean", "-d", "-f"))
	}

	// Clean up any lockfiles, in case the process was interrupted.
	if err := git.DeleteLockFiles(ctx, workdir); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Create the Git checkout. DEPS allows the parent repo to be flexibly
	// located, ie. not necessarily in the root of the workdir, so we can't
	// rely on GitCheckoutParent to do the right thing.
	args := []string{"config"}
	if c.GclientSpec != "" {
		args = append(args, fmt.Sprintf("--spec=%s", c.GclientSpec))
	} else {
		args = append(args, c.GitCheckout.GitCheckout.RepoUrl, "--unmanaged")
	}
	if err := gclient(ctx, args...); err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := sync(ctx); err != nil {
		return nil, skerr.Wrap(err)
	}

	// See documentation for GitCheckoutCreateRollFunc.
	createRollHelper := gitCheckoutFileCreateRollFunc(&config.DependencyConfig{
		Primary: &config.VersionFileConfig{
			Id: c.GitCheckout.Dep.Primary.Id,
			File: []*config.VersionFileConfig_File{
				{Path: deps_parser.DepsFileName},
			},
		},
		Transitive:     c.GitCheckout.Dep.Transitive,
		FindAndReplace: c.GitCheckout.Dep.FindAndReplace,
	})
	createRoll := func(ctx context.Context, co git.Checkout, from *revision.Revision, to *revision.Revision, rolling []*revision.Revision, commitMsg string) (string, error) {
		// Run the helper to set the new dependency version(s).
		if _, err := createRollHelper(ctx, co, from, to, rolling, commitMsg); err != nil {
			return "", skerr.Wrap(err)
		}

		syncExtraArgs := []string{}
		if strings.HasPrefix(to.Id, gerrit.ChangeRefPrefix) {
			if childBranch == "" {
				return "", skerr.Fmt("No child branch is available; don't know where to base the CL")
			}
			// If the rev is a patch ref then add patch-ref args to gclient sync. Syncing
			// the child repo will not work without these args.
			syncExtraArgs = append(syncExtraArgs,
				"--patch-ref", fmt.Sprintf("%s@%s:%s", c.GitCheckout.Dep.Primary.Id, childBranch, to.Id),
				"--no-rebase-patch-ref",
				"--no-reset-patch-ref",
			)
		}
		if err := sync(ctx, syncExtraArgs...); err != nil {
			return "", skerr.Wrap(err)
		}

		// Sometimes DEPS are added without an associated .gitignore entry,
		// which results in untracked files after `gclient sync`.  If there are
		// any untracked files when this function finishes, the roller will fail
		// to create a CL.  Remove any untracked files immediately after syncing
		// to prevent any errors which aren't our fault.
		if _, err := co.Git(ctx, "clean", "-d", "-f"); err != nil {
			return "", skerr.Wrap(err)
		}

		// Handle ExternalChangeId if func is specified.
		if applyExternalChangeFunc != nil && to.ExternalChangeId != "" {
			if err := applyExternalChangeFunc(ctx, co, to.ExternalChangeId); err != nil {
				return "", skerr.Wrap(err)
			}
		}

		// Run the pre-upload steps.
		if err := RunPreUploadStep(ctx, c.PreUploadCommands, depotToolsEnv, client, co.Dir(), from, to); err != nil {
			return "", skerr.Wrapf(err, "failed pre-upload step: %s", err)
		}

		// Commit.
		if _, err := co.Git(ctx, "commit", "-a", "--amend", "--no-edit"); err != nil {
			return "", skerr.Wrap(err)
		}
		out, err := co.RevParse(ctx, "HEAD")
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return strings.TrimSpace(out), nil
	}

	// Find the checkout within the workdir.
	checkoutPath, err := GetDEPSCheckoutPath(c, parentCheckoutDir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	co := git.CheckoutDir(checkoutPath)
	return NewGitCheckout(ctx, c.GitCheckout, checkoutPath, cr, co, createRoll, uploadRoll)
}

// GetDEPSCheckoutPath returns the path to the checkout within the workdir,
// using the given DEPSLocalConfig.
func GetDEPSCheckoutPath(c *config.DEPSLocalParentConfig, parentCheckoutDir string) (string, error) {
	var repoRelPath string
	if c.CheckoutPath == "" {
		normUrl, err := git.NormalizeURL(c.GitCheckout.GitCheckout.RepoUrl)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		repoRelPath = path.Base(normUrl)
	} else {
		repoRelPath = c.CheckoutPath
	}
	return filepath.Join(parentCheckoutDir, repoRelPath), nil
}

// NewDEPSLocalGitHub returns a DEPSLocal parent which creates GitHub pull
// requests.
func NewDEPSLocalGitHub(ctx context.Context, c *config.DEPSLocalGitHubParentConfig, client *http.Client, serverURL, workdir, rollerName string, cr codereview.CodeReview, childBranch string) (*GitCheckoutParent, error) {
	githubClient, ok := cr.Client().(*github.GitHub)
	if !ok {
		return nil, skerr.Fmt("DEPSLocalGitHub must use GitHub for code review.")
	}
	uploadRoll := GitCheckoutUploadGithubRollFunc(githubClient, cr.UserName(), rollerName, c.ForkRepoUrl)
	parentRM, err := NewDEPSLocal(ctx, c.DepsLocal, client, serverURL, workdir, cr, childBranch, uploadRoll, ApplyExternalChangeGithubFunc())
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := github_common.SetupGithub(ctx, parentRM.Checkout.Checkout, c.ForkRepoUrl); err != nil {
		return nil, skerr.Wrap(err)
	}
	return parentRM, nil
}

// NewDEPSLocalGerrit returns a DEPSLocal parent which creates CLs in Gerrit.
func NewDEPSLocalGerrit(ctx context.Context, c *config.DEPSLocalGerritParentConfig, client *http.Client, serverURL, workdir, rollerName string, cr codereview.CodeReview, childBranch string) (*GitCheckoutParent, error) {
	gerritClient, ok := cr.Client().(gerrit.GerritInterface)
	if !ok {
		return nil, skerr.Fmt("DEPSLocalGitHub must use GitHub for code review.")
	}
	uploadRoll := GitCheckoutUploadGerritRollFunc(gerritClient)
	parentRM, err := NewDEPSLocal(ctx, c.DepsLocal, client, serverURL, workdir, cr, childBranch, uploadRoll, nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := gerrit_common.SetupGerrit(ctx, parentRM.Checkout.Checkout, gerritClient); err != nil {
		return nil, skerr.Wrap(err)
	}
	return parentRM, nil
}
