package parent

/*
  Parent implementations which use a local checkout to create changes.
*/

import (
	"context"
	"fmt"
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

const (
	// rollBranch is the git branch which is used to create rolls.
	rollBranch = "roll_branch"
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
	createRoll GitCheckoutCreateRollFunc
	uploadRoll GitCheckoutUploadRollFunc
}

// GitCheckoutCreateRollFunc generates commit(s) in the local Git checkout to
// be used in the next roll and returns the hash of the commit to be uploaded.
// GitCheckoutParent handles creation of the roll branch.
type GitCheckoutCreateRollFunc func(context.Context, *git.Checkout, *revision.Revision, *revision.Revision, []*revision.Revision, string) (string, error)

// GitCheckoutUploadRollFunc uploads a CL using the given commit hash and
// returns its ID.
type GitCheckoutUploadRollFunc func(context.Context, *git.Checkout, string, string, []string, bool, string) (int64, error)

// NewGitCheckout returns a base for implementations of Parent which use
// a local checkout to create changes.
func NewGitCheckout(ctx context.Context, c GitCheckoutConfig, reg *config_vars.Registry, serverURL, workdir, userName, userEmail string, co *git.Checkout, createRoll GitCheckoutCreateRollFunc, uploadRoll GitCheckoutUploadRollFunc) (*GitCheckoutParent, error) {
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
	// Create the roll branch.
	_, upstreamBranch, err := p.Checkout.Update(ctx)
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	_, _ = p.Git(ctx, "branch", "-D", rollBranch) // Fails if the branch does not exist.
	if _, err := p.Git(ctx, "checkout", "-b", rollBranch, "-t", fmt.Sprintf("origin/%s", upstreamBranch)); err != nil {
		return 0, skerr.Wrap(err)
	}
	if _, err := p.Git(ctx, "reset", "--hard", upstreamBranch); err != nil {
		return 0, skerr.Wrap(err)
	}

	// Run the provided function to create the changes for the roll.
	hash, err := p.createRoll(ctx, p.Checkout.Checkout, from, to, rolling, commitMsg)
	if err != nil {
		return 0, skerr.Wrap(err)
	}

	// Ensure that createRoll generated at least one commit downstream of
	// p.baseCommit, and that it did not leave uncommitted changes.
	commits, err := p.RevList(ctx, "--ancestry-path", "--first-parent", fmt.Sprintf("%s..%s", upstreamBranch, hash))
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	if len(commits) == 0 {
		return 0, skerr.Fmt("createRoll generated no commits!")
	}
	if _, err := p.Git(ctx, "diff", "--quiet"); err != nil {
		return 0, skerr.Wrapf(err, "createRoll left uncommitted changes")
	}
	out, err := p.Git(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	if len(strings.Fields(out)) > 0 {
		return 0, skerr.Fmt("createRoll left untracked files:\n%s", out)
	}

	// Upload the CL.
	return p.uploadRoll(ctx, p.Checkout.Checkout, upstreamBranch, hash, emails, dryRun, commitMsg)
}

// gitCheckoutFileCreateRollFunc returns a GitCheckoutCreateRollFunc which uses
// a local Git checkout and pins dependencies using a file checked into the
// repo.
func gitCheckoutFileCreateRollFunc(dep version_file_common.DependencyConfig) GitCheckoutCreateRollFunc {
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
