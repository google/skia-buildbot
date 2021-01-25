package parent

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
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

	// ChildPath is the path to the child repo within the parent.
	ChildPath string `json:"childPath"`

	// Optional fields.

	// Relative path to the repo within the checkout root. Required if
	// GClientSpec is provided and specifies a name other than the default
	// obtained by "git clone".
	CheckoutPath string `json:"checkoutPath,omitempty"`

	// ChildSubdir indicates the subdirectory of the workdir in which
	// the childPath should be rooted. In most cases, this should be empty,
	// but if ChildPath is relative to the parent repo dir (eg. when DEPS
	// specifies use_relative_paths), then this is required.
	ChildSubdir string `json:"childSubdir,omitempty"`

	// Override the default gclient spec with this string.
	GClientSpec string `json:"gclientSpec,omitempty"`

	// Named steps to run before uploading roll CLs.
	PreUploadSteps []string `json:"preUploadSteps,omitempty"`

	// Run "gclient runhooks" if true.
	RunHooks bool `json:"runHooks,omitempty"`
}

// Validate implements util.Validator.
func (c DEPSLocalConfig) Validate() error {
	if err := c.GitCheckoutConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	for _, step := range c.PreUploadSteps {
		if _, err := GetPreUploadStep(step); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// DEPSLocalConfigToProto converts a DEPSLocalConfig to a
// config.DEPSLocalParentConfig.
func DEPSLocalConfigToProto(cfg *DEPSLocalConfig) *config.DEPSLocalParentConfig {
	return &config.DEPSLocalParentConfig{
		GitCheckout:    GitCheckoutConfigToProto(&cfg.GitCheckoutConfig),
		ChildPath:      cfg.ChildPath,
		CheckoutPath:   cfg.CheckoutPath,
		ChildSubdir:    cfg.ChildSubdir,
		GclientSpec:    cfg.GClientSpec,
		PreUploadSteps: PreUploadStepsToProto(cfg.PreUploadSteps),
		RunHooks:       cfg.RunHooks,
	}
}

// ProtoToDEPSLocalConfig converts a config.DEPSLocalParentConfig to a
// DEPSLocalConfig.
func ProtoToDEPSLocalConfig(cfg *config.DEPSLocalParentConfig) (*DEPSLocalConfig, error) {
	co, err := ProtoToGitCheckoutConfig(cfg.GitCheckout)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &DEPSLocalConfig{
		GitCheckoutConfig: *co,
		ChildPath:         cfg.ChildPath,
		CheckoutPath:      cfg.CheckoutPath,
		ChildSubdir:       cfg.ChildSubdir,
		GClientSpec:       cfg.GclientSpec,
		PreUploadSteps:    ProtoToPreUploadSteps(cfg.PreUploadSteps),
		RunHooks:          cfg.RunHooks,
	}, nil
}

// DEPSLocalGithubConfig provides configuration for a Parent which uses a local
// checkout and DEPS to manage dependencies, and uploads pull requests to
// GitHub.
type DEPSLocalGithubConfig struct {
	DEPSLocalConfig
	GitHub      *codereview.GithubConfig
	ForkRepoURL string `json:"forkRepoURL"`
}

// DEPSLocalGithubConfigToProto converts a DEPSLocalGithubConfig to a
// config.DEPSLocalGitHubParentConfig.
func DEPSLocalGithubConfigToProto(cfg *DEPSLocalGithubConfig) *config.DEPSLocalGitHubParentConfig {
	return &config.DEPSLocalGitHubParentConfig{
		DepsLocal:   DEPSLocalConfigToProto(&cfg.DEPSLocalConfig),
		Github:      codereview.GithubConfigToProto(cfg.GitHub),
		ForkRepoUrl: cfg.ForkRepoURL,
	}
}

// ProtoToDEPSLocalGithubConfig converts a config.DEPSLocalGitHubParentConfig to
// a DEPSLocalGithubConfig.
func ProtoToDEPSLocalGithubConfig(cfg *config.DEPSLocalGitHubParentConfig) (*DEPSLocalGithubConfig, error) {
	dl, err := ProtoToDEPSLocalConfig(cfg.DepsLocal)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &DEPSLocalGithubConfig{
		DEPSLocalConfig: *dl,
		GitHub:          codereview.ProtoToGithubConfig(cfg.Github),
		ForkRepoURL:     cfg.ForkRepoUrl,
	}, nil
}

// DEPSLocalGerritConfig provides configuration for a Parent which uses a local
// checkout and DEPS to manage dependencies, and uploads CLs to Gerrit.
type DEPSLocalGerritConfig struct {
	DEPSLocalConfig
	Gerrit *codereview.GerritConfig
}

// DEPSLocalGerritConfigToProto converts a DEPSLocalGerritConfig to a
// config.DEPSLocalGerritParentConfig.
func DEPSLocalGerritConfigToProto(cfg *DEPSLocalGerritConfig) *config.DEPSLocalGerritParentConfig {
	return &config.DEPSLocalGerritParentConfig{
		DepsLocal: DEPSLocalConfigToProto(&cfg.DEPSLocalConfig),
		Gerrit:    codereview.GerritConfigToProto(cfg.Gerrit),
	}
}

// ProtoToDEPSLocalGerritConfig converts a config.DEPSLocalGerritParentConfig to
// a DEPSLocalGerritConfig.
func ProtoToDEPSLocalGerritConfig(cfg *config.DEPSLocalGerritParentConfig) (*DEPSLocalGerritConfig, error) {
	dl, err := ProtoToDEPSLocalConfig(cfg.DepsLocal)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &DEPSLocalGerritConfig{
		DEPSLocalConfig: *dl,
		Gerrit:          codereview.ProtoToGerritConfig(cfg.Gerrit),
	}, nil
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

		syncExtraArgs := []string{}
		if strings.HasPrefix(to.Id, gerrit.CHANGE_REF_PREFIX) {
			// If the rev is a patch ref then add patch-ref args to gclient sync. Syncing
			// the child repo will not work without these args.
			syncExtraArgs = append(syncExtraArgs,
				"--patch-ref", fmt.Sprintf("%s@%s:%s", c.DependencyConfig.ID, c.Branch, to.Id),
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
