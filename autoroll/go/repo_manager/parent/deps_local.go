package parent

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	GClient = "gclient.py"
)

// DEPSLocalConfig provides configuration for a Parent which uses a local
// checkout and DEPS to manage dependencies.
type DEPSLocalConfig struct {
	GitCheckoutConfig

	// Optional fields.

	// Relative path to the repo within the checkout root. Required if
	// GClientSpec is provided and specifies a name other than the default
	// obtained by "git clone".
	CheckoutPath string `json:"checkoutPath,omitempty"`

	// Override the default gclient spec with this string.
	GClientSpec string `json:"gclientSpec,omitempty"`

	// Named steps to run before uploading roll CLs.
	PreUploadSteps []string `json:"preUploadSteps,omitempty"`

	// Run "gclient runhooks" if true.
	RunHooks bool `json:"runHooks,omitempty"`
}

// See documentation for util.Validator interface.
func (c DEPSLocalConfig) Validate() error {
	if err := c.GitCheckoutConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.DependencyConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.ID == "" {
		return skerr.Fmt("Dep is required")
	}
	for _, step := range c.PreUploadSteps {
		if _, err := GetPreUploadStep(step); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// NewDEPSLocal returns a Parent which uses a local checkout and DEPS to manage
// dependencies.
func NewDEPSLocal(ctx context.Context, c DEPSLocalConfig, reg *config_vars.Registry, client *http.Client, serverURL, workdir, userName, userEmail, recipeCfgFile string, uploadRoll git_common.UploadRollFunc) (*GitCheckoutParent, error) {
	// Validation.
	if err := c.Validate(); err != nil {
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
	sync := func(ctx context.Context) error {
		args := []string{"sync", "--delete_unversioned_trees", "--force"}
		if !c.RunHooks {
			args = append(args, "--nohooks")
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
	if c.GClientSpec != "" {
		args = append(args, fmt.Sprintf("--spec=%s", c.GClientSpec))
	} else {
		args = append(args, c.RepoURL, "--unmanaged")
	}
	if err := gclient(ctx, args...); err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := sync(ctx); err != nil {
		return nil, skerr.Wrap(err)
	}

	// See documentation for GitCheckoutCreateRollFunc.
	createRollHelper := gitCheckoutFileCreateRollFunc(version_file_common.DependencyConfig{
		VersionFileConfig: version_file_common.VersionFileConfig{
			ID:   c.ID,
			Path: deps_parser.DepsFileName,
		},
		TransitiveDeps: c.TransitiveDeps,
	})
	createRoll := func(ctx context.Context, co *git.Checkout, from *revision.Revision, to *revision.Revision, rolling []*revision.Revision, commitMsg string) (string, error) {
		// Run the helper to set the new dependency version(s).
		if _, err := createRollHelper(ctx, co, from, to, rolling, commitMsg); err != nil {
			return "", skerr.Wrap(err)
		}

		// Run "gclient sync" to sync dependencies.
		if err := sync(ctx); err != nil {
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
	return NewGitCheckout(ctx, c.GitCheckoutConfig, reg, serverURL, workdir, userName, userEmail, co, createRoll, uploadRoll)
}

// GetDEPSCheckoutPath returns the path to the checkout within the workdir,
// using the given DEPSLocalConfig.
func GetDEPSCheckoutPath(c DEPSLocalConfig, workdir string) (string, error) {
	var repoRelPath string
	if c.CheckoutPath == "" {
		normUrl, err := git.NormalizeURL(c.RepoURL)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		repoRelPath = path.Base(normUrl)
	} else {
		repoRelPath = c.CheckoutPath
	}
	return filepath.Join(workdir, repoRelPath), nil
}
