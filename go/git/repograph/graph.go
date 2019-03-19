package repograph

/*
   The repograph package provides an in-memory representation of an entire Git repo.
*/

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

var (
	ErrStopRecursing = errors.New("Stop recursing")
)

// Commit represents a commit in a Git repo.
type Commit struct {
	*vcsinfo.LongCommit
	parents []*Commit

	// HistoryLen is the number of commits in the longest line of history
	// reachable from this Commit. It is only used during Recurse as an
	// estimate of the number of commits which might be visited, to prevent
	// excessive resizing of the visited map.
	HistoryLen int
}

// Parents returns the parents of this commit.
func (c *Commit) GetParents() []*Commit {
	return c.parents
}

// Recurse runs the given function recursively over commit history, starting
// at the given commit. The function accepts the current Commit as a parameter.
// Returning ErrStopRecursing from the function indicates that recursion should
// stop for the current branch, however, recursion will continue for any other
// branches until they are similarly terminated. Returning any other error
// causes recursion to stop without properly terminating other branches. The
// error will bubble to the top and be returned. Here's an example of printing
// out the entire ancestry of a given commit:
//
// commit.Recurse(func(c *Commit) error {
// 	sklog.Info(c.Hash)
// 	return nil
// })
func (c *Commit) Recurse(f func(*Commit) error) error {
	return c.recurse(f, make(map[*Commit]bool, c.HistoryLen))
}

// recurse is a helper function used by Recurse.
func (c *Commit) recurse(f func(*Commit) error, visited map[*Commit]bool) (rvErr error) {
	// For large repos, we may not have enough stack space to recurse
	// through the whole commit history. Since most commits only have
	// one parent, avoid recursion when possible.
	for {
		visited[c] = true
		if err := f(c); err == ErrStopRecursing {
			return nil
		} else if err != nil {
			return err
		}
		if len(c.parents) == 0 {
			return nil
		} else if len(c.parents) == 1 {
			p := c.parents[0]
			if visited[p] {
				return nil
			}
			c = p
		} else {
			break
		}
	}
	if len(c.Parents) > 1 {
		for _, p := range c.parents {
			if visited[p] {
				continue
			}
			if err := p.recurse(f, visited); err != nil {
				return err
			}
		}
	}
	return nil
}

// AllCommits returns the hashes of all commits reachable from this Commit, in
// reverse topological order.
func (c *Commit) AllCommits() ([]string, error) {
	commits := make(map[*Commit]bool, c.HistoryLen)
	if err := c.recurse(func(c *Commit) error {
		return nil
	}, commits); err != nil {
		return nil, err
	}

	// Topologically sort the commits.
	sorted := topologicalSortHelper(commits)
	return CommitSlice(sorted).Hashes(), nil
}

// HasAncestor returns true iff the given commit is an ancestor of this commit.
func (c *Commit) HasAncestor(other string) bool {
	found := false
	if err := c.Recurse(func(commit *Commit) error {
		if commit.Hash == other {
			found = true
			return ErrStopRecursing
		}
		return nil
	}); err != nil {
		// Our function doesn't return an error, so we shouldn't hit
		// this case.
		sklog.Errorf("Error in Commit.Recurse: %s", err)
	}
	return found
}

// Less compares this Commit to the other Commit and returns true if it is
// considered "less" than the other. This is used as a helper function in
// sorting to ensure stability. Uses the timestamp as the primary sort function
// and the hash as a tie breaker. Note that, because we sort in reverse
// chronological order, this has the opposite meaning that it seems it should.
func (c *Commit) Less(other *Commit) bool {
	if c.Timestamp.Equal(other.Timestamp) {
		// If the timestamps are equal, just use the commit hash as an
		// arbitrary way of comparing the two.
		return c.Hash > other.Hash
	}
	return c.Timestamp.After(other.Timestamp)
}

// Helpers for sorting.
type CommitSlice []*Commit

func (s CommitSlice) Len() int           { return len(s) }
func (s CommitSlice) Less(a, b int) bool { return s[a].Less(s[b]) }
func (s CommitSlice) Swap(a, b int)      { s[a], s[b] = s[b], s[a] }

// Hashes returns a slice of commit hashes corresponding to the commits in s.
func (s CommitSlice) Hashes() []string {
	rv := make([]string, 0, len(s))
	for _, c := range s {
		rv = append(rv, c.Hash)
	}
	return rv
}

// Updater updates a Graph.
type Updater interface {
	// Update the given graph with any new data.
	Update(context.Context, *Graph) error
}

// Graph represents an entire Git repo.
type Graph struct {
	branches []*git.Branch
	commits  map[string]*Commit
	graphMtx sync.RWMutex

	updateMtx sync.Mutex
	updater   Updater
}

// NewLocalGraph returns a Graph instance, creating a git.Repo from the repoUrl
// and workdir. May obtain cached data from a file in the git repo, but does NOT
// update the Graph; the caller is responsible for doing so before using the
// Graph if up-to-date data is required.
func NewLocalGraph(ctx context.Context, repoUrl, workdir string) (*Graph, error) {
	repo, err := git.NewRepo(ctx, repoUrl, workdir)
	if err != nil {
		return nil, fmt.Errorf("Failed to create git repo: %s", err)
	}
	rv := &Graph{
		commits: map[string]*Commit{},
		updater: &repoUpdater{repo},
	}
	if err := initFromFile(rv, path.Join(repo.Dir(), CACHE_FILE)); err != nil {
		return nil, err
	}
	return rv, nil
}

// initFromFile initializes the Graph from a file.
func initFromFile(g *Graph, cacheFile string) error {
	var r gobGraph
	if err := util.MaybeReadGobFile(cacheFile, &r); err != nil {
		sklog.Errorf("Failed to read Graph cache file %s; deleting the file and starting from scratch: %s", cacheFile, err)
		if err2 := os.Remove(cacheFile); err != nil {
			return fmt.Errorf("Failed to read Graph cache file %s: %s\n...and failed to remove with: %s", cacheFile, err, err2)
		}
	}
	if r.Branches != nil {
		g.branches = r.Branches
	}
	if r.Commits != nil {
		g.commits = r.Commits
	}
	for _, c := range g.commits {
		for _, parentHash := range c.Parents {
			c.parents = append(c.parents, g.commits[parentHash])
		}
	}
	return nil
}

// NewGitStoreGraph returns a Graph instance which is backed by a GitStore.
func NewGitStoreGraph(ctx context.Context, gs gitstore.GitStore) (*Graph, error) {
	return NewWithUpdater(ctx, &gitstoreUpdater{
		gs: gs,
	})
}

// NewWithUpdater returns a Graph instance which uses the given Updater.
func NewWithUpdater(ctx context.Context, ud Updater) (*Graph, error) {
	rv := &Graph{
		commits: map[string]*Commit{},
		updater: ud,
	}
	if err := rv.Update(ctx); err != nil {
		return nil, err
	}
	return rv, nil
}

// Len returns the number of commits in the repo.
func (r *Graph) Len() int {
	r.graphMtx.RLock()
	defer r.graphMtx.RUnlock()
	return len(r.commits)
}

// addCommit adds the given commit to the Graph. Requires that the commit's
// parents are already in the Graph. Assumes that the caller holds r.graphMtx.
func (r *Graph) addCommit(lc *vcsinfo.LongCommit) error {
	maxParentHistoryLen := 0
	var parents []*Commit
	if len(lc.Parents) > 0 {
		for _, h := range lc.Parents {
			if h == "" {
				continue
			}
			p, ok := r.commits[h]
			if !ok {
				return fmt.Errorf("repograph.Graph: Could not find parent commit %q", h)
			}
			parents = append(parents, p)
			if p.HistoryLen > maxParentHistoryLen {
				maxParentHistoryLen = p.HistoryLen
			}
		}
	}

	c := &Commit{
		LongCommit: lc,
		parents:    parents,
		HistoryLen: maxParentHistoryLen + 1,
	}
	r.commits[c.Hash] = c
	return nil
}

// UpdateLock locks the update mutex. Should not be used with Update or
// UpdateAndReturnCommitDiffs; only intended for use with UpdateCopy and
// ReplaceContents.
func (r *Graph) UpdateLock() {
	r.updateMtx.Lock()
}

// UpdateUnlock unlocks the update mutex. Should not be used with Update or
// UpdateAndReturnCommitDiffs; only intended for use with UpdateCopy and
// ReplaceContents.
func (r *Graph) UpdateUnlock() {
	r.updateMtx.Unlock()
}

// UpdateCopy returns a copy of the Graph which has been updated. If used in
// conjunction with ReplaceContents, the caller should use UpdateLock and
// UpdateUnlock to ensure that the Graph is not concurrently modified.
func (r *Graph) UpdateCopy(ctx context.Context) (*Graph, error) {
	newGraph := r.ShallowCopy()
	if err := r.updater.Update(ctx, newGraph); err != nil {
		return nil, err
	}
	return newGraph, nil
}

// Update the Graph.
func (r *Graph) Update(ctx context.Context) error {
	r.updateMtx.Lock()
	defer r.updateMtx.Unlock()
	newGraph, err := r.UpdateCopy(ctx)
	if err != nil {
		return err
	}
	r.ReplaceContents(newGraph)
	return nil
}

// UpdateAndReturnCommitDiffs updates the Graph and returns the added and
// removed commits.
func (r *Graph) UpdateAndReturnCommitDiffs(ctx context.Context) ([]*vcsinfo.LongCommit, []*vcsinfo.LongCommit, error) {
	r.updateMtx.Lock()
	defer r.updateMtx.Unlock()
	newGraph, err := r.UpdateCopy(ctx)
	if err != nil {
		return nil, nil, err
	}
	added, removed := r.DiffCommits(newGraph)
	r.ReplaceContents(newGraph)
	return added, removed, nil
}

// DiffCommits returns the commits present in the other Graph but not this one
// and the commits present on this Graph but not the other.
func (r *Graph) DiffCommits(other *Graph) ([]*vcsinfo.LongCommit, []*vcsinfo.LongCommit) {
	r.graphMtx.Lock()
	defer r.graphMtx.Unlock()
	other.graphMtx.Lock()
	defer other.graphMtx.Unlock()
	return r.diffCommits(other)
}

// diffCommits returns the commits present in the other Graph but not this one
// and the commits present in this Graph but not the other. Assumes that the
// caller holds r.graphMtx and other.graphMtx.
func (r *Graph) diffCommits(other *Graph) ([]*vcsinfo.LongCommit, []*vcsinfo.LongCommit) {
	var added []*vcsinfo.LongCommit
	var removed []*vcsinfo.LongCommit
	for hash, c := range r.commits {
		if _, ok := other.commits[hash]; !ok {
			removed = append(removed, c.LongCommit)
		}
	}
	for hash, c := range other.commits {
		if _, ok := r.commits[hash]; !ok {
			added = append(added, c.LongCommit)
		}
	}
	return added, removed
}

// ReplaceContents replaces the contents of this Graph with those of the other.
// If used in conjunction with UpdateCopy, the caller should use UpdateLock and
// UpdateUnlock to prevent concurrent modification.
func (r *Graph) ReplaceContents(other *Graph) {
	r.graphMtx.Lock()
	defer r.graphMtx.Unlock()
	other.graphMtx.Lock()
	defer other.graphMtx.Unlock()
	r.branches = other.branches
	r.commits = other.commits
}

// Write the Graph to the cache file in the given Repo.
func (r *Graph) WriteCacheFile(cacheFile string) error {
	sklog.Infof("  Writing cache file...")
	return util.WithWriteFile(cacheFile, func(w io.Writer) error {
		return gob.NewEncoder(w).Encode(gobGraph{
			Branches: r.branches,
			Commits:  r.commits,
		})
	})
}

// Branches returns the list of known branches in the repo.
func (r *Graph) Branches() []string {
	r.graphMtx.RLock()
	defer r.graphMtx.RUnlock()
	rv := make([]string, 0, len(r.branches))
	for _, b := range r.branches {
		rv = append(rv, b.Name)
	}
	return rv
}

// BranchHeads returns the set of branch heads from the repo.
func (r *Graph) BranchHeads() []*git.Branch {
	branches := r.Branches()
	branchHeads := make([]*git.Branch, 0, len(branches))
	for _, b := range branches {
		branchHeads = append(branchHeads, &git.Branch{
			Name: b,
			Head: r.Get(b).Hash,
		})
	}
	return branchHeads
}

// ShallowCopy() returns a shallow copy of the Graph, ie. the pointers in the
// old and new Graphs will remain equal for a given Commit.
func (r *Graph) ShallowCopy() *Graph {
	r.graphMtx.RLock()
	defer r.graphMtx.RUnlock()
	newCommits := make(map[string]*Commit, len(r.commits))
	for k, v := range r.commits {
		newCommits[k] = v
	}
	newBranches := make([]*git.Branch, 0, len(r.branches))
	for _, branch := range r.branches {
		newBranches = append(newBranches, &git.Branch{
			Head: branch.Head,
			Name: branch.Name,
		})
	}
	return &Graph{
		branches: newBranches,
		commits:  newCommits,
	}
}

// Get returns a Commit object for the given ref, if such a commit exists. This
// function does not understand complex ref types (eg. HEAD~3); only branch
// names and full commit hashes are accepted.
func (r *Graph) Get(ref string) *Commit {
	r.graphMtx.RLock()
	defer r.graphMtx.RUnlock()
	if c, ok := r.commits[ref]; ok {
		return c
	}
	for _, b := range r.branches {
		if ref == b.Name {
			if c, ok := r.commits[b.Head]; ok {
				return c
			}
		}
	}
	return nil
}

// RecurseCommits runs the given function recursively over the given refs, which
// can be either commit hashes or branch names. The function accepts the current
// Commit as a parameter. Returning ErrStopRecursing from the function indicates
// that recursion should stop for the current branch, however, recursion will
// continue for any other branches until they are similarly terminated.
// Returning any other error causes recursion to stop without properly
// terminating other branches. The error will bubble to the top and be returned.
// Here's an example of printing out all of the commits reachable from a given
// set of commits:
//
// commits := []string{...}
// err := repo.RecurseCommits(commits, func(c *Commit) error {
//	sklog.Info(c.Hash)
//	return nil
// })
func (r *Graph) RecurseCommits(commits []string, f func(*Commit) error) error {
	visited := make(map[*Commit]bool, r.Len())
	for _, hash := range commits {
		c := r.Get(hash)
		if c == nil {
			return fmt.Errorf("Unknown commit %q", hash)
		}
		if !visited[c] {
			if err := c.recurse(f, visited); err != nil {
				return err
			}
		}
	}
	return nil
}

// RecurseAllBranches runs the given function recursively over the entire commit
// history, starting at each of the known branch heads. The function accepts the
// current Commit as a parameter. Returning ErrStopRecursing from the function
// indicates that recursion should stop for the current branch, however,
// recursion will continue for any other branches until they are similarly
// terminated. Returning any other error causes recursion to stop without
// properly terminating other branches. The error will bubble to the top and be
// returned. Here's an example of printing out all of the commits in the repo:
//
// repo.RecurseAllBranches(func(c *Commit) error {
//      sklog.Info(c.Hash)
//      return nil
// })
func (r *Graph) RecurseAllBranches(f func(*Commit) error) error {
	return r.RecurseCommits(r.Branches(), f)
}

// RevList is the equivalent of "git rev-list --topo-order from..to".
// Each argument is a commit hash or a branch name. The commits are returned
// in reverse topological order. Per "git rev-list" docs, the returned commit
// hashes consist of all commits reachable from "to" and not reachable from
// "from". In the typical case of linear git history, this just means the
// commits on the line after "from" up through "to", but with branches and
// merges it's possible that there are entire sub-graphs which are reachable
// from "to" but not "from".
func (r *Graph) RevList(from, to string) ([]string, error) {
	fromCommit := r.Get(from)
	if fromCommit == nil {
		return nil, fmt.Errorf("Unknown commit %q", from)
	}
	toCommit := r.Get(to)
	if toCommit == nil {
		return nil, fmt.Errorf("Unknown commit %q", to)
	}
	// Shortcut: if the commits are the same, return now.
	if fromCommit == toCommit {
		return []string{}, nil
	}

	// Find all of the excluded commits.
	exclude := make(map[*Commit]bool, r.Len())
	if err := fromCommit.recurse(func(c *Commit) error {
		return nil
	}, exclude); err != nil {
		return nil, err
	}

	// Find the included commits.
	include := make(map[*Commit]bool, r.Len())
	if err := toCommit.recurse(func(c *Commit) error {
		if exclude[c] {
			return ErrStopRecursing
		}
		return nil
	}, include); err != nil {
		return nil, err
	}

	// include may contain some commits from the exclude map; remove them.
	for c, _ := range include {
		if exclude[c] {
			delete(include, c)
		}
	}

	// Topologically sort the commits.
	sorted := topologicalSortHelper(include)
	return CommitSlice(sorted).Hashes(), nil
}

// TopologicalSort returns a slice containing the given commits in reverse
// topological order, ie. every commit is listed before any of its parents.
func TopologicalSort(commits []*Commit) []*Commit {
	commitsMap := make(map[*Commit]bool, len(commits))
	for _, c := range commits {
		commitsMap[c] = true
	}
	return topologicalSortHelper(commitsMap)
}

// Helper function for TopologicalSort; the caller provides the map, which is
// modified by topologicalSortHelper.
func topologicalSortHelper(commits map[*Commit]bool) []*Commit {
	// children maps each commit to those commits which have it as a parent.
	children := make(map[*Commit]map[*Commit]bool, len(commits))
	for c, _ := range commits {
		for _, p := range c.parents {
			if commits[p] {
				subMap, ok := children[p]
				if !ok {
					subMap = map[*Commit]bool{}
					children[p] = subMap
				}
				subMap[c] = true
			}
		}
	}

	// Sort the commits topologically.
	rv := make([]*Commit, 0, len(commits))
	followBranch := func(c *Commit) {
		for len(children[c]) == 0 {
			// Add this commit to rv.
			rv = append(rv, c)
			delete(commits, c)

			// Remove this commit from its parents' children, so
			// that they can be processed.
			for _, p := range c.parents {
				if commits[p] {
					delete(children[p], c)
				}
			}

			// Find a parent to process next.
			var next *Commit
			for _, p := range c.parents {
				if commits[p] && len(children[p]) == 0 {
					// We are ready to process this parent.
					if next == nil || p.Less(next) {
						next = p
					}
				}
			}
			if next != nil {
				c = next
			} else {
				// None of this commit's parents are ready; we can't do
				// any more.
				return
			}
		}
	}
	for len(commits) > 0 {
		var next *Commit
		for commit, _ := range commits {
			if len(children[commit]) == 0 {
				// We are ready to process this commit.
				if next == nil || commit.Less(next) {
					next = commit
				}
			}
		}
		if next == nil {
			sklog.Error("No commits are ready to process!")
			// Return so that we don't loop forever.
			return rv
		}
		followBranch(next)
	}
	return rv
}

// IsAncestor returns true iff A is an ancestor of B, where A and B are either
// commit hashes or branch names.
func (r *Graph) IsAncestor(a, b string) (bool, error) {
	aCommit := r.Get(a)
	if aCommit == nil {
		return false, fmt.Errorf("No such commit %q", a)
	}
	bCommit := r.Get(b)
	if bCommit == nil {
		return false, fmt.Errorf("No such commit %q", b)
	}
	return bCommit.HasAncestor(aCommit.Hash), nil
}

// Return any commits at or after the given timestamp.
func (r *Graph) GetCommitsNewerThan(ts time.Time) ([]*vcsinfo.LongCommit, error) {
	commits := []*Commit{}
	if err := r.RecurseAllBranches(func(c *Commit) error {
		if !c.Timestamp.Before(ts) {
			commits = append(commits, c)
			return nil
		}
		return ErrStopRecursing
	}); err != nil {
		return nil, err
	}

	// Sort the commits by timestamp, most recent first.
	sort.Sort(CommitSlice(commits))

	rv := make([]*vcsinfo.LongCommit, 0, len(commits))
	for _, c := range commits {
		rv = append(rv, c.LongCommit)
	}
	return rv, nil
}

// GetLastNCommits returns the last N commits in the repo.
func (r *Graph) GetLastNCommits(n int) ([]*vcsinfo.LongCommit, error) {
	// Find the last Nth commit on master, which we assume has far more
	// commits than any other branch.
	commit := r.Get("master")
	for i := 0; i < n-1; i++ {
		p := commit.GetParents()
		if len(p) < 1 {
			// Cut short if we've hit the beginning of history.
			break
		}
		commit = p[0]
	}
	commits, err := r.GetCommitsNewerThan(commit.Timestamp)
	if err != nil {
		return nil, err
	}

	// Return the most-recent N commits.
	if n > len(commits) {
		n = len(commits)
	}
	return commits[:n], nil
}

// Map is a convenience type for dealing with multiple Graphs for different
// repos. The keys are repository URLs.
type Map map[string]*Graph

// NewLocalMap returns a Map instance with Graphs for the given repo URLs.
// May obtain cached data from a file in the git repo, but does NOT update the
// Map; the caller is responsible for doing so before using the Map if
// up-to-date data is required.
func NewLocalMap(ctx context.Context, repos []string, workdir string) (Map, error) {
	rv := make(map[string]*Graph, len(repos))
	for _, r := range repos {
		g, err := NewLocalGraph(ctx, r, workdir)
		if err != nil {
			return nil, err
		}
		rv[r] = g
	}
	return rv, nil
}

// NewGitStoreMap returns a Map instance with Graphs for the given GitStores.
func NewGitStoreMap(ctx context.Context, repos map[string]gitstore.GitStore) (Map, error) {
	rv := make(map[string]*Graph, len(repos))
	for url, gs := range repos {
		g, err := NewGitStoreGraph(ctx, gs)
		if err != nil {
			return nil, err
		}
		rv[url] = g
	}
	return rv, nil
}

// UpdateAndReturnCommitDiffs updates all Graphs in the Map. Returns maps of
// repo URLs to slices of added commits, repo URLs to slices of removed commits,
// or any error which was encountered.
func (m Map) UpdateAndReturnCommitDiffs(ctx context.Context) (map[string][]*vcsinfo.LongCommit, map[string][]*vcsinfo.LongCommit, error) {
	// If any one update fails, we should not apply the updates for any of
	// the repos, to ensure that the caller always gets the correct lists of
	// commits.
	added := make(map[string][]*vcsinfo.LongCommit, len(m))
	removed := make(map[string][]*vcsinfo.LongCommit, len(m))
	newGraphs := make(map[string]*Graph, len(m))
	for repoUrl, g := range m {
		g.updateMtx.Lock()
		defer g.updateMtx.Unlock()
		newGraph, err := g.UpdateCopy(ctx)
		if err != nil {
			return nil, nil, err
		}
		a, r := g.DiffCommits(newGraph)
		added[repoUrl] = a
		removed[repoUrl] = r
		newGraphs[repoUrl] = newGraph
	}
	for repoUrl, g := range m {
		g.ReplaceContents(newGraphs[repoUrl])
	}
	return added, removed, nil

}

// Update updates all Graphs in the Map.
func (m Map) Update(ctx context.Context) error {
	for _, g := range m {
		if err := g.Update(ctx); err != nil {
			return err
		}
	}
	return nil
}

// FindCommit returns the Commit object, repo URL, and Graph object for the
// given commit hash if it exists in any of the Graphs in the Map and an error
// otherwise.
func (m Map) FindCommit(commit string) (*Commit, string, *Graph, error) {
	for name, repo := range m {
		c := repo.Get(commit)
		if c != nil {
			return c, name, repo, nil
		}
	}
	return nil, "", nil, fmt.Errorf("Unable to find commit %s in any repo.", commit)
}

// RepoURLs returns the list of repositories in the Map.
func (m Map) RepoURLs() []string {
	rv := make([]string, 0, len(m))
	for r := range m {
		rv = append(rv, r)
	}
	return rv
}
