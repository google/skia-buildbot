package parent

/*
  Parent implementations which use a local checkout to create changes.
*/

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/config_vars"
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
	BaseConfig
	Branch  *config_vars.Template `json:"branch"`
	RepoURL string                `json:"repoURL"`
}

// GitCheckoutParent is a base for implementations of Parent which use a local
// Git checkout.
type GitCheckoutParent struct {
	*baseParent
	branch         *config_vars.Template
	co             *git.Checkout
	createRoll     GitCheckoutCreateRollFunc
	getLastRollRev GitCheckoutGetLastRollRevFunc
	uploadRoll     GitCheckoutUploadRollFunc
}

// GitCheckoutCreateRollFunc generates commit(s) in the local Git checkout to
// be used in the next roll and returns the hash of the commit to be uploaded.
// GitCheckoutParent handles creation of the roll branch.
type GitCheckoutCreateRollFunc func(context.Context, *git.Checkout, *revision.Revision, *revision.Revision, []*revision.Revision, string) (string, error)

// GitCheckoutUploadRollFunc uploads a CL using the given commit hash and
// returns its ID.
type GitCheckoutUploadRollFunc func(context.Context, *git.Checkout, string, string, []string, bool) (int64, error)

// GitCheckoutGetLastRollRevFunc retrieves the last-rolled revision ID from the
// local Git checkout. GitCheckoutParent handles updating the checkout itself.
type GitCheckoutGetLastRollRevFunc func(context.Context, *git.Checkout) (string, error)

// NewGitCheckout returns a base for implementations of Parent which use
// a local checkout to create changes.
func NewGitCheckout(ctx context.Context, c GitCheckoutConfig, reg *config_vars.Registry, client *http.Client, serverURL, workdir string, getLastRollRev GitCheckoutGetLastRollRevFunc, createRoll GitCheckoutCreateRollFunc, uploadRoll GitCheckoutUploadRollFunc) (*GitCheckoutParent, error) {
	if err := reg.Register(c.Branch); err != nil {
		return nil, skerr.Wrap(err)
	}
	// Create a baseParent.
	base, err := newBaseParent(ctx, c.BaseConfig, serverURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Create the local checkout.
	co, err := git.NewCheckout(ctx, c.RepoURL, workdir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &GitCheckoutParent{
		baseParent:     base,
		branch:         c.Branch,
		co:             co,
		createRoll:     createRoll,
		getLastRollRev: getLastRollRev,
		uploadRoll:     uploadRoll,
	}, nil
}

// See documentation for Parent interface.
func (p *GitCheckoutParent) Update(ctx context.Context) (string, error) {
	if err := p.co.CleanupBranch(ctx, p.branch.String()); err != nil {
		return "", skerr.Wrap(err)
	}
	return p.getLastRollRev(ctx, p.co)
}

// See documentation for Parent interface.
func (p *GitCheckoutParent) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	// Create the roll branch.
	upstreamBranch := p.branch.String()
	if err := p.co.CleanupBranch(ctx, upstreamBranch); err != nil {
		return 0, skerr.Wrap(err)
	}
	_, _ = p.co.Git(ctx, "branch", "-D", rollBranch) // Fails if the branch does not exist.
	if _, err := p.co.Git(ctx, "checkout", "-b", rollBranch, "-t", fmt.Sprintf("origin/%s", upstreamBranch)); err != nil {
		return 0, skerr.Wrap(err)
	}
	if _, err := p.co.Git(ctx, "reset", "--hard", upstreamBranch); err != nil {
		return 0, skerr.Wrap(err)
	}

	// Generate the commit message.
	// TODO(borenet): This should probably move into parentChildRepoManager.
	commitMsg, err := p.buildCommitMsg(from, to, rolling, emails, cqExtraTrybots, nil)
	if err != nil {
		return 0, skerr.Wrap(err)
	}

	// Run the provided function to create the changes for the roll.
	hash, err := p.createRoll(ctx, p.co, from, to, rolling, commitMsg)
	if err != nil {
		return 0, skerr.Wrap(err)
	}

	// Ensure that createRoll generated at least one commit downstream of
	// p.baseCommit, and that it did not leave uncommitted changes.
	commits, err := p.co.RevList(ctx, "--ancestry-path", "--first-parent", fmt.Sprintf("%s..%s", upstreamBranch, hash))
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	if len(commits) == 0 {
		return 0, skerr.Fmt("createRoll generated no commits!")
	}
	if _, err := p.co.Git(ctx, "diff", "--quiet"); err != nil {
		return 0, skerr.Wrapf(err, "createRoll left uncommitted changes")
	}
	out, err := p.co.Git(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	if len(strings.Fields(out)) > 0 {
		return 0, skerr.Fmt("createRoll left untracked files:\n%s", out)
	}

	// Upload the CL.
	return p.uploadRoll(ctx, p.co, upstreamBranch, hash, emails, dryRun)
}

// VersionFileGetLastRollRevFunc returns a GitCheckoutGetLastRollRevFunc which
// reads the given file path from the repo and returns its full contents as the
// last-rolled revision ID.
func VersionFileGetLastRollRevFunc(path string) GitCheckoutGetLastRollRevFunc {
	return func(ctx context.Context, co *git.Checkout) (string, error) {
		contents, err := ioutil.ReadFile(filepath.Join(co.Dir(), path))
		if err != nil {
			return "", skerr.Wrap(err)
		}
		return strings.TrimSpace(string(contents)), nil
	}
}

var _ Parent = &GitCheckoutParent{}
