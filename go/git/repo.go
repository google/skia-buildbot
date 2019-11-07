package git

/*
	Thin wrapper around a local Git repo.
*/

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// Repo is a struct used for managing a local git repo.
type Repo struct {
	GitDir
}

// NewRepo returns a Repo instance based in the given working directory. Uses
// any existing repo in the given directory, or clones one if necessary. Only
// creates bare clones; Repo does not maintain a checkout.
func NewRepo(ctx context.Context, repoUrl, workdir string) (*Repo, error) {
	g, err := newGitDir(ctx, repoUrl, workdir, true)
	if err != nil {
		return nil, err
	}
	return &Repo{g}, nil
}

// Update syncs the Repo from its remote.
func (r *Repo) Update(ctx context.Context) error {
	gitExec, err := Executable(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	cmd := &exec.Command{
		Name:    gitExec,
		Args:    []string{"fetch", "--force", "--all", "--prune"},
		Dir:     r.Dir(),
		Timeout: 2 * time.Minute,
	}
	out, err := exec.RunCommand(ctx, cmd)
	if err != nil {
		return fmt.Errorf("Failed to update Repo: %s; output:\n%s", err, out)
	}
	sklog.Debugf("DEBUG: output of 'git fetch':\n%s", out)
	return nil
}

// Checkout returns a Checkout of the Repo in the given working directory.
func (r *Repo) Checkout(ctx context.Context, workdir string) (*Checkout, error) {
	return NewCheckout(ctx, r.Dir(), workdir)
}

// TempCheckout returns a TempCheckout of the repo.
func (r *Repo) TempCheckout(ctx context.Context) (*TempCheckout, error) {
	return NewTempCheckout(ctx, r.Dir())
}
