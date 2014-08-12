// gitinfo enables querying info from a Git repository.
package gitinfo

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
)

// GitInfo allows querying a Git repo.
type GitInfo struct {
	dir        string
	hashes     []string
	timestamps map[string]time.Time // Key is the hash.
}

// NewGitInfo creates a new GitInfo for the Git repository found in directory
// dir. If pull is true then a git pull is done on the repo before querying it
// for history.
func NewGitInfo(dir string, pull bool) (*GitInfo, error) {
	g := &GitInfo{
		dir:    dir,
		hashes: []string{},
	}
	return g, g.Update(pull)
}

// Update refreshes the history that GitInfo stores for the repo. If pull is
// true then git pull is performed before refreshing.
func (g *GitInfo) Update(pull bool) error {
	glog.Info("Beginning Update.")
	if pull {
		cmd := exec.Command("git", "pull")
		cmd.Dir = g.dir
		b, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("Failed to sync to HEAD: %s - %s", err, string(b))
		}
	}

	hashes, timestamps, err := readCommitsFromGit(g.dir)
	if err != nil {
		return err
	}
	g.hashes = hashes
	g.timestamps = timestamps
	return nil
}

// Details returns the author, subject and timestamp for the given commit.
func (g *GitInfo) Details(hash string) (string, string, time.Time, error) {
	cmd := exec.Command("git", "log", "-n", "1", "--format=format:%an%x20(%ae)%n%s", hash)
	cmd.Dir = g.dir
	b, err := cmd.Output()
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("Failed to execute Git: %s", err)
	}
	lines := strings.SplitN(string(b), "\n", 2)
	if len(lines) == 2 {
		return lines[0], lines[1], g.timestamps[hash], nil
	} else {
		return lines[0], "", time.Time{}, nil
	}
}

// From returns all commits from 'start' to HEAD.
func (g *GitInfo) From(start time.Time) []string {
	ret := []string{}
	for _, h := range g.hashes {
		if g.timestamps[h].After(start) {
			ret = append(ret, h)
		}
	}
	return ret
}

// From returns a --name-only short log for every commit in (begin, end].
//
// If end is "" then it returns just the short log for the single commit at
// begin.
//
// Example response:
//
//    commit b7988a21fdf23cc4ace6145a06ea824aa85db099
//    Author: Joe Gregorio <jcgregorio@google.com>
//    Date:   Tue Aug 5 16:19:48 2014 -0400
//
//        A description of the commit.
//
//    perf/go/skiaperf/perf.go
//    perf/go/types/types.go
//    perf/res/js/logic.js
//
func (g *GitInfo) Log(begin, end string) (string, error) {
	command := []string{"log", "--name-only"}
	hashrange := begin
	if end != "" {
		hashrange += ".." + end
		command = append(command, hashrange)
	} else {
		command = append(command, "-n", "1", hashrange)
	}
	cmd := exec.Command("git", command...)
	cmd.Dir = g.dir
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// gitHash represents information on a single Git commit.
type gitHash struct {
	hash      string
	timeStamp time.Time
}

type gitHashSlice []*gitHash

func (p gitHashSlice) Len() int           { return len(p) }
func (p gitHashSlice) Less(i, j int) bool { return p[i].timeStamp.Before(p[j].timeStamp) }
func (p gitHashSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// readCommitsFromGit reads the commit history from a Git repository.
func readCommitsFromGit(dir string) ([]string, map[string]time.Time, error) {
	cmd := exec.Command("git", "log", "--format=format:%H%x20%ci")
	cmd.Dir = dir
	b, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to execute git log: %s - %s", err, string(b))
	}
	lines := strings.Split(string(b), "\n")
	gitHashes := make([]*gitHash, 0, len(lines))
	timestamps := map[string]time.Time{}
	for _, line := range lines {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			t, err := time.Parse("2006-01-02 15:04:05 -0700", parts[1])
			if err != nil {
				return nil, nil, fmt.Errorf("Failed parsing Git log timestamp: %s", err)
			}
			hash := parts[0]
			gitHashes = append(gitHashes, &gitHash{hash: hash, timeStamp: t})
			timestamps[hash] = t
		}
	}
	sort.Sort(gitHashSlice(gitHashes))
	hashes := make([]string, len(gitHashes), len(gitHashes))
	for i, h := range gitHashes {
		hashes[i] = h.hash
	}
	return hashes, timestamps, nil
}
