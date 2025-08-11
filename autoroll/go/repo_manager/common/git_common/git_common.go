package git_common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	// RollBranch is the git branch which is used to create rolls.
	RollBranch = "roll_branch"
)

// Checkout provides common functionality for git checkouts.
type Checkout struct {
	git.Checkout
	Branch            *config_vars.Template
	defaultBugProject string
	Dependencies      []*config.VersionFileConfig
	RepoURL           string
	RevLinkTmpl       string
}

// NewCheckout returns a Checkout instance.
func NewCheckout(ctx context.Context, c *config.GitCheckoutConfig, reg *config_vars.Registry, workdir, userName, userEmail string, co git.Checkout) (*Checkout, error) {
	// Clean up any lockfiles, in case the process was interrupted.
	if err := git.DeleteLockFiles(ctx, workdir); err != nil {
		return nil, skerr.Wrap(err)
	}
	// Register the configured branch template.
	branch, err := config_vars.NewTemplate(c.Branch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := reg.Register(branch); err != nil {
		return nil, skerr.Wrap(err)
	}
	// Create the local checkout.
	if co == nil {
		var err error
		co, err = git.NewCheckout(ctx, c.RepoUrl, workdir)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	// Set the git user name and email.
	if _, err := co.Git(ctx, "config", "--local", "user.name", userName); err != nil {
		return nil, skerr.Wrap(err)
	}
	if _, err := co.Git(ctx, "config", "--local", "user.email", userEmail); err != nil {
		return nil, skerr.Wrap(err)
	}
	return &Checkout{
		Checkout:          co,
		Branch:            branch,
		defaultBugProject: c.DefaultBugProject,
		Dependencies:      c.Dependencies,
		RepoURL:           c.RepoUrl,
		RevLinkTmpl:       c.RevLinkTmpl,
	}, nil
}

// GetRevision implements Child.
func (c *Checkout) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	details, err := c.Details(ctx, id)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rev := revision.FromLongCommit(c.RevLinkTmpl, c.defaultBugProject, details)
	if len(c.Dependencies) > 0 {
		deps, err := version_file_common.GetPinnedRevs(ctx, c.Dependencies, func(ctx context.Context, path string) (string, error) {
			return c.GetFile(ctx, path, rev.Id)
		})
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rev.Dependencies = deps
	}
	return rev, nil
}

// Download implements Child.
func (c *Checkout) Download(ctx context.Context, rev *revision.Revision, dest string) error {
	return Clone(ctx, c.RepoURL, dest, rev)
}

// Update resolves the configured branch template, updates the Checkout to the
// newest Revision on the resulting branch and returns both the revision and
// resolved branch name.
func (c *Checkout) Update(ctx context.Context) (*revision.Revision, string, error) {
	branch := c.Branch.String()
	if err := c.UpdateBranch(ctx, branch); err != nil {
		return nil, "", skerr.Wrap(err)
	}
	tipRev, err := c.GetRevision(ctx, "HEAD")
	if err != nil {
		return nil, "", skerr.Wrap(err)
	}
	return tipRev, branch, nil
}

// LogRevisions implements Child.
func (c *Checkout) LogRevisions(ctx context.Context, from, to *revision.Revision) ([]*revision.Revision, error) {
	hashes, err := c.RevList(ctx, "--first-parent", git.LogFromTo(from.Id, to.Id))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	revs := make([]*revision.Revision, 0, len(hashes))
	for _, hash := range hashes {
		rev, err := c.GetRevision(ctx, hash)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		revs = append(revs, rev)
	}
	return revs, nil
}

// ApplyExternalChangeFunc applies the specified ExternalChangeId in whichever
// way makes sense for the implementation. Example: git_checkout_github uses
// the ExternalChangeId as a Github PR and cherry-picks the PR patch locally.
type ApplyExternalChangeFunc func(context.Context, git.Checkout, string) error

// CreateRollFunc generates commit(s) in the local Git checkout to
// be used in the next roll and returns the hash of the commit to be uploaded.
// GitCheckoutParent handles creation of the roll branch.
type CreateRollFunc func(context.Context, git.Checkout, *revision.Revision, *revision.Revision, []*revision.Revision, string) (string, error)

// UploadRollFunc uploads a CL using the given commit hash and
// returns its ID.
type UploadRollFunc func(context.Context, git.Checkout, string, string, []string, bool, bool, string) (int64, error)

// CreateNewRoll uploads a new roll using the given createRoll and uploadRoll
// functions.
// See documentation for the Parent interface for more details.
func (c *Checkout) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, dryRun, canary bool, commitMsg string, createRoll CreateRollFunc, uploadRoll UploadRollFunc) (int64, error) {
	// Create the roll branch.
	_, upstreamBranch, err := c.Update(ctx)
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	_, _ = c.Git(ctx, "branch", "-D", RollBranch) // Fails if the branch does not exist.
	if _, err := c.Git(ctx, "checkout", "-b", RollBranch, "-t", fmt.Sprintf("origin/%s", upstreamBranch)); err != nil {
		return 0, skerr.Wrap(err)
	}
	if _, err := c.Git(ctx, "reset", "--hard", upstreamBranch); err != nil {
		return 0, skerr.Wrap(err)
	}

	// Run the provided function to create the changes for the roll.
	hash, err := createRoll(ctx, c.Checkout, from, to, rolling, commitMsg)
	if err != nil {
		return 0, skerr.Wrap(err)
	}

	// Ensure that createRoll generated at least one commit downstream of
	// p.baseCommit, and that it did not leave uncommitted changes.
	commits, err := c.RevList(ctx, "--ancestry-path", "--first-parent", fmt.Sprintf("%s..%s", upstreamBranch, hash))
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	if len(commits) == 0 {
		return 0, skerr.Fmt("createRoll generated no commits!")
	}
	if _, err := c.Git(ctx, "diff", "--quiet"); err != nil {
		return 0, skerr.Wrapf(err, "createRoll left uncommitted changes")
	}
	out, err := c.Git(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	if len(strings.Fields(out)) > 0 {
		return 0, skerr.Fmt("createRoll left untracked files:\n%s", out)
	}

	// Upload the CL.
	return uploadRoll(ctx, c.Checkout, upstreamBranch, hash, emails, dryRun, canary, commitMsg)
}

// Clone clones the given repo into the given destination and syncs it to the
// given Revision.
func Clone(ctx context.Context, repoUrl, dest string, rev *revision.Revision) error {
	// If the checkout does not already exist in dest, create it.
	gitDir := filepath.Join(dest, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if err := git.Clone(ctx, repoUrl, dest, false); err != nil {
			return skerr.Wrap(err)
		}
	}

	// Fetch and reset to the given revision.
	co := git.CheckoutDir(dest)
	if err := co.Fetch(ctx); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := co.Git(ctx, "reset", "--hard", rev.Id); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// RepoInterface is used by GetNotSubmittedReason to interact with a git repo.
type RepoInterface interface {
	// IsAncestor which returns true iff A is an ancestor of B.
	IsAncestor(ctx context.Context, a, b string) (bool, error)

	// ResolveRef resolves the given ref to a commit hash.
	ResolveRef(ctx context.Context, ref string) (string, error)
}

// Assert that git.Checkout and gitiles.GitilesRepo implement RepoInterface.
var _ RepoInterface = git.Checkout(nil)
var _ RepoInterface = gitiles.GitilesRepo(nil)

// GetNotSubmittedReason returns the reason we think the given revision ID is
// not submitted, or the empty string if we think it is.
func GetNotSubmittedReason(ctx context.Context, repo RepoInterface, revID string, targetBranch string) (string, error) {
	// Check whether the commit is the head of or an ancestor of the target
	// branch or other known branches.
	knownBranches := []string{
		targetBranch,
		git.MainBranch,
		git.MasterBranch,
	}
	for _, branch := range knownBranches {
		// Is the revision the branch head?
		branchHead, err := repo.ResolveRef(ctx, branch)
		if err != nil {
			// The branch may not exist in this repo. Ignore and move on.
			sklog.Warningf("failed to resolve branch %q: %s", branch, err)
			continue
		}
		if branchHead == revID {
			return "", nil
		}

		// Is the revision an ancestor of the branch head?
		if ok, err := repo.IsAncestor(ctx, revID, branch); err != nil {
			return "", skerr.Wrap(err)
		} else if ok {
			return "", nil
		}
	}
	return fmt.Sprintf("%s is not an ancestor of any of %v", revID, knownBranches), nil
}
