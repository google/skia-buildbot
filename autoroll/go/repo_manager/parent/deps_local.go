package parent

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/proto"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
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
func NewDEPSLocal(ctx context.Context, c *proto.DEPSLocalParentConfig, reg *config_vars.Registry, client *http.Client, serverURL, workdir, recipeCfgFile string, cr codereview.CodeReview, uploadRoll git_common.UploadRollFunc) (*GitCheckoutParent, error) {
	// Validation.
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}

	branch, err := config_vars.NewTemplate(c.GitCheckout.GitCheckout.Branch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := reg.Register(branch); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Set up depot tools.
	depotTools, err := depot_tools.GetDepotTools(ctx, workdir, recipeCfgFile)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	depotToolsEnv := append(depot_tools.Env(depotTools), "SKIP_GCE_AUTH_FOR_GIT=1")
	gclientCmd := []string{filepath.Join(depotTools, GClient)}
	gclient := func(ctx context.Context, cmd ...string) error {
		args := append(gclientCmd, cmd...)
		sklog.Infof("Running: %s %s", "python", strings.Join(args, " "))
		_, err := exec.RunCommand(ctx, &exec.Command{
			Dir:  workdir,
			Env:  depotToolsEnv,
			Name: "python",
			Args: args,
		})
		return skerr.Wrap(err)
	}
	sync := func(ctx context.Context, extraArgs ...string) error {
		args := []string{"sync", "--delete_unversioned_trees", "--force"}
		if !c.RunHooks {
			args = append(args, "--nohooks")
		}
		if len(extraArgs) > 0 {
			args = append(args, extraArgs...)
		}
		return skerr.Wrap(gclient(ctx, args...))
	}

	// Pre-upload steps are run after setting the new dependency version and
	// syncing, but before committing and uploading.
	preUploadSteps, err := GetPreUploadSteps(c.PreUploadSteps)
	if err != nil {
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
	createRollHelper := gitCheckoutFileCreateRollFunc(&proto.DependencyConfig{
		Primary: &proto.VersionFileConfig{
			Id:   c.GitCheckout.Dep.Primary.Id,
			Path: deps_parser.DepsFileName,
		},
		Transitive: c.GitCheckout.Dep.Transitive,
	})
	createRoll := func(ctx context.Context, co *git.Checkout, from *revision.Revision, to *revision.Revision, rolling []*revision.Revision, commitMsg string) (string, error) {
		// Run the helper to set the new dependency version(s).
		if _, err := createRollHelper(ctx, co, from, to, rolling, commitMsg); err != nil {
			return "", skerr.Wrap(err)
		}

		syncExtraArgs := []string{}
		if strings.HasPrefix(to.Id, gerrit.CHANGE_REF_PREFIX) {
			// If the rev is a patch ref then add patch-ref args to gclient sync. Syncing
			// the child repo will not work without these args.
			syncExtraArgs = append(syncExtraArgs,
				"--patch-ref", fmt.Sprintf("%s@%s:%s", c.GitCheckout.Dep.Primary.Id, branch, to.Id),
				"--no-rebase-patch-ref",
				"--no-reset-patch-ref",
			)
		}
		if err := sync(ctx, syncExtraArgs...); err != nil {
			return "", skerr.Wrap(err)
		}

		// Run the pre-upload steps.
		sklog.Infof("Running %d pre-upload steps.", len(preUploadSteps))
		for _, s := range preUploadSteps {
			if err := s(ctx, depotToolsEnv, client, co.Dir()); err != nil {
				return "", skerr.Wrapf(err, "failed pre-upload step")
			}
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
	checkoutPath, err := GetDEPSCheckoutPath(c, workdir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	co := &git.Checkout{GitDir: git.GitDir(checkoutPath)}
	return NewGitCheckout(ctx, c.GitCheckout, reg, workdir, cr, co, createRoll, uploadRoll)
}

// GetDEPSCheckoutPath returns the path to the checkout within the workdir,
// using the given DEPSLocalproto.
func GetDEPSCheckoutPath(c *proto.DEPSLocalParentConfig, workdir string) (string, error) {
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
	return filepath.Join(workdir, repoRelPath), nil
}

// NewDEPSLocalGitHub returns a DEPSLocal parent which creates GitHub pull
// requests.
func NewDEPSLocalGitHub(ctx context.Context, c *proto.DEPSLocalGitHubParentConfig, reg *config_vars.Registry, client *http.Client, serverURL, workdir, rollerName, recipeCfgFile string, cr codereview.CodeReview) (*GitCheckoutParent, error) {
	githubClient, ok := cr.Client().(*github.GitHub)
	if !ok {
		return nil, skerr.Fmt("DEPSLocalGitHub must use GitHub for code review.")
	}
	uploadRoll := GitCheckoutUploadGithubRollFunc(githubClient, cr.UserName(), rollerName, c.ForkRepoUrl)
	parentRM, err := NewDEPSLocal(ctx, c.DepsLocal, reg, client, serverURL, workdir, recipeCfgFile, cr, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := github_common.SetupGithub(ctx, parentRM.Checkout.Checkout, c.ForkRepoUrl); err != nil {
		return nil, skerr.Wrap(err)
	}
	return parentRM, nil
}

// NewDEPSLocalGerrit returns a DEPSLocal parent which creates CLs in Gerrit.
func NewDEPSLocalGerrit(ctx context.Context, c *proto.DEPSLocalGerritParentConfig, reg *config_vars.Registry, client *http.Client, serverURL, workdir, rollerName, recipeCfgFile string, cr codereview.CodeReview) (*GitCheckoutParent, error) {
	gerritClient, ok := cr.Client().(gerrit.GerritInterface)
	if !ok {
		return nil, skerr.Fmt("DEPSLocalGitHub must use GitHub for code review.")
	}
	uploadRoll := GitCheckoutUploadGerritRollFunc(gerritClient)
	parentRM, err := NewDEPSLocal(ctx, c.DepsLocal, reg, client, serverURL, workdir, recipeCfgFile, cr, uploadRoll)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := gerrit_common.SetupGerrit(ctx, parentRM.Checkout.Checkout, gerritClient); err != nil {
		return nil, skerr.Wrap(err)
	}
	return parentRM, nil
}
