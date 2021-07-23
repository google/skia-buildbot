package git

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
)

/*
	Utility for managing a Git checkout.
*/

// Checkout is a struct used for managing a local git checkout.
type Checkout struct {
	GitDir
}

// NewCheckout returns a Checkout instance based in the given working directory.
// Uses any existing checkout in the given directory, or clones one if
// necessary. In general, servers should use Repo instead of Checkout unless it
// is absolutely necessary to have a working copy.
func NewCheckout(ctx context.Context, repoUrl, workdir string) (*Checkout, error) {
	g, err := newGitDir(ctx, repoUrl, workdir, false)
	if err != nil {
		return nil, err
	}
	return &Checkout{g}, nil
}

// FetchRefFromRepo syncs the specified ref from the repo without modifying the
// working copy.
func (c *Checkout) FetchRefFromRepo(ctx context.Context, repo, ref string) error {
	_, err := c.Git(ctx, "fetch", repo, ref)
	return err
}

// Fetch syncs refs from the remote without modifying the working copy.
func (c *Checkout) Fetch(ctx context.Context) error {
	_, err := c.Git(ctx, "fetch", "--prune", DefaultRemote)
	return err
}

// AddRemote checks to see if a remote already exists in the checkout, if it
// exists then the URL is matched with the repoURL. If the remote does not exist
// then it is added.
func (c *Checkout) AddRemote(ctx context.Context, remote, repoUrl string) error {
	// Check to see whether there is an upstream yet.
	remoteOutput, err := c.Git(ctx, "remote", "get-url", remote)
	if err != nil {
		if strings.Contains(err.Error(), "No such remote") {
			if _, err := c.Git(ctx, "remote", "add", remote, repoUrl); err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		// Remote already exists. Make sure that the URLs match.
		if strings.TrimSpace(remoteOutput) != repoUrl {
			return fmt.Errorf("%s points to %s instead of %s", remote, strings.TrimSpace(remoteOutput), repoUrl)
		}
	}
	return nil
}

// CleanupBranch forcibly resets all changes and checks out the given branch,
// forcing it to match the same branch from origin. All local changes will be
// lost.
func (c *Checkout) CleanupBranch(ctx context.Context, branch string) error {
	if _, err := c.Git(ctx, "checkout", branch, "-f"); err != nil {
		return err
	}
	if _, err := c.Git(ctx, "clean", "-d", "-f"); err != nil {
		return err
	}
	if _, err := c.Git(ctx, "reset", "--hard", fmt.Sprintf("origin/%s", branch)); err != nil {
		return err
	}
	return nil
}

// Cleanup forcibly resets all changes and checks out the main branch to match
// that of the remote. All local changes will be lost.
func (c *Checkout) Cleanup(ctx context.Context) error {
	return c.CleanupBranch(ctx, MasterBranch)
}

// UpdateBranch syncs the Checkout from its remote. Forcibly resets and checks
// out the given branch, forcing it to match the same branch from origin. All
// local changes will be lost. Equivalent to c.Fetch() + c.CleanupBranch().
func (c *Checkout) UpdateBranch(ctx context.Context, branch string) error {
	if err := c.Fetch(ctx); err != nil {
		return err
	}
	if err := c.CleanupBranch(ctx, branch); err != nil {
		return err
	}
	return nil
}

// Update syncs the Checkout from its remote. Forcibly resets and checks out
// the main branch to match the remote. All local changes will be lost.
// Equivalent to c.Fetch() + c.Cleanup().
func (c *Checkout) Update(ctx context.Context) error {
	return c.UpdateBranch(ctx, MasterBranch)
}

// TempCheckout is a temporary Git Checkout.
type TempCheckout Checkout

// NewTempCheckout returns a TempCheckout instance. Creates a temporary
// directory and then clones the repoUrl into a subdirectory, based on default
// "git clone" behavior.
func NewTempCheckout(ctx context.Context, repoUrl string) (*TempCheckout, error) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, err
	}
	c, err := NewCheckout(ctx, repoUrl, tmpDir)
	if err != nil {
		return nil, err
	}
	return (*TempCheckout)(c), nil
}

// Delete removes the TempCheckout's working directory.
func (c *TempCheckout) Delete() {
	deleteDir := filepath.Dir(c.Dir())
	if err := os.RemoveAll(deleteDir); err != nil {
		// Some processes (eg. gclient) leave files that are owned by us but not
		// writeable. Make everything writeable before attempting to delete. We
		// do this only after the first RemoveAll attempt, in case there are a
		// large number of files.
		if _, err2 := exec.RunCwd(context.TODO(), ".", "chmod", "-R", "+w", deleteDir); err2 != nil {
			sklog.Errorf("Failed to remove git.TempCheckout with: %s; and failed to make writeable with: %s", err, err2)
			return
		}
		if err := os.RemoveAll(deleteDir); err != nil {
			sklog.Errorf("Failed to remove git.TempCheckout despite making writeable: %s", err)
		}
	}
}
