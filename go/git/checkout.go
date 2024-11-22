package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

/*
	Utility for managing a Git checkout.
*/

// Checkout is used for managing a local git checkout.
type Checkout interface {
	GitDir

	// FetchRefFromRepo syncs the specified ref from the repo without modifying the
	// working copy.
	FetchRefFromRepo(ctx context.Context, repo, ref string) error

	// Fetch syncs refs from the remote without modifying the working copy.
	Fetch(ctx context.Context) error

	// AddRemote checks to see if a remote already exists in the checkout, if it
	// exists then the URL is matched with the repoURL. If the remote does not exist
	// then it is added.
	AddRemote(ctx context.Context, remote, repoUrl string) error

	// CleanupBranch forcibly resets all changes and checks out the given branch,
	// forcing it to match the same branch from origin. All local changes will be
	// lost.
	CleanupBranch(ctx context.Context, branch string) error

	// Cleanup forcibly resets all changes and checks out the main branch to match
	// that of the remote. All local changes will be lost.
	Cleanup(ctx context.Context) error

	// UpdateBranch syncs the Checkout from its remote. Forcibly resets and checks
	// out the given branch, forcing it to match the same branch from origin. All
	// local changes will be lost. Equivalent to c.Fetch() + c.CleanupBranch().
	UpdateBranch(ctx context.Context, branch string) error

	// Update syncs the Checkout from its remote. Forcibly resets and checks out
	// the main branch to match the remote. All local changes will be lost.
	// Equivalent to c.Fetch() + c.Cleanup().
	Update(ctx context.Context) error

	// IsDirty returns true if the Checkout is dirty, ie. any of the following are
	// true:
	// 1. There are unstaged changes.
	// 2. There are untracked files (not including .gitignore'd files).
	// 3. HEAD is not an ancestor of origin/main.
	//
	// Also returns the output of "git status", for human consumption if desired.
	IsDirty(ctx context.Context) (bool, string, error)
}

// CheckoutDir implements Checkout.
type CheckoutDir string

// NewCheckout returns a Checkout instance based in the given working directory.
// Uses any existing checkout in the given directory, or clones one if
// necessary. In general, servers should use Repo instead of Checkout unless it
// is absolutely necessary to have a working copy.
func NewCheckout(ctx context.Context, repoUrl, workdir string) (CheckoutDir, error) {
	g, err := newGitDir(ctx, repoUrl, workdir, false)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return CheckoutDir(g), nil
}

// Dir returns the working directory of the GitDir.
func (c CheckoutDir) Dir() string {
	return string(c)
}

// Git runs the given git command in the Checkout.
func (c CheckoutDir) Git(ctx context.Context, cmd ...string) (string, error) {
	git, err := Executable(ctx)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	baseCmd := []string{git}
	return exec.RunCwd(ctx, c.Dir(), append(baseCmd, cmd...)...)
}

// FetchRefFromRepo syncs the specified ref from the repo without modifying the
// working copy.
func (c CheckoutDir) FetchRefFromRepo(ctx context.Context, repo, ref string) error {
	_, err := c.Git(ctx, "fetch", repo, ref)
	return skerr.Wrap(err)
}

// Fetch syncs refs from the remote without modifying the working copy.
func (c CheckoutDir) Fetch(ctx context.Context) error {
	_, err := c.Git(ctx, "fetch", "--prune", DefaultRemote)
	return skerr.Wrap(err)
}

// AddRemote checks to see if a remote already exists in the checkout, if it
// exists then the URL is matched with the repoURL. If the remote does not exist
// then it is added.
func (c CheckoutDir) AddRemote(ctx context.Context, remote, repoUrl string) error {
	// Check to see whether there is an upstream yet.
	remoteOutput, err := c.Git(ctx, "remote", "get-url", remote)
	if err != nil {
		if strings.Contains(err.Error(), "No such remote") {
			if _, err := c.Git(ctx, "remote", "add", remote, repoUrl); err != nil {
				return skerr.Wrap(err)
			}
		} else {
			return skerr.Wrap(err)
		}
	} else {
		// Remote already exists. Make sure that the URLs match.
		if strings.TrimSpace(remoteOutput) != repoUrl {
			return skerr.Fmt("%s points to %s instead of %s", remote, strings.TrimSpace(remoteOutput), repoUrl)
		}
	}
	return nil
}

// CleanupBranch forcibly resets all changes and checks out the given branch,
// forcing it to match the same branch from origin. All local changes will be
// lost.
func (c CheckoutDir) CleanupBranch(ctx context.Context, branch string) error {
	if _, err := c.Git(ctx, "checkout", branch, "-f"); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := c.Git(ctx, "clean", "-d", "-f"); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := c.Git(ctx, "reset", "--hard", fmt.Sprintf("origin/%s", branch)); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Cleanup forcibly resets all changes and checks out the main branch to match
// that of the remote. All local changes will be lost.
func (c CheckoutDir) Cleanup(ctx context.Context) error {
	return c.CleanupBranch(ctx, MainBranch)
}

// UpdateBranch syncs the Checkout from its remote. Forcibly resets and checks
// out the given branch, forcing it to match the same branch from origin. All
// local changes will be lost. Equivalent to c.Fetch() + c.CleanupBranch().
func (c CheckoutDir) UpdateBranch(ctx context.Context, branch string) error {
	if err := c.Fetch(ctx); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.CleanupBranch(ctx, branch); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Update syncs the Checkout from its remote. Forcibly resets and checks out
// the main branch to match the remote. All local changes will be lost.
// Equivalent to c.Fetch() + c.Cleanup().
func (c CheckoutDir) Update(ctx context.Context) error {
	return c.UpdateBranch(ctx, MainBranch)
}

// IsDirty returns true if the Checkout is dirty, ie. any of the following are
// true:
// 1. There are unstaged changes.
// 2. There are untracked files (not including .gitignore'd files).
// 3. HEAD is not an ancestor of origin/main.
//
// Also returns the output of "git status", for human consumption if desired.
func (c CheckoutDir) IsDirty(ctx context.Context) (bool, string, error) {
	status, err := c.Git(ctx, "status")
	if err != nil {
		return false, "", skerr.Wrap(err)
	}
	if _, err := c.Git(ctx, "update-index", "--refresh"); err != nil {
		return true, status, nil
	}
	if _, err := c.Git(ctx, "diff-index", "--quiet", "HEAD", "--"); err != nil {
		return true, status, nil
	}
	output, err := c.Git(ctx, "ls-files", "--other", "--directory", "--exclude-standard")
	if err != nil {
		return false, status, skerr.Wrap(err)
	}
	output = strings.TrimSpace(output)
	if output != "" {
		return true, status, nil
	}
	if anc, err := c.IsAncestor(ctx, "HEAD", DefaultRemoteBranch); err != nil {
		return false, status, skerr.Wrap(err)
	} else if !anc {
		return true, status, nil
	}
	return false, status, nil
}

// Details returns a vcsinfo.LongCommit instance representing the given commit.
func (c CheckoutDir) Details(ctx context.Context, name string) (*vcsinfo.LongCommit, error) {
	return gitRunner_Details(ctx, c, name)
}

// RevParse runs "git rev-parse <name>" and returns the result.
func (c CheckoutDir) RevParse(ctx context.Context, args ...string) (string, error) {
	return gitRunner_RevParse(ctx, c, args...)
}

// RevList runs "git rev-list <name>" and returns a slice of commit hashes.
func (c CheckoutDir) RevList(ctx context.Context, args ...string) ([]string, error) {
	return gitRunner_RevList(ctx, c, args...)
}

// GetBranchHead returns the commit hash at the HEAD of the given branch.
func (c CheckoutDir) GetBranchHead(ctx context.Context, branchName string) (string, error) {
	return gitRunner_GetBranchHead(ctx, c, branchName)
}

// Branches runs "git branch" and returns a slice of Branch instances.
func (c CheckoutDir) Branches(ctx context.Context) ([]*Branch, error) {
	return gitRunner_Branches(ctx, c)
}

// GetFile returns the contents of the given file at the given commit.
func (c CheckoutDir) GetFile(ctx context.Context, fileName, commit string) (string, error) {
	return gitRunner_GetFile(ctx, c, fileName, commit)
}

// IsSubmodule returns true if the given path is submodule, ie contains gitlink.
func (c CheckoutDir) IsSubmodule(ctx context.Context, path, commit string) (bool, error) {
	return gitRunner_IsSubmodule(ctx, c, path, commit)
}

// ReadSubmodule returns commit hash of the given path, if the path is git
// submodule. ErrorNotFound is returned if path is not found in the git
// worktree. ErrorNotSubmodule is returned if path exists, but it's not a
// submodule.
func (c CheckoutDir) ReadSubmodule(ctx context.Context, path, commit string) (string, error) {
	return gitRunner_ReadSubmodule(ctx, c, path, commit)
}

// UpdateSubmodule updates git submodule of the given path to the given commit.
// If submodule doesn't exist, it returns ErrorNotFound since it doesn't have
// all necessary information to create a valid submodule (requires an entry in
// .gitmodules).
func (c CheckoutDir) UpdateSubmodule(ctx context.Context, path, commit string) error {
	return gitRunner_UpdateSubmodule(ctx, c, path, commit)
}

// NumCommits returns the number of commits in the repo.
func (c CheckoutDir) NumCommits(ctx context.Context) (int64, error) {
	return gitRunner_NumCommits(ctx, c)
}

// IsAncestor returns true iff A is an ancestor of B.
func (c CheckoutDir) IsAncestor(ctx context.Context, a, b string) (bool, error) {
	return gitRunner_IsAncestor(ctx, c, a, b)
}

// Version returns the Git version.
func (c CheckoutDir) Version(ctx context.Context) (int, int, error) {
	return gitRunner_Version(ctx)
}

// FullHash gives the full commit hash for the given ref.
func (c CheckoutDir) FullHash(ctx context.Context, ref string) (string, error) {
	return gitRunner_FullHash(ctx, c, ref)
}

// CatFile runs "git cat-file -p <ref>:<path>".
func (c CheckoutDir) CatFile(ctx context.Context, ref, path string) ([]byte, error) {
	return gitRunner_CatFile(ctx, c, ref, path)
}

// ReadDir is analogous to os.File.Readdir for a particular ref.
func (c CheckoutDir) ReadDir(ctx context.Context, ref, path string) ([]os.FileInfo, error) {
	return gitRunner_ReadDir(ctx, c, ref, path)
}

// GetRemotes returns a mapping of remote repo name to URL.
func (c CheckoutDir) GetRemotes(ctx context.Context) (map[string]string, error) {
	return gitRunner_GetRemotes(ctx, c)
}

// VFS returns a vfs.FS using Git for the given revision.
func (c CheckoutDir) VFS(ctx context.Context, ref string) (*FS, error) {
	return VFS(ctx, c, ref)
}

// TempCheckout is a temporary Git Checkout.
type TempCheckout struct {
	Checkout
}

// NewTempCheckout returns a TempCheckout instance. Creates a temporary
// directory and then clones the repoUrl into a subdirectory, based on default
// "git clone" behavior.
func NewTempCheckout(ctx context.Context, repoUrl string) (*TempCheckout, error) {
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	c, err := NewCheckout(ctx, repoUrl, tmpDir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &TempCheckout{
		Checkout: c,
	}, nil
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

// Assert that CheckoutDir implements Checkout.
var _ Checkout = CheckoutDir("")
