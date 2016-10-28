package git

/*
	Common utils used by Repo and Checkout.
*/

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/vcsinfo"
)

// Branch describes a Git branch.
type Branch struct {
	// The human-readable name of the branch.
	Name string

	// The commit hash pointed to by this branch.
	Head string
}

// GitDir is a directory in which one may run Git commands.
type GitDir string

// newGitDir creates a GitDir instance based in the given directory.
func newGitDir(repoUrl, workdir string, mirror bool) (GitDir, error) {
	dest := path.Join(workdir, strings.TrimSuffix(path.Base(repoUrl), ".git"))
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			// Clone the repo.
			cmd := []string{"git", "clone"}
			if mirror {
				cmd = append(cmd, "--mirror")
			}
			cmd = append(cmd, repoUrl, dest)
			if _, err := exec.RunCwd(workdir, cmd...); err != nil {
				return "", fmt.Errorf("Failed to clone repo: %s", err)
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
func (g GitDir) Git(cmd ...string) (string, error) {
	return exec.RunCwd(string(g), append([]string{"git"}, cmd...)...)
}

// Details returns a vcsinfo.LongCommit instance representing the given commit.
func (g GitDir) Details(name string) (*vcsinfo.LongCommit, error) {
	output, err := g.Git("log", "-n", "1", "--format=format:%H%n%P%n%an%x20(%ae)%n%s%n%ct%n%b", name)
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
		Timestamp: time.Unix(ts, 0),
	}, nil
}

// RevParse runs "git rev-parse <name>" and returns the result.
func (g GitDir) RevParse(args ...string) (string, error) {
	out, err := g.Git(append([]string{"rev-parse"}, args...)...)
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
func (g GitDir) RevList(name string) ([]string, error) {
	out, err := g.Git("rev-list", name)
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// GetBranchHead returns the commit hash at the HEAD of the given branch.
func (g GitDir) GetBranchHead(branchName string) (string, error) {
	return g.RevParse("--verify", fmt.Sprintf("refs/heads/%s^{commit}", branchName))
}

// Branches runs "git branch" and returns a slice of Branch instances.
func (g GitDir) Branches() ([]*Branch, error) {
	out, err := g.Git("branch")
	if err != nil {
		return nil, err
	}
	branchNames := strings.Fields(out)
	branches := make([]*Branch, 0, len(branchNames))
	for _, name := range branchNames {
		if name == "*" {
			continue
		}
		head, err := g.GetBranchHead(name)
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
func (g GitDir) GetFile(fileName, commit string) (string, error) {
	return g.Git("show", commit+":"+fileName)
}

// NumCommits returns the number of commits in the repo.
func (g GitDir) NumCommits() (int64, error) {
	out, err := g.Git("rev-list", "--all", "--count")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(out), 10, 64)
}
