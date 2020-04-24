package git_common

import (
	"context"
	"os"
	"path/filepath"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

// GitCheckoutConfig provides configuration for a Checkout.
type GitCheckoutConfig struct {
	Branch      *config_vars.Template `json:"branch"`
	RepoURL     string                `json:"repoURL"`
	RevLinkTmpl string                `json:"revLinkTmpl"`
}

// See documentation for util.Validator interface.
func (c GitCheckoutConfig) Validate() error {
	if c.Branch == nil {
		return skerr.Fmt("Branch is required")
	}
	if err := c.Branch.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.RepoURL == "" {
		return skerr.Fmt("RepoURL is required")
	}
	return nil
}

// Checkout provides common functionality for git checkouts.
type Checkout struct {
	*git.Checkout
	Branch      *config_vars.Template
	RepoURL     string
	RevLinkTmpl string
}

// NewCheckout returns a Checkout instance.
func NewCheckout(ctx context.Context, c GitCheckoutConfig, reg *config_vars.Registry, workdir, userName, userEmail string, co *git.Checkout) (*Checkout, error) {
	// Clean up any lockfiles, in case the process was interrupted.
	if err := git.DeleteLockFiles(ctx, workdir); err != nil {
		return nil, skerr.Wrap(err)
	}
	// Register the configured branch template.
	if err := reg.Register(c.Branch); err != nil {
		return nil, skerr.Wrap(err)
	}
	// Create the local checkout.
	if co == nil {
		var err error
		co, err = git.NewCheckout(ctx, c.RepoURL, workdir)
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
		Checkout:    co,
		Branch:      c.Branch,
		RepoURL:     c.RepoURL,
		RevLinkTmpl: c.RevLinkTmpl,
	}, nil
}

// See documentation for child.Child interface.
func (c *Checkout) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	details, err := c.Details(ctx, id)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return revision.FromLongCommit(c.RevLinkTmpl, details), nil
}

// See documentation for child.Child interface.
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

// LogFirstParent returns a slice of revision.Revision instances in the given range.
func (c *Checkout) LogFirstParent(ctx context.Context, from, to *revision.Revision) ([]*revision.Revision, error) {
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
	co := &git.Checkout{GitDir: git.GitDir(dest)}
	if err := co.Fetch(ctx); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := co.Git(ctx, "reset", "--hard", rev.Id); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}
