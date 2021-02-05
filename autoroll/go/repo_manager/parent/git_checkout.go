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

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

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
func NewGitCheckout(ctx context.Context, c *config.GitCheckoutParentConfig, reg *config_vars.Registry, workdir string, cr codereview.CodeReview, co *git.Checkout, createRoll git_common.CreateRollFunc, uploadRoll git_common.UploadRollFunc) (*GitCheckoutParent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	// Create the local checkout.
	// TODO(borenet): Don't modify the passed-in config!
	deps := make([]*config.VersionFileConfig, 0, len(c.Dep.Transitive)+1)
	deps = append(deps, c.Dep.Primary)
	for _, td := range c.Dep.Transitive {
		deps = append(deps, td.Parent)
	}
	c.GitCheckout.Dependencies = deps
	checkout, err := git_common.NewCheckout(ctx, c.GitCheckout, reg, workdir, cr.UserName(), cr.UserEmail(), co)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &GitCheckoutParent{
		Checkout:   checkout,
		childID:    c.Dep.Primary.Id,
		createRoll: createRoll,
		uploadRoll: uploadRoll,
	}, nil
}

// Update implements Parent.
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

// CreateNewRoll implements Parent.
func (p *GitCheckoutParent) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, dryRun bool, commitMsg string) (int64, error) {
	return p.Checkout.CreateNewRoll(ctx, from, to, rolling, emails, dryRun, commitMsg, p.createRoll, p.uploadRoll)
}

// gitCheckoutFileCreateRollFunc returns a GitCheckoutCreateRollFunc which uses
// a local Git checkout and pins dependencies using a file checked into the
// repo.
func gitCheckoutFileCreateRollFunc(dep *config.DependencyConfig) git_common.CreateRollFunc {
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
