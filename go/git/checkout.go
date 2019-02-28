package git

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

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
	_, err := c.Git(ctx, "fetch", "--prune", "origin")
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

// Cleanup forcibly resets all changes and checks out the master branch at
// origin/master. All local changes will be lost.
func (c *Checkout) Cleanup(ctx context.Context) error {
	if _, err := c.Git(ctx, "reset", "--hard", "HEAD"); err != nil {
		return err
	}
	if _, err := c.Git(ctx, "clean", "-d", "-f"); err != nil {
		return err
	}
	if _, err := c.Git(ctx, "checkout", "master", "-f"); err != nil {
		return err
	}
	if _, err := c.Git(ctx, "reset", "--hard", "origin/master"); err != nil {
		return err
	}
	return nil
}

// Update syncs the Checkout from its remote. Forcibly resets and checks out
// the master branch at origin/master. All local changes will be lost.
// Equivalent to c.Fetch() + c.Cleanup().
func (c *Checkout) Update(ctx context.Context) error {
	if err := c.Fetch(ctx); err != nil {
		return err
	}
	if err := c.Cleanup(ctx); err != nil {
		return err
	}
	return nil
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
	if err := os.RemoveAll(path.Dir(c.Dir())); err != nil {
		sklog.Errorf("Failed to remove git.TempCheckout: %s", err)
	}
}
