// Package gitinfo enables querying info from Git repository using git and a local checkout.
package gitinfo

import (
	"context"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

// commitLineRe matches one line of commit log and captures hash, author and
// subject groups.
var commitLineRe = regexp.MustCompile(`([0-9a-f]{40}),([^,\n]+),(.+)$`)

// GitInfo allows querying a Git repo.
type GitInfo struct {
	dir          git.GitDir
	hashes       []string
	timestamps   map[string]time.Time           // The git hash is the key.
	detailsCache map[string]*vcsinfo.LongCommit // The git hash is the key.
	firstCommit  string

	// Any access to hashes or timestamps must be protected.
	mutex sync.Mutex
}

// GetBranch implements the vcsinfo.VCS interface.
func (g *GitInfo) GetBranch() string {
	// This was added later and we have implicitly assumed that we are tracking
	// the main branch.
	return git.MasterBranch
}

// NewGitInfo creates a new GitInfo for the Git repository found in directory
// dir. If pull is true then a git pull is done on the repo before querying it
// for history.
func NewGitInfo(ctx context.Context, dir string, pull, allBranches bool) (*GitInfo, error) {
	g := &GitInfo{
		dir:          git.GitDir(dir),
		hashes:       []string{},
		detailsCache: map[string]*vcsinfo.LongCommit{},
	}
	return g, g.Update(ctx, pull, allBranches)
}

// Clone creates a new GitInfo by running "git clone" in the given directory.
func Clone(ctx context.Context, repoUrl, dir string, allBranches bool) (*GitInfo, error) {
	gitPath, _, _, err := git_common.FindGit(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if _, err := exec.RunSimple(ctx, fmt.Sprintf("%s clone %s %s", gitPath, repoUrl, dir)); err != nil {
		return nil, fmt.Errorf("Failed to clone %s into %s: %s", repoUrl, dir, err)
	}
	return NewGitInfo(ctx, dir, false, allBranches)
}

// CloneOrUpdate creates a new GitInfo by running "git clone" or "git pull"
// depending on whether the repo already exists.
func CloneOrUpdate(ctx context.Context, repoUrl, dir string, allBranches bool) (*GitInfo, error) {
	gitDir := path.Join(dir, ".git")
	_, err := os.Stat(gitDir)
	if err == nil {
		return NewGitInfo(ctx, dir, true, allBranches)
	}
	if os.IsNotExist(err) {
		return Clone(ctx, repoUrl, dir, allBranches)
	}
	return nil, err
}

// Update refreshes the history that GitInfo stores for the repo. If pull is
// true then git pull is performed before refreshing.
func (g *GitInfo) Update(ctx context.Context, pull, allBranches bool) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	sklog.Info("Beginning Update.")
	if pull {
		if _, err := g.dir.Git(ctx, "pull"); err != nil {
			return fmt.Errorf("Failed to sync to HEAD: %s", err)
		}
	}
	sklog.Info("Finished pull.")
	var hashes []string
	var timestamps map[string]time.Time
	var err error
	if allBranches {
		hashes, timestamps, err = readCommitsFromGitAllBranches(ctx, g.dir)
	} else {
		hashes, timestamps, err = readCommitsFromGit(ctx, g.dir, "HEAD")
	}
	sklog.Infof("Finished reading commits: %s", g.dir)
	if err != nil {
		return fmt.Errorf("Failed to read commits from: %s : %s", g.dir, err)
	}
	g.hashes = hashes
	g.timestamps = timestamps
	g.firstCommit, err = g.InitialCommit(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get initial commit: %s", err)
	}
	return nil
}

// Dir returns the checkout dir of the GitInfo..
func (g *GitInfo) Dir() string {
	return g.dir.Dir()
}

// Details returns more information than ShortCommit about a given commit.
// See the vcsinfo.VCS interface for details.
func (g *GitInfo) Details(ctx context.Context, hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return g.details(ctx, hash, includeBranchInfo)
}

// See the vcsinfo.VCS interface for details.
func (g *GitInfo) DetailsMulti(ctx context.Context, hashes []string, includeBranchInfo bool) ([]*vcsinfo.LongCommit, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	ret := make([]*vcsinfo.LongCommit, len(hashes))
	for idx, hash := range hashes {
		var err error
		if ret[idx], err = g.details(ctx, hash, includeBranchInfo); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

// details returns more information than ShortCommit about a given commit.
// See the vcsinfo.VCS interface for details.
//
// Caller is responsible for locking the mutex.
func (g *GitInfo) details(ctx context.Context, hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	if c, ok := g.detailsCache[hash]; ok {
		// Return the cached value if the branchInfo request matches.
		if !includeBranchInfo || (len(c.Branches) > 0) {
			return c, nil
		}
	}
	output, err := g.dir.Git(ctx, "log", "-n", "1", "--format=format:%H%n%P%n%an%x20(%ae)%n%s%n%b", hash)
	if err != nil {
		return nil, fmt.Errorf("Failed to execute Git: %s", err)
	}
	lines := strings.SplitN(output, "\n", 5)
	if len(lines) != 5 {
		return nil, fmt.Errorf("Failed to parse output of 'git log'.")
	}
	branches := map[string]bool{}
	if includeBranchInfo {
		branches, err = g.getBranchesForCommit(ctx, hash)
		if err != nil {
			return nil, err
		}
	}

	var parents []string
	if lines[1] != "" {
		parents = strings.Split(lines[1], " ")
	}
	c := vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    lines[0],
			Author:  lines[2],
			Subject: lines[3],
		},
		Parents:   parents,
		Body:      lines[4],
		Timestamp: g.timestamps[hash],
		Branches:  branches,
	}
	g.detailsCache[hash] = &c
	return &c, nil
}

func (g *GitInfo) Reset(ctx context.Context, ref string) error {
	_, err := g.dir.Git(ctx, "reset", "--hard", ref)
	if err != nil {
		return fmt.Errorf("Failed to roll back/forward to commit %s: %s", ref, err)
	}
	return nil
}

func (g *GitInfo) Checkout(ctx context.Context, ref string) error {
	if _, err := g.dir.Git(ctx, "checkout", ref); err != nil {
		return fmt.Errorf("Failed to checkout %s: %s", ref, err)
	}
	return nil
}

// getBranchesForCommit returns a string set with all the branches that can reach
// the commit with the given hash.
// TODO(stephana): Speed up this method, there are either better ways to do this
// in git or the results can be cached.
func (g *GitInfo) getBranchesForCommit(ctx context.Context, hash string) (map[string]bool, error) {
	output, err := g.dir.Git(ctx, "branch", "--all", "--list", "--contains", hash)
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
func (g *GitInfo) RevList(ctx context.Context, args ...string) ([]string, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	output, err := g.dir.Git(ctx, append([]string{"rev-list"}, args...)...)
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

// Range returns all commits from the half open interval ['begin', 'end'), i.e.
// includes 'begin' and excludes 'end'.
func (g *GitInfo) Range(begin, end time.Time) []*vcsinfo.IndexCommit {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	ret := []*vcsinfo.IndexCommit{}
	first := sort.Search(len(g.hashes), func(i int) bool {
		ts := g.timestamps[g.hashes[i]]
		return ts.After(begin) || ts.Equal(begin)
	})
	if first == len(g.timestamps) {
		return ret
	}
	for i, h := range g.hashes[first:] {
		if g.timestamps[h].Before(end) {
			ret = append(ret, &vcsinfo.IndexCommit{
				Hash:      h,
				Index:     first + i,
				Timestamp: g.timestamps[h],
			})
		} else {
			break
		}
	}
	return ret
}

// LastNIndex returns the last N commits.
func (g *GitInfo) LastNIndex(N int) []*vcsinfo.IndexCommit {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	var hashes []string
	offset := 0
	if len(g.hashes) < N {
		hashes = g.hashes
	} else {
		hashes = g.hashes[len(g.hashes)-N:]
		offset = len(g.hashes) - N
	}
	ret := []*vcsinfo.IndexCommit{}
	for i, h := range hashes {
		ret = append(ret, &vcsinfo.IndexCommit{
			Hash:      h,
			Index:     i + offset,
			Timestamp: g.timestamps[h],
		})
	}
	return ret
}

// IndexOf returns the index of given hash as counted from the first commit in
// this branch by 'rev-list'. The index is 0 based.
func (g *GitInfo) IndexOf(ctx context.Context, hash string) (int, error) {
	// Count the lines from running:
	//   git rev-list --count <first-commit>..hash.
	output, err := g.RevList(ctx, "--count", git.LogFromTo(g.firstCommit, hash))
	if err != nil {
		return 0, fmt.Errorf("git rev-list failed: %s", err)
	}
	if len(output) != 1 {
		return 0, fmt.Errorf("git rev-list wrong size output: %s", err)
	}
	n, err := strconv.Atoi(output[0])
	if err != nil {
		return 0, fmt.Errorf("Didn't get a number: %s", err)
	}
	return n, nil
}

// ByIndex returns a LongCommit describing the commit
// at position N, as ordered in the current branch.
//
// Does not make sense if readCommitsFromGitAllBranches has been
// called.
func (g *GitInfo) ByIndex(ctx context.Context, N int) (*vcsinfo.LongCommit, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	numHashes := len(g.hashes)
	if N < 0 || N >= numHashes {
		return nil, fmt.Errorf("Hash index not found: %d", N)
	}
	return g.details(ctx, g.hashes[N], false)
}

// LastN returns the last N commits.
func (g *GitInfo) LastN(ctx context.Context, N int) []string {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if len(g.hashes) < N {
		return g.hashes[0:len(g.hashes)]
	} else {
		return g.hashes[len(g.hashes)-N:]
	}
}

// This is a temporary performance enhancement for Perf.
// It will be removed once Perf moves to gitstore.
func (g *GitInfo) TimestampAtIndex(N int) (time.Time, error) {
	if N < 0 || N >= len(g.hashes) {
		return time.Time{}, fmt.Errorf("Hash index not found: %d", N)
	}
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return g.timestamps[g.hashes[N]], nil
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
func (g *GitInfo) Log(ctx context.Context, begin, end string) (string, error) {
	command := []string{"log", "--name-only"}
	hashrange := begin
	if end != "" {
		hashrange += ".." + end
		command = append(command, hashrange)
	} else {
		command = append(command, "-n", "1", hashrange)
	}
	output, err := g.dir.Git(ctx, command...)
	if err != nil {
		return "", err
	}
	return output, nil
}

// LogFine is the same as Log() but appends all the 'args' to the Log
// request to allow finer control of the log output. I.e. you could call:
//
//   LogFine(begin, end, "--format=format:%ct", "infra/bots/assets/skp/VERSION")

func (g *GitInfo) LogFine(ctx context.Context, begin, end string, args ...string) (string, error) {
	command := []string{"log"}
	hashrange := begin
	if end != "" {
		hashrange += ".." + end
		command = append(command, hashrange)
	} else {
		command = append(command, "-n", "1", hashrange)
	}
	command = append(command, args...)
	output, err := g.dir.Git(ctx, command...)
	if err != nil {
		return "", err
	}
	return output, nil
}

// LogArgs is the same as Log() but appends all the 'args' to the Log
// request to allow finer control of the log output. I.e. you could call:
//
//   LogArgs("--since=2015-10-24", "--format=format:%ct", "infra/bots/assets/skp/VERSION")
func (g *GitInfo) LogArgs(ctx context.Context, args ...string) (string, error) {
	command := []string{"log"}
	command = append(command, args...)
	output, err := g.dir.Git(ctx, command...)
	if err != nil {
		return "", err
	}
	return output, nil
}

// FullHash gives the full commit hash for the given ref.
func (g *GitInfo) FullHash(ctx context.Context, ref string) (string, error) {
	output, err := g.dir.Git(ctx, "rev-parse", fmt.Sprintf("%s^{commit}", ref))
	if err != nil {
		return "", fmt.Errorf("Failed to obtain full hash: %s", err)
	}
	return strings.Trim(output, "\n"), nil
}

// InitialCommit returns the hash of the initial commit.
func (g *GitInfo) InitialCommit(ctx context.Context) (string, error) {
	output, err := g.dir.Git(ctx, "rev-list", "--max-parents=0", "--first-parent", "HEAD")
	if err != nil {
		return "", fmt.Errorf("Failed to determine initial commit: %v", err)
	}
	return strings.Trim(output, "\n"), nil
}

// GetBranches returns a slice of strings naming the branches in the repo.
func (g *GitInfo) GetBranches(ctx context.Context) ([]*GitBranch, error) {
	return GetBranches(ctx, string(g.dir))
}

// ShortCommits stores a slice of ShortCommit struct.
type ShortCommits struct {
	Commits []*vcsinfo.ShortCommit
}

// ShortList returns a slice of ShortCommit for every commit in (begin, end].
func (g *GitInfo) ShortList(ctx context.Context, begin, end string) (*ShortCommits, error) {
	command := []string{"log", "--pretty='%H,%an,%s", begin + ".." + end}
	output, err := g.dir.Git(ctx, command...)
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
func GetBranches(ctx context.Context, dir string) ([]*GitBranch, error) {
	output, err := git.GitDir(dir).Git(ctx, "show-ref")
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
func readCommitsFromGit(ctx context.Context, gd git.GitDir, branch string) ([]string, map[string]time.Time, error) {
	output, err := gd.Git(ctx, "log", "--format=format:%H%x20%ci", branch)
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
			t = t.UTC()
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

// GetBranchCommits gets all the commits in the given branch and directory in topological order
// and only with the first parent (omitting commits from branches that are merged in).
// The earliest commits are returned first.
// Note: Primarily used for testing and will probably be removed in the future.
func GetBranchCommits(ctx context.Context, dir, branch string) ([]*vcsinfo.IndexCommit, error) {
	output, err := git.GitDir(dir).Git(ctx, "log", "--format=format:%H%x20%ci", "--first-parent", "--topo-order", "--reverse", branch)
	if err != nil {
		return nil, fmt.Errorf("Failed to execute git log: %s", err)
	}
	lines := strings.Split(output, "\n")
	ret := make([]*vcsinfo.IndexCommit, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			t, err := time.Parse("2006-01-02 15:04:05 -0700", parts[1])
			if err != nil {
				return nil, fmt.Errorf("Failed parsing Git log timestamp: %s", err)
			}
			t = t.UTC()
			hash := parts[0]
			ret = append(ret, &vcsinfo.IndexCommit{
				Hash:      hash,
				Timestamp: t,
				Index:     len(ret),
			})
		}
	}
	return ret, nil
}

func readCommitsFromGitAllBranches(ctx context.Context, gd git.GitDir) ([]string, map[string]time.Time, error) {
	branches, err := GetBranches(ctx, gd.Dir())
	if err != nil {
		return nil, nil, fmt.Errorf("Could not read commits; unable to get branch list: %v", err)
	}
	timestamps := map[string]time.Time{}
	for _, b := range branches {
		_, ts, err := readCommitsFromGit(ctx, gd, b.Name)
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

// Ensure that GitInfo implements vcsinfo.VCS.
var _ vcsinfo.VCS = &GitInfo{}
