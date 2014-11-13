// gitinfo enables querying info from a Git repository.
package gitinfo

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/types"

	"github.com/golang/glog"
)

// commitLineRe matches one line of commit log and captures hash, author and
// subject groups.
var commitLineRe = regexp.MustCompile(`([0-9a-f]{40}),([^,\n]+),(.+)$`)

// GitInfo allows querying a Git repo.
type GitInfo struct {
	dir        string
	hashes     []string
	timestamps map[string]time.Time // Key is the hash.

	// Any access to hashes or timestamps must be protected.
	mutex sync.Mutex
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

// Clone creates a new GitInfo by running "git clone" in the given directory.
func Clone(repoUrl, dir string) (*GitInfo, error) {
	cmd := exec.Command("git", "clone", repoUrl, dir)
	_, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Failed to clone %s into %s: %v", repoUrl, dir, err)
	}
	return NewGitInfo(dir, false)
}

// CloneOrUpdate creates a new GitInfo by running "git clone" or "git pull"
// depending on whether the repo already exists.
func CloneOrUpdate(repoUrl, dir string) (*GitInfo, error) {
	gitDir := path.Join(dir, ".git")
	_, err := os.Stat(gitDir)
	if err == nil {
		return NewGitInfo(dir, true)
	}
	if os.IsNotExist(err) {
		return Clone(repoUrl, dir)
	}
	return nil, err
}

// Update refreshes the history that GitInfo stores for the repo. If pull is
// true then git pull is performed before refreshing.
func (g *GitInfo) Update(pull bool) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()
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
func (g GitInfo) Details(hash string) (string, string, time.Time, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	cmd := exec.Command("git", "log", "-n", "1", "--format=format:%an%x20(%ae)%n%s", hash)
	cmd.Dir = g.dir
	b, err := cmd.Output()
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("Failed to execute Git: %s", err)
	}
	lines := strings.SplitN(string(b), "\n", 2)
	if len(lines) != 2 {
		return lines[0], "", time.Time{}, nil
	}

	return lines[0], lines[1], g.timestamps[hash], nil
}

// From returns all commits from 'start' to HEAD.
func (g GitInfo) From(start time.Time) []string {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	ret := []string{}
	for _, h := range g.hashes {
		if g.timestamps[h].After(start) {
			ret = append(ret, h)
		}
	}
	return ret
}

// Log returns a --name-only short log for every commit in (begin, end].
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
func (g GitInfo) Log(begin, end string) (string, error) {
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

// FullHash gives the full commit hash for the given ref.
func (g GitInfo) FullHash(ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = g.dir
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// GetFile returns the contents of the given file at the given commit.
func (g GitInfo) GetFile(fileName, commit string) (string, error) {
	cmd := exec.Command("git", "show", commit+":"+fileName)
	cmd.Dir = g.dir
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// InitalCommit returns the hash of the initial commit.
func (g GitInfo) InitialCommit() (string, error) {
	cmd := exec.Command("git", "rev-list", "--max-parents=0", "HEAD")
	cmd.Dir = g.dir
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Failed to determine initial commit: %v", err)
	}
	return strings.Trim(string(b), "\n"), nil
}

// ShortCommit stores the hash, author, and subject of a git commit.
type ShortCommit struct {
	Hash    string
	Author  string
	Subject string
}

// ShortCommits stores a slice of ShortCommit struct.
type ShortCommits struct {
	Commits []*ShortCommit
}

// ShortList returns a slice of ShortCommit for every commit in (begin, end].
func (g *GitInfo) ShortList(begin, end string) (*ShortCommits, error) {
	command := []string{"log", "--pretty='%H,%an,%s", begin + ".." + end}
	cmd := exec.Command("git", command...)
	cmd.Dir = g.dir
	b, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	ret := &ShortCommits{
		Commits: []*ShortCommit{},
	}
	for _, line := range strings.Split(string(b), "\n") {
		match := commitLineRe.FindStringSubmatch(line)
		if match == nil {
			// This could happen if the subject has new line, in which case we truncate it and ignore the remainder.
			continue
		}
		commit := &ShortCommit{
			Hash:    match[1],
			Author:  match[2],
			Subject: match[3],
		}
		ret.Commits = append(ret.Commits, commit)
	}

	return ret, nil
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

// SkpCommits returns the indices for all the commits that contain SKP updates.
func (g GitInfo) SkpCommits(tile *types.Tile) ([]int, error) {
	// Executes a git log command that looks like:
	//
	//   git log --format=format:%H  32956400b4d8f33394e2cdef9b66e8369ba2a0f3..e7416bfc9858bde8fc6eb5f3bfc942bc3350953a SKP_VERSION
	//
	// The output should be a \n separated list of hashes that match.
	first, last := tile.CommitRange()
	cmd := exec.Command("git", "log", "--format=format:%H", first+".."+last, "SKP_VERSION")
	cmd.Dir = g.dir
	b, err := cmd.Output()
	if err != nil {
		glog.Error(string(b))
		return nil, err
	}
	hashes := strings.Split(string(b), "\n")

	ret := []int{}
	for i, c := range tile.Commits {
		if c.CommitTime != 0 && util.In(c.Hash, hashes) {
			ret = append(ret, i)
		}
	}
	return ret, nil
}

// LastSkpCommit returns the time of the last change to the SKP_VERSION file.
func (g GitInfo) LastSkpCommit() (time.Time, error) {
	// Executes a git log command that looks like:
	//
	// git log --format=format:%ct -n 1 SKP_VERSION
	//
	// The output should be a single unix timestamp.
	cmd := exec.Command("git", "log", "--format=format:%ct", "-n", "1", "SKP_VERSION")
	cmd.Dir = g.dir
	b, err := cmd.Output()
	if err != nil {
		glog.Error("Failed to read git log: ", err)
		return time.Time{}, err
	}
	ts, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		glog.Error("Failed to parse timestamp: ", string(b), err)
		return time.Time{}, err
	}
	return time.Unix(ts, 0), nil
}

// TileAddressFromHash takes a commit hash and time, then returns the Level 0
// tile number that contains the hash, and its position in the tile commit array.
// This assumes that tiles are built for commits since after the given time.
func (g *GitInfo) TileAddressFromHash(hash string, start time.Time) (num, offset int, err error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	i := 0
	for _, h := range g.hashes {
		if g.timestamps[h].Before(start) {
			continue
		}
		if h == hash {
			return i / config.TILE_SIZE, i % config.TILE_SIZE, nil
		}
		i++
	}
	return -1, -1, fmt.Errorf("Cannot find hash %s.\n", hash)
}

// NumCommits returns the number of commits in the repo.
func (g *GitInfo) NumCommits() int {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return len(g.hashes)
}
