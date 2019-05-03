package repograph

/*
   The repograph package provides an in-memory representation of an entire Git repo.
*/

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
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

// RepoImpl provides methods for interacting with a git repo, agnostic of how the
// repo is actually accessed. It is used when updating a Graph. It should not be
// used concurrently.
type RepoImpl interface {
	// Update the local view of the repo.
	Update(context.Context) error

	// Return the given commits.
	Get(context.Context, []string) ([]*vcsinfo.LongCommit, error)

	// Return the branch heads, as of the last call to Update().
	Branches(context.Context) ([]*git.Branch, error)

	// UpdateCallback is a function which is called after the Graph is
	// updated but before the changes are committed.
	UpdateCallback(context.Context, *Graph) error
}

// Graph represents an entire Git repo.
type Graph struct {
	branches []*git.Branch
	commits  map[string]*Commit
	graphMtx sync.RWMutex

	updateMtx sync.Mutex
	repoImpl  RepoImpl
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
		commits:  map[string]*Commit{},
		repoImpl: &localRepoImpl{repo},
	}
	if err := initFromFile(rv, path.Join(repo.Dir(), CACHE_FILE)); err != nil {
		return nil, err
	}
	return rv, nil
}

// NewGitStoreGraph returns a Graph instance which is backed by a GitStore.
func NewGitStoreGraph(ctx context.Context, gs gitstore.GitStore) (*Graph, error) {
	return NewWithRepoImpl(ctx, &gitstoreRepoImpl{
		gs: gs,
	})
}

// NewWithRepoImpl returns a Graph instance which uses the given RepoImpl.
func NewWithRepoImpl(ctx context.Context, ri RepoImpl) (*Graph, error) {
	rv := &Graph{
		commits:  map[string]*Commit{},
		repoImpl: ri,
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
	defer metrics2.FuncTimer().Stop()
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

// updateFrom updates the Graph using the given RepoImpl and returns the lists of
// new and deleted commits, or any error which occurred.
func (r *Graph) updateFrom(ctx context.Context, ri RepoImpl) ([]*vcsinfo.LongCommit, []*vcsinfo.LongCommit, error) {
	// Retrieve the new commits.
	sklog.Info("Updating repograph.Graph...")
	if err := ri.Update(ctx); err != nil {
		return nil, nil, fmt.Errorf("Failed RepoImpl.Update(): %s", err)
	}

	// Obtain the list of branches.
	sklog.Info("  Getting branches...")
	newBranchesList, err := ri.Branches(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to obtain branch list from RepoImpl: %s", err)
	}
	newBranchesMap := make(map[string]string, len(newBranchesList))
	for _, branch := range newBranchesList {
		newBranchesMap[branch.Name] = branch.Head
	}
	sort.Sort(git.BranchList(newBranchesList))
	r.graphMtx.Lock()
	defer r.graphMtx.Unlock()
	oldBranchesMap := make(map[string]string, len(r.branches))
	for _, branch := range r.branches {
		oldBranchesMap[branch.Name] = branch.Head
	}

	// Load new commits.
	var newCommits []*vcsinfo.LongCommit
	sklog.Infof("  Loading commits...")
	defer sklog.Infof("Done loading repograph.Graph")

	needOrphanCheck := false
	for _, branch := range newBranchesList {
		newHead := newBranchesMap[branch.Name]
		oldHead := oldBranchesMap[branch.Name]

		// Shortcut: if the branch is up-to-date, skip it.
		if newHead == oldHead {
			continue
		}

		// Trace back in time from the new branch head until we find the
		// old branch head, any other commit we already have, or a
		// commit with no parents.
		toProcess := map[string]bool{newHead: true}
		for len(toProcess) > 0 {
			// Choose a commit to process.
			var c string
			for commit := range toProcess {
				c = commit
				break
			}
			delete(toProcess, c)

			// If we've seen this commit before, we're done.
			if c == oldHead {
				continue
			}
			if _, ok := r.commits[c]; ok {
				if oldHead != "" {
					// If we found a previously-known commit before
					// we found the old branch head, then history
					// has changed and we need to run the orphan
					// check.
					needOrphanCheck = true
				}
				continue
			}

			// We haven't seen this commit before; add it to newCommits.
			details, err := ri.Get(ctx, []string{c})
			if err != nil {
				return nil, nil, fmt.Errorf("Failed to Get commit details from RepoImpl: %s", err)
			}
			newCommits = append(newCommits, details[0])

			// Add the commit's parent(s) to the toProcess map.
			for _, p := range details[0].Parents {
				toProcess[p] = true
			}
			if len(details[0].Parents) == 0 && oldHead != "" {
				// If we found a commit with no parents and this
				// is not a new branch, then we've discovered a
				// completely new line of history and need to
				// check whether the commits on the old line are
				// now orphaned.
				needOrphanCheck = true
			}
		}
	}

	// Add newCommits in reverse order to ensure that all parents are added
	// before their children.
	sklog.Info("  Adding commits...")
	addedCommits := make([]*vcsinfo.LongCommit, 0, len(newCommits))
	for i := len(newCommits) - 1; i >= 0; i-- {
		commit := newCommits[i]
		if _, ok := r.commits[commit.Hash]; !ok {
			if err := r.addCommit(commit); err != nil {
				return nil, nil, fmt.Errorf("Failed to add commit: %s", err)
			}
			addedCommits = append(addedCommits, commit)
		}
	}

	if !needOrphanCheck {
		// Check to see whether any branches were deleted.
		for branch := range oldBranchesMap {
			if _, ok := newBranchesMap[branch]; !ok {
				needOrphanCheck = true
				break
			}
		}
	}
	var removedCommits []*vcsinfo.LongCommit
	if needOrphanCheck {
		sklog.Warningf("History change detected; checking for orphaned commits.")
		visited := make(map[*Commit]bool, len(r.commits))
		for _, newBranchHead := range newBranchesMap {
			// Not using Get() because graphMtx is locked.
			if err := r.commits[newBranchHead].recurse(func(c *Commit) error {
				return nil
			}, visited); err != nil {
				return nil, nil, fmt.Errorf("Failed to trace commit history during orphan check: %s", err)
			}
		}
		for hash, c := range r.commits {
			if !visited[c] {
				sklog.Warningf("Commit %s is orphaned. Removing from the Graph.", hash)
				delete(r.commits, hash)
				removedCommits = append(removedCommits, c.LongCommit)
			}
		}
	}

	// Update the rest of the Graph.
	r.branches = newBranchesList
	sklog.Info("  Finished update.")
	return addedCommits, removedCommits, nil
}

// update the Graph, returning any added and removed commits, and run the given
// callback function on the Graph before the changes are committed. If the
// callback returns an error, the changes are not committed.
func (r *Graph) update(ctx context.Context, cb func(*Graph) error) ([]*vcsinfo.LongCommit, []*vcsinfo.LongCommit, error) {
	r.updateMtx.Lock()
	defer r.updateMtx.Unlock()
	defer metrics2.FuncTimer().Stop()
	newGraph := r.shallowCopy()
	added, removed, err := newGraph.updateFrom(ctx, r.repoImpl)
	if err != nil {
		return nil, nil, err
	}
	if cb != nil {
		if err := cb(newGraph); err != nil {
			return nil, nil, err
		}
	}
	if err := r.repoImpl.UpdateCallback(ctx, newGraph); err != nil {
		return nil, nil, err
	}
	r.graphMtx.Lock()
	defer r.graphMtx.Unlock()
	r.branches = newGraph.branches
	r.commits = newGraph.commits
	return added, removed, nil
}

// Update the Graph.
func (r *Graph) Update(ctx context.Context) error {
	_, _, err := r.update(ctx, nil)
	return err
}

// UpdateAndReturnCommitDiffs updates the Graph and returns the added and
// removed commits, in arbitrary order.
func (r *Graph) UpdateAndReturnCommitDiffs(ctx context.Context) ([]*vcsinfo.LongCommit, []*vcsinfo.LongCommit, error) {
	return r.update(ctx, nil)
}

// UpdateWithCallback updates the Graph and runs the given function before the
// changes are committed. If the function returns an error, the changes are not
// committed. The function is allowed to call non-Update methods of the Graph.
func (r *Graph) UpdateWithCallback(ctx context.Context, cb func(*Graph) error) error {
	_, _, err := r.update(ctx, cb)
	return err
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

// shallowCopy() returns a shallow copy of the Graph, ie. the pointers in the
// old and new Graphs will remain equal for a given Commit.
func (r *Graph) shallowCopy() *Graph {
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
	for c := range include {
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
	for c := range commits {
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
		for commit := range commits {
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
func NewBTGitStoreMap(ctx context.Context, repoUrls []string, btConf *bt_gitstore.BTConfig) (Map, error) {
	rv := make(map[string]*Graph, len(repoUrls))
	for _, repoUrl := range repoUrls {
		gs, err := bt_gitstore.New(ctx, btConf, repoUrl)
		if err != nil {
			return nil, fmt.Errorf("Failed to create GitStore for %s: %s", repoUrl, err)
		}
		graph, err := NewGitStoreGraph(ctx, gs)
		if err != nil {
			return nil, fmt.Errorf("Failed to create Graph from GitStore for %s: %s", repoUrl, err)
		}
		rv[repoUrl] = graph
	}
	return rv, nil
}

// update the Graphs in the Map, returning any added and removed commits, and
// run the given callback function on the Graphs before the changes are
// committed. If the callback returns an error for any Graph, the changes are
// not committed.
func (m Map) update(ctx context.Context, cb func(string, *Graph) error) (map[string][]*vcsinfo.LongCommit, map[string][]*vcsinfo.LongCommit, error) {
	added := make(map[string][]*vcsinfo.LongCommit, len(m))
	removed := make(map[string][]*vcsinfo.LongCommit, len(m))
	newGraphs := make(map[string]*Graph, len(m))
	for repoUrl, r := range m {
		r.updateMtx.Lock()
		defer r.updateMtx.Unlock()
		newGraph := r.shallowCopy()
		a, r, err := newGraph.updateFrom(ctx, r.repoImpl)
		if err != nil {
			return nil, nil, err
		}
		added[repoUrl] = a
		removed[repoUrl] = r
		newGraphs[repoUrl] = newGraph
		if cb != nil {
			if err := cb(repoUrl, newGraph); err != nil {
				return nil, nil, err
			}
		}
	}
	for repoUrl, r := range m {
		r.graphMtx.Lock()
		defer r.graphMtx.Unlock()
		newGraph := newGraphs[repoUrl]
		r.branches = newGraph.branches
		r.commits = newGraph.commits
	}
	return added, removed, nil
}

// Update all Graphs in the Map.
func (m Map) Update(ctx context.Context) error {
	_, _, err := m.update(ctx, nil)
	return err
}

// UpdateAndReturnCommitDiffs updates all Graphs in the Map. Returns maps of
// repo URLs to slices of added commits, repo URLs to slices of removed commits,
// or any error which was encountered. If any Graph failed to update, no changes
// are committed.
func (m Map) UpdateAndReturnCommitDiffs(ctx context.Context) (map[string][]*vcsinfo.LongCommit, map[string][]*vcsinfo.LongCommit, error) {
	return m.update(ctx, nil)
}

// UpdateWithCallback updates the Graphs in the Map and runs the given function
// on each Graph before the changes are committed. If the function returns an
// error for any Graph, no changes are committed. The function is allowed to
// call non-Update methods of the Graph.
func (m Map) UpdateWithCallback(ctx context.Context, cb func(string, *Graph) error) error {
	_, _, err := m.update(ctx, cb)
	return err
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
