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

const (
	// Name of the file we store inside the Git checkout to speed up the
	// initial Update().
	CACHE_FILE = "sk_gitrepo.gob"
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

// Graph represents an entire Git repo.
type Graph struct {
	branches []*git.Branch
	commits  map[string]*Commit
	graphMtx sync.RWMutex

	// repo is only set if NewLocalGraph() is used.
	repo    *git.Repo
	repoMtx sync.Mutex

	// gitstore is only set if NewGitStoreGraph() is used.
	gitstore           gitstore.GitStore
	gitstoreLastUpdate time.Time
	gitstoreMtx        sync.Mutex
}

// gobGraph is a utility struct used for serializing a Graph using gob.
type gobGraph struct {
	Branches []*git.Branch
	Commits  map[string]*Commit
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
		repo:    repo,
	}
	cacheFile := path.Join(repo.Dir(), CACHE_FILE)
	var r gobGraph
	if err := util.MaybeReadGobFile(cacheFile, &r); err != nil {
		sklog.Errorf("Failed to read Graph cache file %s; deleting the file and starting from scratch: %s", cacheFile, err)
		if err2 := os.Remove(cacheFile); err != nil {
			return nil, fmt.Errorf("Failed to read Graph cache file %s: %s\n...and failed to remove with: %s", cacheFile, err, err2)
		}
	}
	if r.Branches != nil {
		rv.branches = r.Branches
	}
	if r.Commits != nil {
		rv.commits = r.Commits
	}
	for _, c := range rv.commits {
		for _, parentHash := range c.Parents {
			c.parents = append(c.parents, rv.commits[parentHash])
		}
	}
	return rv, nil
}

// NewGitStoreGraph returns a Graph instance which is backed by a GitStore.
func NewGitStoreGraph(ctx context.Context, gs gitstore.GitStore) (*Graph, error) {
	rv := &Graph{
		commits:  map[string]*Commit{},
		gitstore: gs,
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

// addCommit adds the commit with the given hash to the Graph. Assumes that the
// caller holds r.graphMtx.
func (r *Graph) addCommit(d *vcsinfo.LongCommit) error {
	maxParentHistoryLen := 0
	var parents []*Commit
	if len(d.Parents) > 0 {
		for _, h := range d.Parents {
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
		LongCommit: d,
		parents:    parents,
		HistoryLen: maxParentHistoryLen + 1,
	}
	r.commits[c.Hash] = c
	return nil
}

// update the Graph.
func (r *Graph) update(ctx context.Context, returnNewCommits bool) ([]*vcsinfo.LongCommit, error) {
	if r.repo != nil {
		r.repoMtx.Lock()
		defer r.repoMtx.Unlock()
		return r.UpdateFromRepo(ctx, r.repo, returnNewCommits)
	}
	return r.UpdateFromGitStore(ctx, r.gitstore, returnNewCommits)
}

// Update syncs the local copy of the repo and loads new commits/branches into
// the Graph object.
func (r *Graph) Update(ctx context.Context) error {
	_, err := r.update(ctx, false)
	return err
}

// UpdateAndReturnNewCommits syncs the local copy of the repo and loads new
// commits/branches into the Graph object. Returns a slice of any new commits,
// in no particular order.
func (r *Graph) UpdateAndReturnNewCommits(ctx context.Context) ([]*vcsinfo.LongCommit, error) {
	return r.update(ctx, true)
}

func updateFromRepo(ctx context.Context, repo *git.Repo, graph *Graph) ([]*vcsinfo.LongCommit, error) {
	// Update the local copy.
	sklog.Infof("Updating repograph.Graph...")
	if err := repo.Update(ctx); err != nil {
		return nil, fmt.Errorf("Failed to update repograph.Graph: %s", err)
	}

	// Obtain the list of branches.
	sklog.Info("  Getting branches...")
	newBranchesList, err := repo.Branches(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to get branches for repograph.Graph: %s", err)
	}
	newBranchesMap := make(map[string]string, len(newBranchesList))
	for _, branch := range newBranchesList {
		newBranchesMap[branch.Name] = branch.Head
	}
	graph.graphMtx.Lock()
	defer graph.graphMtx.Unlock()
	oldBranchesMap := make(map[string]string, len(graph.branches))
	for _, branch := range graph.branches {
		oldBranchesMap[branch.Name] = branch.Head
	}

	// Load new commits from the repo.
	var newCommits []*vcsinfo.LongCommit
	sklog.Infof("  Loading commits...")
	needOrphanCheck := false
	for _, branch := range newBranchesList {
		newHead := newBranchesMap[branch.Name]
		oldHead := oldBranchesMap[branch.Name]

		// Shortcut: if the branch is up-to-date, skip it.
		if newHead == oldHead {
			continue
		}

		// Load all commits on this branch.
		// First, try to load only new commits on this branch.
		var commits []string
		newBranch := true
		if oldHead != "" {
			anc, err := repo.IsAncestor(ctx, oldHead, newHead)
			if err != nil {
				return nil, err
			}
			if anc {
				commits, err = repo.RevList(ctx, "--topo-order", fmt.Sprintf("%s..%s", oldHead, newHead))
				if err != nil {
					return nil, err
				}
				newBranch = false
			} else {
				needOrphanCheck = true
			}
		}

		// If this is a new branch, or if the old branch head is not
		// reachable from the new (eg. if commit history was modified),
		// load ALL commits reachable from the branch head.
		if newBranch {
			sklog.Infof("  Branch %s is new or its history has changed; loading all commits.", branch.Name)
			commits, err = repo.RevList(ctx, "--topo-order", newHead)
			if err != nil {
				return nil, fmt.Errorf("Failed to 'git rev-list' for repograph.Graph: %s", err)
			}
		}
		for i := len(commits) - 1; i >= 0; i-- {
			hash := commits[i]
			if hash == "" {
				continue
			}
			if _, ok := graph.commits[hash]; ok {
				continue
			}
			d, err := repo.Details(ctx, hash)
			if err != nil {
				return nil, fmt.Errorf("repograph.Graph: Failed to obtain Git commit details: %s", err)
			}
			if err := graph.addCommit(d); err != nil {
				return nil, err
			}
			newCommits = append(newCommits, d)
		}
	}
	if !needOrphanCheck {
		// Check to see whether any branches were deleted.
		for branch, _ := range oldBranchesMap {
			if _, ok := newBranchesMap[branch]; !ok {
				needOrphanCheck = true
				break
			}
		}
	}
	if needOrphanCheck {
		sklog.Warningf("History change detected; checking for orphaned commits.")
		visited := make(map[*Commit]bool, len(graph.commits))
		for _, newBranchHead := range newBranchesMap {
			// Not using Get() because graphMtx is locked.
			if err := graph.commits[newBranchHead].recurse(func(c *Commit) error {
				return nil
			}, visited); err != nil {
				return nil, err
			}
		}
		for hash, c := range graph.commits {
			if !visited[c] {
				sklog.Warningf("Commit %s is orphaned. Removing from the Graph.", hash)
				delete(graph.commits, hash)
			}
		}
	}

	graph.branches = newBranchesList
	return newCommits, nil
}

// Write the Graph to the cache file in the given Repo.
func (r *Graph) writeCacheFile(repo *git.Repo) error {
	sklog.Infof("  Writing cache file...")
	cacheFile := path.Join(repo.Dir(), CACHE_FILE)
	return util.WithWriteFile(cacheFile, func(w io.Writer) error {
		return gob.NewEncoder(w).Encode(gobGraph{
			Branches: r.branches,
			Commits:  r.commits,
		})
	})
}

// UpdateFromRepo updates the Graph from a local git.Repo.
func (r *Graph) UpdateFromRepo(ctx context.Context, repo *git.Repo, returnNewCommits bool) ([]*vcsinfo.LongCommit, error) {
	newGraph := r.ShallowCopy()
	newCommits, err := updateFromRepo(ctx, repo, newGraph)
	if err != nil {
		return nil, err
	}
	if err := newGraph.writeCacheFile(repo); err != nil {
		return nil, fmt.Errorf("Failed to write cache file with: %s", err)
	}
	r.graphMtx.Lock()
	defer r.graphMtx.Unlock()
	r.branches = newGraph.branches
	r.commits = newGraph.commits
	sklog.Infof("  Done. Graph has %d commits.", len(r.commits))
	return newCommits, nil
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

// Update updates all Graphs in the Map.
func (m Map) Update(ctx context.Context) error {
	_, err := m.update(ctx, false)
	return err
}

// UpdateAndReturnNewCommits updates all Graphs in the Map. Returns a map of
// repo URLs to slices of new commits in each of the repos.
func (m Map) UpdateAndReturnNewCommits(ctx context.Context) (map[string][]*vcsinfo.LongCommit, error) {
	return m.update(ctx, true)
}

// Update updates all Graphs in the Map. Returns a map of repo URLs to slices of
// new commits in each of the repos.
func (m Map) update(ctx context.Context, returnNewCommits bool) (map[string][]*vcsinfo.LongCommit, error) {
	newCommits := make(map[string][]*vcsinfo.LongCommit, len(m))
	for repoUrl, g := range m {
		commits, err := g.update(ctx, returnNewCommits)
		if err != nil {
			return nil, err
		}
		newCommits[repoUrl] = commits
	}
	return newCommits, nil
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
