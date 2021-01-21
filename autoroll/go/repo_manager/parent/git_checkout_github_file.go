package parent

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// GitCheckoutGithubFileConfig provides configuration for a Parent which uses a
// local Git checkout, uploads pull requests on Github, and pins dependencies
// using a file checked into the repo. The revision ID of the dependency makes
// up the full contents of the file, unless the file is "DEPS", which is a
// special case.
type GitCheckoutGithubFileConfig struct {
	GitCheckoutGithubConfig

	// Named steps to run before uploading roll CLs.
	PreUploadSteps []string `json:"preUploadSteps,omitempty"`
}

// Validate implements util.Validator.
func (c GitCheckoutGithubFileConfig) Validate() error {
	if err := c.GitCheckoutGithubConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.DependencyConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	for _, step := range c.PreUploadSteps {
		if _, err := GetPreUploadStep(step); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// GitCheckoutGithubFileConfigToProto converts a GitCheckoutGithubFileConfig to
// a config.GitCheckoutGithubFileParentConfig.
func GitCheckoutGithubFileConfigToProto(cfg *GitCheckoutGithubFileConfig) *config.GitCheckoutGitHubFileParentConfig {
	return &config.GitCheckoutGitHubFileParentConfig{
		GitCheckout:    GitCheckoutGithubConfigToProto(&cfg.GitCheckoutGithubConfig),
		PreUploadSteps: PreUploadStepsToProto(cfg.PreUploadSteps),
	}
}

// ProtoToGitCheckoutGithubFileConfig converts a
// config.GitCheckoutGitHubFileParentConfig to a GitCheckoutGithubFileConfig.
func ProtoToGitCheckoutGithubFileConfig(cfg *config.GitCheckoutGitHubFileParentConfig) (*GitCheckoutGithubFileConfig, error) {
	co, err := ProtoToGitCheckoutGithubConfig(cfg.GitCheckout)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &GitCheckoutGithubFileConfig{
		GitCheckoutGithubConfig: *co,
		PreUploadSteps:          ProtoToPreUploadSteps(cfg.PreUploadSteps),
	}, nil
}

// NewGitCheckoutGithubFile returns a Parent which uses a local checkout and a
// version file (eg. DEPS) to manage dependencies.
func NewGitCheckoutGithubFile(ctx context.Context, c GitCheckoutGithubFileConfig, reg *config_vars.Registry, client *http.Client, githubClient *github.GitHub, serverURL, workdir, userName, userEmail, rollerName string, co *git.Checkout) (*GitCheckoutParent, error) {
	// Pre-upload steps are run after setting the new dependency version and
	// syncing, but before committing and uploading.
	preUploadSteps, err := GetPreUploadSteps(c.PreUploadSteps)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	createRollHelper := gitCheckoutFileCreateRollFunc(c.DependencyConfig)
	createRoll := func(ctx context.Context, co *git.Checkout, from *revision.Revision, to *revision.Revision, rolling []*revision.Revision, commitMsg string) (string, error) {
		// Run the helper to add commits pointing to each of the Revision in the
		// roll.
		// TODO(borenet): This should be optional and configured in
		// GitCheckoutGithubFileConfig.
		prev := from
		for i := len(rolling) - 1; i >= 0; i-- {
			rev := rolling[i]
			msg := fmt.Sprintf("%s %s", rev.Id[:9], rev.Description)
			if _, err := createRollHelper(ctx, co, prev, rev, []*revision.Revision{rev}, msg); err != nil {
				return "", skerr.Wrap(err)
			}
		}

		// Run the pre-upload steps.
		sklog.Infof("Running %d pre-upload steps.", len(preUploadSteps))
		for _, s := range preUploadSteps {
			if err := s(ctx, nil, client, co.Dir()); err != nil {
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
	return NewGitCheckoutGithub(ctx, c.GitCheckoutGithubConfig, reg, githubClient, serverURL, workdir, userName, userEmail, rollerName, co, createRoll)
}
