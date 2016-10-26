package git

/*
	Thin wrapper around a local Git repo.
*/

import "fmt"

// Repo is a struct used for managing a local git repo.
type Repo struct {
	GitDir
}

// NewRepo returns a Repo instance based in the given working directory. Uses
// any existing repo in the given directory, or clones one if necessary. Only
// creates bare clones; Repo does not maintain a checkout.
func NewRepo(repoUrl, workdir string) (*Repo, error) {
	g, err := newGitDir(repoUrl, workdir, true)
	if err != nil {
		return nil, err
	}
	return &Repo{g}, nil
}

// Update syncs the Repo from its remote.
func (r *Repo) Update() error {
	out, err := r.Git("remote", "update")
	if err != nil {
		return fmt.Errorf("Failed to update Repo: %s; output:\n%s", err, out)
	}
	return nil
}

// Checkout returns a Checkout of the Repo in the given working directory.
func (r *Repo) Checkout(workdir string) (*Checkout, error) {
	return NewCheckout(r.Dir(), workdir)
}

// TempCheckout returns a TempCheckout of the repo.
func (r *Repo) TempCheckout() (*TempCheckout, error) {
	return NewTempCheckout(r.Dir())
}
