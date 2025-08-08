package git

/*
	Common utils used by Repo and Checkout.
*/

import (
	"context"
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vcsinfo"
)

const gitlinkMode = "160000"

var ErrorNotSubmodule = skerr.Fmt("not a submodule")
var ErrorNotFound = skerr.Fmt("file not found")
var lsTreeRe = regexp.MustCompile(`(\d+) (\w+) ([[:xdigit:]]+)\t(.*)`)

// Branch describes a Git branch.
type Branch struct {
	// The human-readable name of the branch.
	Name string `json:"name"`

	// The commit hash pointed to by this branch.
	Head string `json:"head"`
}

// BranchList is a slice of Branch objects which implements sort.Interface.
type BranchList []*Branch

func (bl BranchList) Len() int           { return len(bl) }
func (bl BranchList) Less(a, b int) bool { return bl[a].Name < bl[b].Name }
func (bl BranchList) Swap(a, b int)      { bl[a], bl[b] = bl[b], bl[a] }

// GitDir is a directory in which one may run Git commands.
type GitDir interface {
	// Dir returns the working directory of the GitDir.
	Dir() string

	// Git runs the given git command in the GitDir.
	Git(ctx context.Context, cmd ...string) (string, error)

	// Details returns a vcsinfo.LongCommit instance representing the given commit.
	Details(ctx context.Context, name string) (*vcsinfo.LongCommit, error)

	// RevParse runs "git rev-parse <name>" and returns the result.
	RevParse(ctx context.Context, args ...string) (string, error)

	// RevList runs "git rev-list <name>" and returns a slice of commit hashes.
	RevList(ctx context.Context, args ...string) ([]string, error)

	// ResolveRef resolves the given ref to a commit hash.
	ResolveRef(ctx context.Context, ref string) (string, error)

	// Branches runs "git branch" and returns a slice of Branch instances.
	Branches(ctx context.Context) ([]*Branch, error)

	// GetFile returns the contents of the given file at the given commit.
	GetFile(ctx context.Context, fileName, commit string) (string, error)

	// IsSubmodule returns true if the given path is submodule, ie contains gitlink.
	IsSubmodule(ctx context.Context, path, commit string) (bool, error)

	// ReadSubmodule returns commit hash of the given path, if the path is git
	// submodule. ErrorNotFound is returned if path is not found in the git
	// worktree. ErrorNotSubmodule is returned if path exists, but it's not a
	// submodule.
	ReadSubmodule(ctx context.Context, path, commit string) (string, error)

	// UpdateSubmodule updates git submodule of the given path to the given commit.
	// If submodule doesn't exist, it returns ErrorNotFound since it doesn't have
	// all necessary information to create a valid submodule (requires an entry in
	// .gitmodules).
	UpdateSubmodule(ctx context.Context, path, commit string) error

	// NumCommits returns the number of commits in the repo.
	NumCommits(ctx context.Context) (int64, error)

	// IsAncestor returns true iff A is an ancestor of B.
	IsAncestor(ctx context.Context, a, b string) (bool, error)

	// Version returns the Git version.
	Version(ctx context.Context) (int, int, error)

	// FullHash gives the full commit hash for the given ref.
	FullHash(ctx context.Context, ref string) (string, error)

	// CatFile runs "git cat-file -p <ref>:<path>".
	CatFile(ctx context.Context, ref, path string) ([]byte, error)

	// ReadDir is analogous to os.File.Readdir for a particular ref.
	ReadDir(ctx context.Context, ref, path string) ([]os.FileInfo, error)

	// GetRemotes returns a mapping of remote repo name to URL.
	GetRemotes(ctx context.Context) (map[string]string, error)

	// VFS returns a vfs.FS using Git for the given revision.
	VFS(ctx context.Context, ref string) (*FS, error)
}

type gitRunner interface {
	Git(ctx context.Context, cmd ...string) (string, error)
}

// newGitDir creates a GitDir instance based in the given directory.
func newGitDir(ctx context.Context, repoUrl, workdir string, mirror bool) (string, error) {
	dest := path.Join(workdir, strings.TrimSuffix(path.Base(repoUrl), ".git"))
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			if err := Clone(ctx, repoUrl, dest, mirror); err != nil {
				return "", skerr.Wrap(err)
			}
		} else {
			return "", skerr.Wrapf(err, "there is a problem with the git directory")
		}
	}
	return dest, nil
}

// Details returns a vcsinfo.LongCommit instance representing the given commit.
func gitRunner_Details(ctx context.Context, g gitRunner, name string) (*vcsinfo.LongCommit, error) {
	output, err := g.Git(ctx, "log", "-n", "1", "--format=format:%H%n%P%n%an%x20(%ae)%n%s%n%ct%n%b", name)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	lines := strings.SplitN(output, "\n", 6)
	if len(lines) != 6 {
		return nil, skerr.Fmt("failed to parse output of 'git log'")
	}
	var parents []string
	if lines[1] != "" {
		parents = strings.Split(lines[1], " ")
	}
	ts, err := strconv.ParseInt(lines[4], 10, 64)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    lines[0],
			Author:  lines[2],
			Subject: lines[3],
		},
		Parents:   parents,
		Body:      strings.TrimRight(lines[5], "\n"),
		Timestamp: time.Unix(ts, 0).UTC(),
	}, nil
}

// RevParse runs "git rev-parse <name>" and returns the result.
func gitRunner_RevParse(ctx context.Context, g gitRunner, args ...string) (string, error) {
	out, err := g.Git(ctx, append([]string{"rev-parse"}, args...)...)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	// Ensure that we got a single, 40-character commit hash.
	split := strings.Fields(out)
	if len(split) != 1 {
		return "", skerr.Fmt("unable to parse commit hash from output: %s", out)
	}
	if len(split[0]) != 40 {
		return "", skerr.Fmt("rev-parse returned invalid commit hash: %s", out)
	}
	return split[0], nil
}

// RevList runs "git rev-list <name>" and returns a slice of commit hashes.
func gitRunner_RevList(ctx context.Context, g gitRunner, args ...string) ([]string, error) {
	out, err := g.Git(ctx, append([]string{"rev-list"}, args...)...)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return strings.Fields(out), nil
}

// ResolveRef resolves the given ref to a commit hash.
func gitRunner_ResolveRef(ctx context.Context, g gitRunner, branchName string) (string, error) {
	return gitRunner_RevParse(ctx, g, "--verify", fmt.Sprintf("refs/heads/%s^{commit}", branchName))
}

// Branches runs "git branch" and returns a slice of Branch instances.
func gitRunner_Branches(ctx context.Context, g gitRunner) ([]*Branch, error) {
	out, err := g.Git(ctx, "branch")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	branchNames := strings.Fields(out)
	branches := make([]*Branch, 0, len(branchNames))
	for _, name := range branchNames {
		if name == "*" {
			continue
		}
		head, err := gitRunner_ResolveRef(ctx, g, name)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		branches = append(branches, &Branch{
			Head: head,
			Name: name,
		})
	}
	return branches, nil
}

// GetFile returns the contents of the given file at the given commit.
func gitRunner_GetFile(ctx context.Context, g gitRunner, fileName, commit string) (string, error) {
	return g.Git(ctx, "show", commit+":"+fileName)
}

// IsSubmodule returns true if the given path is submodule, ie contains gitlink.
func gitRunner_IsSubmodule(ctx context.Context, g gitRunner, path, commit string) (bool, error) {
	_, err := gitRunner_ReadSubmodule(ctx, g, path, commit)
	switch skerr.Unwrap(err) {
	case ErrorNotSubmodule:
		return false, nil
	case nil:
		return true, nil
	default:
		return false, skerr.Wrap(err)
	}
}

// ReadSubmodule returns commit hash of the given path, if the path is git
// submodule. ErrorNotFound is returned if path is not found in the git
// worktree. ErrorNotSubmodule is returned if path exists, but it's not a
// submodule.
func gitRunner_ReadSubmodule(ctx context.Context, g gitRunner, path, commit string) (string, error) {
	// Detect if we are dealing with submodules or regular files.
	// Expected output for submodules:
	// <mode> SP <type> SP <object> TAB <file>
	out, err := g.Git(ctx, "ls-tree", commit, "--", path)
	if err != nil {
		return "", skerr.Wrap(err)
	}

	matches := lsTreeRe.FindAllStringSubmatch(out, -1)
	if len(matches) != 1 {
		// We expect one match. If is non one, it's either not found or
		// it's a tree. In either case, we return not found.
		return "", skerr.Wrap(ErrorNotFound)
	}

	if matches[0][1] != gitlinkMode {
		return "", skerr.Wrap(ErrorNotSubmodule)
	}
	return matches[0][3], nil
}

// UpdateSubmodule updates git submodule of the given path to the given commit.
// If submodule doesn't exist, it returns ErrorNotFound since it doesn't have
// all necessary information to create a valid submodule (requires an entry in
// .gitmodules).
func gitRunner_UpdateSubmodule(ctx context.Context, g gitRunner, path, commit string) error {
	if _, err := gitRunner_ReadSubmodule(ctx, g, path, "HEAD"); err != nil {
		return skerr.Wrap(err)
	}
	cacheInfo := fmt.Sprintf("%s,%s,%s", gitlinkMode, commit, path)
	_, err := g.Git(ctx, "update-index", "--add", "--cacheinfo", cacheInfo)
	return skerr.Wrap(err)
}

// NumCommits returns the number of commits in the repo.
func gitRunner_NumCommits(ctx context.Context, g gitRunner) (int64, error) {
	out, err := g.Git(ctx, "rev-list", "--all", "--count")
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	return strconv.ParseInt(strings.TrimSpace(out), 10, 64)
}

// IsAncestor returns true iff A is an ancestor of B.
func gitRunner_IsAncestor(ctx context.Context, g gitRunner, a, b string) (bool, error) {
	out, err := g.Git(ctx, "merge-base", "--is-ancestor", a, b)
	if err != nil {
		// Either a is not an ancestor of b, or we got a real error. If
		// the output is empty, assume it's the former case.
		if out == "" {
			return false, nil
		}
		// "Not a valid commit name" indicates that the given commit does not
		//  exist and thus history was probably changed upstream.
		// A non-existent commit cannot be an ancestor of one which does exist.
		if strings.Contains(out, fmt.Sprintf("Not a valid commit name %s", a)) {
			return false, nil
		}
		// Otherwise, return the presumably real error.
		return false, skerr.Wrap(err)
	}
	return true, nil
}

// Version returns the Git version.
func gitRunner_Version(ctx context.Context) (int, int, error) {
	maj, min, _, err := git_common.Version(ctx)
	return maj, min, skerr.Wrap(err)
}

// FullHash gives the full commit hash for the given ref.
func gitRunner_FullHash(ctx context.Context, g gitRunner, ref string) (string, error) {
	output, err := gitRunner_RevParse(ctx, g, fmt.Sprintf("%s^{commit}", ref))
	if err != nil {
		return "", skerr.Wrapf(err, "failed to obtain full hash")
	}
	return output, nil
}

// CatFile runs "git cat-file -p <ref>:<path>".
func gitRunner_CatFile(ctx context.Context, g gitRunner, ref, path string) ([]byte, error) {
	output, err := g.Git(ctx, "cat-file", "-p", fmt.Sprintf("%s:%s", ref, path))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return []byte(output), nil
}

// ReadDir is analogous to os.File.Readdir for a particular ref.
func gitRunner_ReadDir(ctx context.Context, g gitRunner, ref, path string) ([]os.FileInfo, error) {
	contents, err := gitRunner_CatFile(ctx, g, ref, path)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return ParseDir(contents)
}

// GetRemotes returns a mapping of remote repo name to URL.
func gitRunner_GetRemotes(ctx context.Context, g gitRunner) (map[string]string, error) {
	output, err := g.Git(ctx, "remote", "-v")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	rv := make(map[string]string, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, skerr.Fmt("Got invalid output from `git remote -v`:\n%s", output)
		}
		// First field is the remote name, second is the URL. The third field
		// indicates whether the URL is used for fetching or pushing. In some
		// cases the same remote name might use different URLs for fetching and
		// pushing, in which case the return value will be incorrect.  For our
		// use cases this implementation is enough, but if that changes we may
		// need to return a slice of structs containing the remote name and the
		// fetch and push URLs.
		rv[fields[0]] = fields[1]
	}
	return rv, nil
}
