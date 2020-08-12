package snapshots

import (
	"fmt"
	"sort"
	"time"

	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

type RepoQueryFunc func() []*vcsinfo.LongCommit

func LastNQueryFunc(repos repograph.Map, repo, branch string, n int) (string, RepoQueryFunc) {
	graph := repos[repo]
	return fmt.Sprintf("LastNQuery(%q, %q, %d)", repo, branch, n), func() []*vcsinfo.LongCommit {
		// This follows the flow of repograph.Graph.GetLastNCommits but
		// supports finding commits on all branches or just one.
		head := graph.Get(branch)
		if branch == "" {
			// No specified branch means all branches. Start with
			// master to find the time range.
			// TODO(borenet): This could return incorrect results
			// if there are fewer than N commits on master.
			head = graph.Get("master")
		}
		if head == nil {
			// TODO(borenet): Should we 404 instead?
			return nil
		}

		// Trace back until we've found N commits.
		count := 0
		var nth *repograph.Commit
		if err := head.RecurseFirstParent(func(c *repograph.Commit) error {
			count++
			nth = c
			if count >= n {
				return repograph.ErrStopRecursing
			}
			return nil
		}); err != nil {
			// TODO(borenet): This shouldn't happen, but we should probably handle it.
			sklog.Fatal(err)
		}

		// Find all commits newer than the Nth found above.
		commits := make([]*repograph.Commit, 0, n)
		fn := head.Recurse
		if branch == "" {
			fn = graph.RecurseAllBranches
		}
		if err := fn(func(c *repograph.Commit) error {
			if !c.Timestamp.After(nth.Timestamp) {
				commits = append(commits, c)
			}
		}); err != nil {
			// TODO(borenet): This shouldn't happen, but we should probably handle it.
			sklog.Fatal(err)
		}

		// Sort commits by time and return the last N.
		sort.Sort(repograph.CommitSlice(commits))
		rv := make([]*vcsinfo.LongCommit, 0, n)
		for i, c := range commits {
			if i == n {
				break
			}
			rv = append(rv, c.LongCommit)
		}
		return rv
	}
}

// start is inclusive, end exclusive
func TimeRangeQueryFunc(repos repograph.Map, repo, branch string, start, end time.Time) (string, RepoQueryFunc) {
	graph := repos[repo]
	// If the end of the time range is before the current time, then we can
	// cache the results rather than re-running the query every time.
	endIsBeforeCurrentTime := end.Before(time.Now())
	var cached []*vcsinfo.LongCommit
	return fmt.Sprintf("TimeRangeQueryFunc(%q, %q, %d, %d)", repo, branch, start.UnixNano(), end.UnixNano()), func() []*vcsinfo.LongCommit {
		if endIsBeforeCurrentTime && cached != nil {
			return cached
		}
		if head := graph.Get(branch); head == nil && branch != "" {
			// TODO(borenet): Should we 404 intead?
			return nil
		}
		commits := []*repograph.Commit{}
		fn := func(c *repograph.Commit) error {
			if c.Timestamp.Before(start) {
				return repograph.ErrStopRecursing
			}
			if c.Timestamp.Before(end) {
				commits = append(commits, c)
			}
			return nil
		}
		var err error
		if branch == "" {
			err = graph.RecurseAllBranches(fn)
		} else {
			err = graph.Get(branch).Recurse(fn)
		}
		if err != nil {
			// TODO(borenet): This shouldn't happen, but we should probably handle it.
			sklog.Fatal(err)
		}
		sort.Sort(repograph.CommitSlice(commits))
		rv := make([]*vcsinfo.LongCommit, 0, len(commits))
		for _, c := range commits {
			rv = append(rv, c.LongCommit)
		}
		if endIsBeforeCurrentTime {
			cached = rv
		}
		return rv
	}
}

func CommitRangeQueryFunc(repos repograph.Map, repo, from, to string) (string, RepoQueryFunc) {
	graph := repos[repo]
	// If both from and to are commit hashes which already exist in the
	// graph, we can perform the query once and simply cache the results.
	var cached []*vcsinfo.LongCommit
	var cachedFrom string
	var cachedTo string
	return fmt.Sprintf("CommitRangeQueryFunc(%q, %q, %q)", repo, from, to), func() []*vcsinfo.LongCommit {
		// Resolve from and to. If the hashes are the same as last time,
		// just return the previous list of commits.
		fromHash := graph.Get(from).Hash // TODO: What if this is nil?
		toHash := graph.Get(to).Hash     // TODO: What if this is nil?
		if !(cachedFrom == fromHash && cachedTo == toHash) {
			hashes, err := graph.RevList(fromHash, toHash)
			if err != nil {
				// TODO(borenet): Is there a better way to handle this?
				sklog.Errorf("Failed to RevList(%q, %q): %s", from, to, err)
				return nil
			}
			commits := make([]*vcsinfo.LongCommit, 0, len(hashes))
			for _, hash := range hashes {
				commits = append(commits, graph.Get(hash).LongCommit)
			}
			cached = commits
			cachedFrom = fromHash
			cachedTo = toHash
		}
		return cached
	}
}
