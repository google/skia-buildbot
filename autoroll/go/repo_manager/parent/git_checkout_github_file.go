package parent

import (
	"context"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// GitCheckoutGithubFileConfig provides configuration for a Parent which uses a
// local Git checkout, uploads pull requests on Github, and pins dependencies
// using a file checked into the repo. The revision ID of the dependency makes
// up the full contents of the file, unless the file is "DEPS", which is a
// special case.
type GitCheckoutGithubFileConfig struct {
	GitCheckoutGithubConfig
	version_file_common.DependencyConfig
}

// See documentation for util.Validator interface.
func (c GitCheckoutGithubFileConfig) Validate() error {
	if err := c.GitCheckoutGithubConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.DependencyConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// NewGitCheckoutGithubFile returns a Parent which uses a local checkout and a
// version file (eg. DEPS) to manage dependencies.
func NewGitCheckoutGithubFile(ctx context.Context, c GitCheckoutGithubFileConfig, reg *config_vars.Registry, githubClient *github.GitHub, serverURL, workdir, userName, userEmail string, co *git.Checkout) (*GitCheckoutParent, error) {
	getLastRollRev := gitCheckoutFileGetLastRollRevFunc(c.VersionFileConfig)
	createRoll := gitCheckoutFileCreateRollFunc(c.DependencyConfig)
	return NewGitCheckoutGithub(ctx, c.GitCheckoutGithubConfig, reg, githubClient, serverURL, workdir, userName, userEmail, co, getLastRollRev, createRoll)
}
