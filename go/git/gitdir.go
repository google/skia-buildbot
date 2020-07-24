package git

/*
	Common utils used by Repo and Checkout.
*/

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vcsinfo"
)

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

// Executable returns the path to Git.
func Executable(ctx context.Context) (string, error) {
	git, _, _, err := git_common.FindGit(ctx)
	return git, err
}

// GitDir is a directory in which one may run Git commands.
type GitDir string

// newGitDir creates a GitDir instance based in the given directory.
func newGitDir(ctx context.Context, repoUrl, workdir string, mirror bool) (GitDir, error) {
	dest := path.Join(workdir, strings.TrimSuffix(path.Base(repoUrl), ".git"))
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			if err := Clone(ctx, repoUrl, dest, mirror); err != nil {
				return "", err
			}
		} else {
			return "", fmt.Errorf("There is a problem with the git directory: %s", err)
		}
	}
	return GitDir(dest), nil
}

// Dir returns the working directory of the GitDir.
func (g GitDir) Dir() string {
	return string(g)
}

// Git runs the given git command in the GitDir.
func (g GitDir) Git(ctx context.Context, cmd ...string) (string, error) {
	git, err := Executable(ctx)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return exec.RunCwd(ctx, g.Dir(), append([]string{git}, cmd...)...)
}

// Details returns a vcsinfo.LongCommit instance representing the given commit.
func (g GitDir) Details(ctx context.Context, name string) (*vcsinfo.LongCommit, error) {
	output, err := g.Git(ctx, "log", "-n", "1", "--format=format:%H%n%P%n%an%x20(%ae)%n%s%n%ct%n%b", name)
	if err != nil {
		return nil, err
	}
	lines := strings.SplitN(output, "\n", 6)
	if len(lines) != 6 {
		return nil, fmt.Errorf("Failed to parse output of 'git log'.")
	}
	var parents []string
	if lines[1] != "" {
		parents = strings.Split(lines[1], " ")
	}
	ts, err := strconv.ParseInt(lines[4], 10, 64)
	if err != nil {
		return nil, err
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
func (g GitDir) RevParse(ctx context.Context, args ...string) (string, error) {
	out, err := g.Git(ctx, append([]string{"rev-parse"}, args...)...)
	if err != nil {
		return "", err
	}
	// Ensure that we got a single, 40-character commit hash.
	split := strings.Fields(out)
	if len(split) != 1 {
		return "", fmt.Errorf("Unable to parse commit hash from output: %s", out)
	}
	if len(split[0]) != 40 {
		return "", fmt.Errorf("rev-parse returned invalid commit hash: %s", out)
	}
	return split[0], nil
}

// RevList runs "git rev-list <name>" and returns a slice of commit hashes.
func (g GitDir) RevList(ctx context.Context, args ...string) ([]string, error) {
	out, err := g.Git(ctx, append([]string{"rev-list"}, args...)...)
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// GetBranchHead returns the commit hash at the HEAD of the given branch.
func (g GitDir) GetBranchHead(ctx context.Context, branchName string) (string, error) {
	return g.RevParse(ctx, "--verify", fmt.Sprintf("refs/heads/%s^{commit}", branchName))
}

// Branches runs "git branch" and returns a slice of Branch instances.
func (g GitDir) Branches(ctx context.Context) ([]*Branch, error) {
	out, err := g.Git(ctx, "branch")
	if err != nil {
		return nil, err
	}
	branchNames := strings.Fields(out)
	branches := make([]*Branch, 0, len(branchNames))
	for _, name := range branchNames {
		if name == "*" {
			continue
		}
		head, err := g.GetBranchHead(ctx, name)
		if err != nil {
			return nil, err
		}
		branches = append(branches, &Branch{
			Head: head,
			Name: name,
		})
	}
	return branches, nil
}

// GetFile returns the contents of the given file at the given commit.
func (g GitDir) GetFile(ctx context.Context, fileName, commit string) (string, error) {
	return g.Git(ctx, "show", commit+":"+fileName)
}

// NumCommits returns the number of commits in the repo.
func (g GitDir) NumCommits(ctx context.Context) (int64, error) {
	out, err := g.Git(ctx, "rev-list", "--all", "--count")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(out), 10, 64)
}

// IsAncestor returns true iff A is an ancestor of B.
func (g GitDir) IsAncestor(ctx context.Context, a, b string) (bool, error) {
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
		return false, fmt.Errorf("%s: %s", err, out)
	}
	return true, nil
}

// Version returns the Git version.
func (g GitDir) Version(ctx context.Context) (int, int, error) {
	_, maj, min, err := git_common.FindGit(ctx)
	return maj, min, err
}

// FullHash gives the full commit hash for the given ref.
func (g GitDir) FullHash(ctx context.Context, ref string) (string, error) {
	output, err := g.RevParse(ctx, fmt.Sprintf("%s^{commit}", ref))
	if err != nil {
		return "", fmt.Errorf("Failed to obtain full hash: %s", err)
	}
	return output, nil
}

// CatFile runs "git cat-file -p <ref>:<path>".
func (g GitDir) CatFile(ctx context.Context, ref, path string) ([]byte, error) {
	output, err := g.Git(ctx, "cat-file", "-p", fmt.Sprintf("%s:%s", ref, path))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return []byte(output), nil
}

// ReadDir is analogous to os.File.Readdir for a particular ref.
func (g GitDir) ReadDir(ctx context.Context, ref, path string) ([]os.FileInfo, error) {
	contents, err := g.CatFile(ctx, ref, path)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return ParseDir(contents)
}
