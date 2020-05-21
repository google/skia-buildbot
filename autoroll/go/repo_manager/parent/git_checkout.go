package parent

/*
  Parent implementations which use a local checkout to create changes.
*/

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

// GitCheckoutConfig provides configuration for a Parent which uses a local
// checkout to create changes.
type GitCheckoutConfig struct {
	git_common.GitCheckoutConfig
	version_file_common.DependencyConfig
}

// See documentation for util.Validator interface.
func (c GitCheckoutConfig) Validate() error {
	if err := c.GitCheckoutConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.DependencyConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if len(c.GitCheckoutConfig.Dependencies) != 0 {
		return skerr.Fmt("Dependencies are inherited from the DependencyConfig and should not be set on the GitCheckoutConfig.")
	}
	return nil
}

// GitCheckoutParent is a base for implementations of Parent which use a local
// Git checkout.
type GitCheckoutParent struct {
	*git_common.Checkout
	childID    string
	createRoll git_common.CreateRollFunc
	uploadRoll git_common.UploadRollFunc
}

// NewGitCheckout returns a base for implementations of Parent which use
// a local checkout to create changes.
func NewGitCheckout(ctx context.Context, c GitCheckoutConfig, reg *config_vars.Registry, serverURL, workdir, userName, userEmail string, co *git.Checkout, createRoll git_common.CreateRollFunc, uploadRoll git_common.UploadRollFunc) (*GitCheckoutParent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	// Create the local checkout.
	deps := make([]*version_file_common.VersionFileConfig, 0, len(c.DependencyConfig.TransitiveDeps)+1)
	deps = append(deps, &c.DependencyConfig.VersionFileConfig)
	for _, td := range c.TransitiveDeps {
		deps = append(deps, td.Parent)
	}
	c.GitCheckoutConfig.Dependencies = deps
	checkout, err := git_common.NewCheckout(ctx, c.GitCheckoutConfig, reg, workdir, userName, userEmail, co)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &GitCheckoutParent{
		Checkout:   checkout,
		childID:    c.DependencyConfig.ID,
		createRoll: createRoll,
		uploadRoll: uploadRoll,
	}, nil
}

// See documentation for Parent interface.
func (p *GitCheckoutParent) Update(ctx context.Context) (string, error) {
	rev, _, err := p.Checkout.Update(ctx)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	lastRollRev, ok := rev.Dependencies[p.childID]
	if !ok {
		return "", skerr.Fmt("Unable to find dependency %q in %#v", p.childID, rev)
	}
	return lastRollRev, nil
}

// See documentation for Parent interface.
func (p *GitCheckoutParent) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, dryRun bool, commitMsg string) (int64, error) {
	return p.Checkout.CreateNewRoll(ctx, from, to, rolling, emails, dryRun, commitMsg, p.createRoll, p.uploadRoll)
}

// gitCheckoutFileCreateRollFunc returns a GitCheckoutCreateRollFunc which uses
// a local Git checkout and pins dependencies using a file checked into the
// repo.
func gitCheckoutFileCreateRollFunc(dep version_file_common.DependencyConfig) git_common.CreateRollFunc {
	return func(ctx context.Context, co *git.Checkout, from *revision.Revision, to *revision.Revision, rolling []*revision.Revision, commitMsg string) (string, error) {
		// Determine what changes need to be made.
		getFile := func(ctx context.Context, path string) (string, error) {
			return co.GetFile(ctx, path, "HEAD")
		}
		changes, err := version_file_common.UpdateDep(ctx, dep, to, getFile)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		// Perform the changes.
		for path, contents := range changes {
			fullPath := filepath.Join(co.Dir(), path)
			if err := ioutil.WriteFile(fullPath, []byte(contents), os.ModePerm); err != nil {
				return "", skerr.Wrap(err)
			}
			if _, err := co.Git(ctx, "add", path); err != nil {
				return "", skerr.Wrap(err)
			}
		}
		// Commit.
		if _, err := co.Git(ctx, "commit", "-m", commitMsg); err != nil {
			return "", skerr.Wrap(err)
		}
		out, err := co.RevParse(ctx, "HEAD")
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return strings.TrimSpace(out), nil
	}
}

var _ Parent = &GitCheckoutParent{}
