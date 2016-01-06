// gitinfo enables querying info from a Git repository.
package gitinfo

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

// commitLineRe matches one line of commit log and captures hash, author and
// subject groups.
var commitLineRe = regexp.MustCompile(`([0-9a-f]{40}),([^,\n]+),(.+)$`)

// GitInfo allows querying a Git repo.
type GitInfo struct {
	dir          string
	hashes       []string
	timestamps   map[string]time.Time // Key is the hash.
	detailsCache map[string]*vcsinfo.LongCommit

	// Any access to hashes or timestamps must be protected.
	mutex sync.Mutex
}

// NewGitInfo creates a new GitInfo for the Git repository found in directory
// dir. If pull is true then a git pull is done on the repo before querying it
// for history.
func NewGitInfo(dir string, pull, allBranches bool) (*GitInfo, error) {
	g := &GitInfo{
		dir:          dir,
		hashes:       []string{},
		detailsCache: map[string]*vcsinfo.LongCommit{},
	}
	return g, g.Update(pull, allBranches)
}

// Clone creates a new GitInfo by running "git clone" in the given directory.
func Clone(repoUrl, dir string, allBranches bool) (*GitInfo, error) {
	if _, err := exec.RunSimple(fmt.Sprintf("git clone %s %s", repoUrl, dir)); err != nil {
		return nil, fmt.Errorf("Failed to clone %s into %s: %s", repoUrl, dir, err)
	}
	return NewGitInfo(dir, false, allBranches)
}

// CloneOrUpdate creates a new GitInfo by running "git clone" or "git pull"
// depending on whether the repo already exists.
func CloneOrUpdate(repoUrl, dir string, allBranches bool) (*GitInfo, error) {
	gitDir := path.Join(dir, ".git")
	_, err := os.Stat(gitDir)
	if err == nil {
		return NewGitInfo(dir, true, allBranches)
	}
	if os.IsNotExist(err) {
		return Clone(repoUrl, dir, allBranches)
	}
	return nil, err
}

// Update refreshes the history that GitInfo stores for the repo. If pull is
// true then git pull is performed before refreshing.
func (g *GitInfo) Update(pull, allBranches bool) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	glog.Info("Beginning Update.")
	if pull {
		if _, err := exec.RunCwd(g.dir, "git", "pull"); err != nil {
			return fmt.Errorf("Failed to sync to HEAD: %s", err)
		}
	}
	glog.Info("Finished pull.")
	var hashes []string
	var timestamps map[string]time.Time
	var err error
	if allBranches {
		hashes, timestamps, err = readCommitsFromGitAllBranches(g.dir)
	} else {
		hashes, timestamps, err = readCommitsFromGit(g.dir, "HEAD")
	}
	glog.Infof("Finished reading commits: %s", g.dir)
	if err != nil {
		return fmt.Errorf("Failed to read commits from: %s : %s", g.dir, err)
	}
	g.hashes = hashes
	g.timestamps = timestamps
	return nil
}

// Details returns more information than ShortCommit about a given commit.
// See the vcsinfo.VCS interface for details.
func (g *GitInfo) Details(hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if c, ok := g.detailsCache[hash]; ok {
		return c, nil
	}
	output, err := exec.RunCwd(g.dir, "git", "log", "-n", "1", "--format=format:%H%n%P%n%an%x20(%ae)%n%s%n%b", hash)
	if err != nil {
		return nil, fmt.Errorf("Failed to execute Git: %s", err)
	}
	lines := strings.SplitN(output, "\n", 5)
	if len(lines) != 5 {
		return nil, fmt.Errorf("Failed to parse output of 'git log'.")
	}
	branches := map[string]bool{}
	if includeBranchInfo {
		branches, err = g.getBranchesForCommit(hash)
		if err != nil {
			return nil, err
		}
	}

	c := vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    lines[0],
			Author:  lines[2],
			Subject: lines[3],
		},
		Parents:   strings.Split(lines[1], " "),
		Body:      lines[4],
		Timestamp: g.timestamps[hash],
		Branches:  branches,
	}
	g.detailsCache[hash] = &c
	return &c, nil
}

func (g *GitInfo) SetToCommit(hash string) error {
	_, err := exec.RunCwd(g.dir, "git", "reset", "--hard", hash)
	if err != nil {
		return fmt.Errorf("Failed to roll back/forward to commit %s: %s", hash, err)
	}
	return nil
}

// getBranchesForCommit returns a string set with all the branches that can reach
// the commit with the given hash.
// TODO(stephana): Speed up this method, there are either better ways to do this
// in git or the results can be cached.
func (g *GitInfo) getBranchesForCommit(hash string) (map[string]bool, error) {
	output, err := exec.RunCwd(g.dir, "git", "branch", "--list", "--contains", hash)
	if err != nil {
		return nil, fmt.Errorf("Failed to get branches for commit %s: %s", hash, err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	ret := map[string]bool{}
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l != "" {
			// Splitting the line to filter out the '*' that marks the active branch.
			parts := strings.Split(l, " ")
			ret[parts[len(parts)-1]] = true
		}
	}
	return ret, nil
}

// RevList returns the results of "git rev-list".
func (g *GitInfo) RevList(args ...string) ([]string, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	output, err := exec.RunCwd(g.dir, append([]string{"git", "rev-list"}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("git rev-list failed: %v", err)
	}
	res := strings.Trim(output, "\n")
	if res == "" {
		return []string{}, nil
	}
	return strings.Split(res, "\n"), nil
}

// From returns all commits from 'start' to HEAD.
func (g *GitInfo) From(start time.Time) []string {
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

// LastN returns the last N commits.
func (g *GitInfo) LastN(N int) []string {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if len(g.hashes) < N {
		return g.hashes[0:len(g.hashes)]
	} else {
		return g.hashes[len(g.hashes)-N:]
	}
}

// Timestamp returns the timestamp for the given hash.
func (g *GitInfo) Timestamp(hash string) time.Time {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return g.timestamps[hash]
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
func (g *GitInfo) Log(begin, end string) (string, error) {
	command := []string{"git", "log", "--name-only"}
	hashrange := begin
	if end != "" {
		hashrange += ".." + end
		command = append(command, hashrange)
	} else {
		command = append(command, "-n", "1", hashrange)
	}
	output, err := exec.RunCwd(g.dir, command...)
	if err != nil {
		return "", err
	}
	return output, nil
}

// FullHash gives the full commit hash for the given ref.
func (g *GitInfo) FullHash(ref string) (string, error) {
	output, err := exec.RunCwd(g.dir, "git", "rev-parse", fmt.Sprintf("%s^{commit}", ref))
	if err != nil {
		return "", fmt.Errorf("Failed to obtain full hash: %s", err)
	}
	return strings.Trim(output, "\n"), nil
}

// GetFile returns the contents of the given file at the given commit.
func (g *GitInfo) GetFile(fileName, commit string) (string, error) {
	output, err := exec.RunCwd(g.dir, "git", "show", commit+":"+fileName)
	if err != nil {
		return "", err
	}
	return output, nil
}

// InitalCommit returns the hash of the initial commit.
func (g *GitInfo) InitialCommit() (string, error) {
	output, err := exec.RunCwd(g.dir, "git", "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return "", fmt.Errorf("Failed to determine initial commit: %v", err)
	}
	return strings.Trim(output, "\n"), nil
}

// GetBranches returns a slice of strings naming the branches in the repo.
func (g *GitInfo) GetBranches() ([]*GitBranch, error) {
	return GetBranches(g.dir)
}

// ShortCommits stores a slice of ShortCommit struct.
type ShortCommits struct {
	Commits []*vcsinfo.ShortCommit
}

// ShortList returns a slice of ShortCommit for every commit in (begin, end].
func (g *GitInfo) ShortList(begin, end string) (*ShortCommits, error) {
	command := []string{"git", "log", "--pretty='%H,%an,%s", begin + ".." + end}
	output, err := exec.RunCwd(g.dir, command...)
	if err != nil {
		return nil, err
	}
	ret := &ShortCommits{
		Commits: []*vcsinfo.ShortCommit{},
	}
	for _, line := range strings.Split(output, "\n") {
		match := commitLineRe.FindStringSubmatch(line)
		if match == nil {
			// This could happen if the subject has new line, in which case we truncate it and ignore the remainder.
			continue
		}
		commit := &vcsinfo.ShortCommit{
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

// GitBranch represents a Git branch.
type GitBranch struct {
	Name string `json:"name"`
	Head string `json:"head"`
}

// includeBranchPrefixes is the list of branch prefixes that we should consider in the
// output of the 'git show-ref' command issued in GetBranches below.
var includeBranchPrefixes = []string{
	"refs/remotes/",
	"refs/heads/",
}

// GetBranches returns the list of branch heads in a Git repository.
// In order to separate local working branches from published branches, only
// remote branches in 'origin' are returned.
func GetBranches(dir string) ([]*GitBranch, error) {
	output, err := exec.RunCwd(dir, "git", "show-ref")
	if err != nil {
		return nil, fmt.Errorf("Failed to get branch list: %v", err)
	}
	branches := []*GitBranch{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("Could not parse output of 'git show-ref'.")
		}

		for _, prefix := range includeBranchPrefixes {
			if strings.HasPrefix(parts[1], prefix) {
				name := parts[1][len(prefix):]
				branches = append(branches, &GitBranch{
					Name: name,
					Head: parts[0],
				})
			}
		}
	}
	return branches, nil
}

// readCommitsFromGit reads the commit history from a Git repository.
func readCommitsFromGit(dir, branch string) ([]string, map[string]time.Time, error) {
	output, err := exec.RunCwd(dir, "git", "log", "--format=format:%H%x20%ci", branch)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to execute git log: %s", err)
	}
	lines := strings.Split(output, "\n")
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

func readCommitsFromGitAllBranches(dir string) ([]string, map[string]time.Time, error) {
	branches, err := GetBranches(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not read commits; unable to get branch list: %v", err)
	}
	timestamps := map[string]time.Time{}
	for _, b := range branches {
		_, ts, err := readCommitsFromGit(dir, b.Name)
		if err != nil {
			return nil, nil, err
		}
		for k, v := range ts {
			timestamps[k] = v
		}
	}
	gitHashes := make([]*gitHash, len(timestamps), len(timestamps))
	i := 0
	for h, t := range timestamps {
		gitHashes[i] = &gitHash{hash: h, timeStamp: t}
		i++
	}
	sort.Sort(gitHashSlice(gitHashes))
	hashes := make([]string, len(timestamps), len(timestamps))
	for i, h := range gitHashes {
		hashes[i] = h.hash
	}
	return hashes, timestamps, nil
}

// SkpCommits returns the indices for all the commits that contain SKP updates.
func (g *GitInfo) SkpCommits(tile *tiling.Tile) ([]int, error) {
	// Executes a git log command that looks like:
	//
	//   git log --format=format:%H  32956400b4d8f33394e2cdef9b66e8369ba2a0f3..e7416bfc9858bde8fc6eb5f3bfc942bc3350953a SKP_VERSION
	//
	// The output should be a \n separated list of hashes that match.
	first, last := tile.CommitRange()
	output, err := exec.RunCwd(g.dir, "git", "log", "--format=format:%H", first+".."+last, "SKP_VERSION")
	if err != nil {
		return nil, fmt.Errorf("SkpCommits: Failed to find git log of SKP_VERSION: %s", err)
	}
	hashes := strings.Split(output, "\n")

	ret := []int{}
	for i, c := range tile.Commits {
		if c.CommitTime != 0 && util.In(c.Hash, hashes) {
			ret = append(ret, i)
		}
	}
	return ret, nil
}

// LastSkpCommit returns the time of the last change to the SKP_VERSION file.
func (g *GitInfo) LastSkpCommit() (time.Time, error) {
	// Executes a git log command that looks like:
	//
	// git log --format=format:%ct -n 1 SKP_VERSION
	//
	// The output should be a single unix timestamp.
	output, err := exec.RunCwd(g.dir, "git", "log", "--format=format:%ct", "-n", "1", "SKP_VERSION")
	if err != nil {
		return time.Time{}, fmt.Errorf("LastSkpCommit: Failed to read git log: %s", err)
	}
	ts, err := strconv.ParseInt(output, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("LastSkpCommit: Failed to parse timestamp: %s", err)
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
			return i / tiling.TILE_SIZE, i % tiling.TILE_SIZE, nil
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

// RepoMap is a struct used for managing multiple Git repositories.
type RepoMap struct {
	repos   map[string]*GitInfo
	mutex   sync.RWMutex
	workdir string
}

// NewRepoMap creates and returns a RepoMap which operates within the given
// workdir.
func NewRepoMap(workdir string) *RepoMap {
	return &RepoMap{
		repos:   map[string]*GitInfo{},
		workdir: workdir,
	}
}

// Repo retrieves a pointer to a GitInfo for the requested repo URL. If the
// repo does not yet exist in the repoMap, it is cloned and added before it is
// returned.
func (m *RepoMap) Repo(r string) (*GitInfo, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	repo, ok := m.repos[r]
	if !ok {
		var err error
		split := strings.Split(r, "/")
		repoPath := path.Join(m.workdir, split[len(split)-1])
		repo, err = CloneOrUpdate(r, repoPath, true)
		if err != nil {
			return nil, fmt.Errorf("Failed to check out %s: %v", r, err)
		}
		m.repos[r] = repo
	}
	return repo, nil
}

// RepoForCommit attempts to determine which repository the given commit hash
// belongs to and returns the associated repo URL if found. This is fragile,
// because it's possible, though very unlikely, that there may be collisions
// of commit hashes between repositories.
func (m *RepoMap) RepoForCommit(hash string) (string, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for url, r := range m.repos {
		if _, err := r.FullHash(hash); err == nil {
			return url, nil
		}
	}
	return "", fmt.Errorf("Could not find commit %s in any repo!", hash)
}

// Update causes all of the repos in the RepoMap to be updated.
func (m *RepoMap) Update() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, r := range m.repos {
		if err := r.Update(true, true); err != nil {
			return err
		}
	}
	return nil
}

// Repos returns the list of repos contained in the RepoMap.
func (m *RepoMap) Repos() []string {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	rv := make([]string, 0, len(m.repos))
	for url, _ := range m.repos {
		rv = append(rv, url)
	}
	return rv
}
