// gitinfo enables querying info from a Git repository.
package gitinfo

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// gitHash represents information on a single Git commit.
type gitHash struct {
	hash      string
	timeStamp time.Time
}

// GitInfo allows querying a Git repo.
type GitInfo struct {
	dir    string
	hashes []*gitHash
}

// NewGitInfo creates a new GitInfo for the Git repository found in directory
// dir. If pull is true then a git pull is done on the repo before querying it
// for history.
func NewGitInfo(dir string, pull bool) (*GitInfo, error) {
	g := &GitInfo{
		dir:    dir,
		hashes: []*gitHash{},
	}
	return g, g.Update(pull)
}

// Update refreshes the history that GitInfo stores for the repo. If pull is
// true then git pull is performed before refreshing.
func (g *GitInfo) Update(pull bool) error {
	if pull {
		cmd := exec.Command("git", "pull")
		cmd.Dir = g.dir
		b, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("Failed to sync to HEAD: %s - %s", err, string(b))
		}
	}

	hashes, err := readCommitsFromGit(g.dir)
	if err != nil {
		return err
	}
	g.hashes = hashes
	return nil
}

// Details returns the subject and body for the given commit.
func (g *GitInfo) Details(hash string) (string, string, error) {
	cmd := exec.Command("git", "log", "-n", "1", "--format=format:%s%n%b", hash)
	cmd.Dir = g.dir
	b, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("Failed to execute Git: %s", err)
	}
	lines := strings.SplitN(string(b), "\n", 2)
	if len(lines) == 2 {
		return lines[0], lines[1], nil
	} else {
		return lines[0], "", nil
	}
}

// From returns all commits from 'start' to HEAD.
func (g *GitInfo) From(start time.Time) []string {
	ret := []string{}
	for _, h := range g.hashes {
		if h.timeStamp.After(start) {
			ret = append(ret, h.hash)
		}
	}
	return ret
}

type gitHashSlice []*gitHash

func (p gitHashSlice) Len() int           { return len(p) }
func (p gitHashSlice) Less(i, j int) bool { return p[i].timeStamp.Before(p[j].timeStamp) }
func (p gitHashSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// readCommitsFromGit reads the commit history from a Git repository.
func readCommitsFromGit(dir string) ([]*gitHash, error) {
	cmd := exec.Command("git", "log", "--format=format:%H%x20%ci")
	cmd.Dir = dir
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Failed to execute git log: %s - %s", err, string(b))
	}
	lines := strings.Split(string(b), "\n")
	hashes := make([]*gitHash, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			t, err := time.Parse("2006-01-02 15:04:05 -0700", parts[1])
			if err != nil {
				return nil, fmt.Errorf("Failed parsing Git log timestamp: %s", err)
			}
			hashes = append(hashes, &gitHash{hash: parts[0], timeStamp: t})
		}
	}
	sort.Sort(gitHashSlice(hashes))
	return hashes, nil
}
