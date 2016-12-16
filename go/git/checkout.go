package git

import (
	"io/ioutil"
	"os"
	"path"

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
func NewCheckout(repoUrl, workdir string) (*Checkout, error) {
	g, err := newGitDir(repoUrl, workdir, false)
	if err != nil {
		return nil, err
	}
	return &Checkout{g}, nil
}

// Fetch syncs refs from the remote without modifying the working copy.
func (c *Checkout) Fetch() error {
	_, err := c.Git("fetch", "origin")
	return err
}

// Cleanup forcibly resets all changes and checks out the master branch at
// origin/master. All local changes will be lost.
func (c *Checkout) Cleanup() error {
	if _, err := c.Git("reset", "--hard", "HEAD"); err != nil {
		return err
	}
	if _, err := c.Git("clean", "-d", "-f"); err != nil {
		return err
	}
	if _, err := c.Git("checkout", "master", "-f"); err != nil {
		return err
	}
	if _, err := c.Git("reset", "--hard", "origin/master"); err != nil {
		return err
	}
	return nil
}

// Update syncs the Checkout from its remote. Forcibly resets and checks out
// the master branch at origin/master. All local changes will be lost.
// Equivalent to c.Fetch() + c.Cleanup().
func (c *Checkout) Update() error {
	if err := c.Fetch(); err != nil {
		return err
	}
	if err := c.Cleanup(); err != nil {
		return err
	}
	return nil
}

// TempCheckout is a temporary Git Checkout.
type TempCheckout Checkout

// NewTempCheckout returns a TempCheckout instance.
func NewTempCheckout(repoUrl string) (*TempCheckout, error) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, err
	}
	c, err := NewCheckout(repoUrl, tmpDir)
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
