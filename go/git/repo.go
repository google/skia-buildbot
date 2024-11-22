package git

/*
	Thin wrapper around a local Git repo.
*/

import (
	"context"
	"os"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

// Repo is used for managing a local git repo, without a working copy.
type Repo interface {
	GitDir

	// Update syncs the Repo from its remote.
	Update(ctx context.Context) error

	// Checkout returns a Checkout of the Repo in the given working directory.
	Checkout(ctx context.Context, workdir string) (CheckoutDir, error)

	// TempCheckout returns a TempCheckout of the repo.
	TempCheckout(ctx context.Context) (*TempCheckout, error)
}

// RepoDir implements Repo.
type RepoDir string

// NewRepo returns a Repo instance based in the given working directory. Uses
// any existing repo in the given directory, or clones one if necessary. Only
// creates bare clones; Repo does not maintain a checkout.
func NewRepo(ctx context.Context, repoUrl, workdir string) (RepoDir, error) {
	g, err := newGitDir(ctx, repoUrl, workdir, true)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return RepoDir(g), nil
}

// Update syncs the Repo from its remote.
func (r RepoDir) Update(ctx context.Context) error {
	gitExec, err := Executable(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	cmd := &exec.Command{
		Name:    gitExec,
		Args:    []string{"--git-dir=.", "fetch", "--force", "--all", "--prune"},
		Dir:     r.Dir(),
		Timeout: 2 * time.Minute,
	}
	out, err := exec.RunCommand(ctx, cmd)
	if err != nil {
		return skerr.Wrapf(err, "failed to update repo")
	}
	sklog.Debugf("DEBUG: output of 'git fetch':\n%s", out)
	return nil
}

// Checkout returns a Checkout of the Repo in the given working directory.
func (r RepoDir) Checkout(ctx context.Context, workdir string) (CheckoutDir, error) {
	return NewCheckout(ctx, r.Dir(), workdir)
}

// TempCheckout returns a TempCheckout of the repo.
func (r RepoDir) TempCheckout(ctx context.Context) (*TempCheckout, error) {
	return NewTempCheckout(ctx, r.Dir())
}

// Dir returns the working directory of the Repo.
func (r RepoDir) Dir() string {
	return string(r)
}

// Git runs the given git command in the Repo.
func (r RepoDir) Git(ctx context.Context, cmd ...string) (string, error) {
	git, err := Executable(ctx)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	baseCmd := []string{git, "--git-dir=."}
	return exec.RunCwd(ctx, r.Dir(), append(baseCmd, cmd...)...)
}

// Details returns a vcsinfo.LongCommit instance representing the given commit.
func (r RepoDir) Details(ctx context.Context, name string) (*vcsinfo.LongCommit, error) {
	return gitRunner_Details(ctx, r, name)
}

// RevParse runs "git rev-parse <name>" and returns the result.
func (r RepoDir) RevParse(ctx context.Context, args ...string) (string, error) {
	return gitRunner_RevParse(ctx, r, args...)
}

// RevList runs "git rev-list <name>" and returns a slice of commit hashes.
func (r RepoDir) RevList(ctx context.Context, args ...string) ([]string, error) {
	return gitRunner_RevList(ctx, r, args...)
}

// GetBranchHead returns the commit hash at the HEAD of the given branch.
func (r RepoDir) GetBranchHead(ctx context.Context, branchName string) (string, error) {
	return gitRunner_GetBranchHead(ctx, r, branchName)
}

// Branches runs "git branch" and returns a slice of Branch instances.
func (r RepoDir) Branches(ctx context.Context) ([]*Branch, error) {
	return gitRunner_Branches(ctx, r)
}

// GetFile returns the contents of the given file at the given commit.
func (r RepoDir) GetFile(ctx context.Context, fileName, commit string) (string, error) {
	return gitRunner_GetFile(ctx, r, fileName, commit)
}

// IsSubmodule returns true if the given path is submodule, ie contains gitlink.
func (r RepoDir) IsSubmodule(ctx context.Context, path, commit string) (bool, error) {
	return gitRunner_IsSubmodule(ctx, r, path, commit)
}

// ReadSubmodule returns commit hash of the given path, if the path is git
// submodule. ErrorNotFound is returned if path is not found in the git
// worktree. ErrorNotSubmodule is returned if path exists, but it's not a
// submodule.
func (r RepoDir) ReadSubmodule(ctx context.Context, path, commit string) (string, error) {
	return gitRunner_ReadSubmodule(ctx, r, path, commit)
}

// UpdateSubmodule updates git submodule of the given path to the given commit.
// If submodule doesn't exist, it returns ErrorNotFound since it doesn't have
// all necessary information to create a valid submodule (requires an entry in
// .gitmodules).
func (r RepoDir) UpdateSubmodule(ctx context.Context, path, commit string) error {
	return gitRunner_UpdateSubmodule(ctx, r, path, commit)
}

// NumCommits returns the number of commits in the repo.
func (r RepoDir) NumCommits(ctx context.Context) (int64, error) {
	return gitRunner_NumCommits(ctx, r)
}

// IsAncestor returns true iff A is an ancestor of B.
func (r RepoDir) IsAncestor(ctx context.Context, a, b string) (bool, error) {
	return gitRunner_IsAncestor(ctx, r, a, b)
}

// Version returns the Git version.
func (r RepoDir) Version(ctx context.Context) (int, int, error) {
	return gitRunner_Version(ctx)
}

// FullHash gives the full commit hash for the given ref.
func (r RepoDir) FullHash(ctx context.Context, ref string) (string, error) {
	return gitRunner_FullHash(ctx, r, ref)
}

// CatFile runs "git cat-file -p <ref>:<path>".
func (r RepoDir) CatFile(ctx context.Context, ref, path string) ([]byte, error) {
	return gitRunner_CatFile(ctx, r, ref, path)
}

// ReadDir is analogous to os.File.Readdir for a particular ref.
func (r RepoDir) ReadDir(ctx context.Context, ref, path string) ([]os.FileInfo, error) {
	return gitRunner_ReadDir(ctx, r, ref, path)
}

// GetRemotes returns a mapping of remote repo name to URL.
func (r RepoDir) GetRemotes(ctx context.Context) (map[string]string, error) {
	return gitRunner_GetRemotes(ctx, r)
}

// VFS returns a vfs.FS using Git for the given revision.
func (r RepoDir) VFS(ctx context.Context, ref string) (*FS, error) {
	return VFS(ctx, r, ref)
}

// Assert that RepoDir implements Repo.
var _ Repo = RepoDir("")
