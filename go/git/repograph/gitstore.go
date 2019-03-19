package repograph

import (
	"context"
	"sort"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

type gitstoreUpdater struct {
	gs         gitstore.GitStore
	lastUpdate time.Time
}

func (g *gitstoreUpdater) Update(ctx context.Context, graph *Graph) error {
	// Retrieve the new commits.
	sklog.Info("Updating repograph.Graph...")
	from := g.lastUpdate.Add(-10 * time.Minute)
	now := time.Now()
	to := now.Add(time.Second)
	indexCommits, err := g.gs.RangeByTime(ctx, from, to, "")
	if err != nil {
		return err
	}
	hashes := make([]string, 0, len(indexCommits))
	for _, c := range indexCommits {
		hashes = append(hashes, c.Hash)
	}
	commits, err := g.gs.Get(ctx, hashes)
	if err != nil {
		return err
	}
	commitsMap := make(map[string]*vcsinfo.LongCommit, len(commits))
	for _, c := range commits {
		commitsMap[c.Hash] = c
	}

	// Obtain the list of branches.
	sklog.Info("  Getting branches...")
	newBranches, err := g.gs.GetBranches(ctx)
	if err != nil {
		return err
	}
	// Ignore the empty branch, which contains all commits.
	delete(newBranches, "")
	newBranchesList := make([]string, 0, len(newBranches))
	newBranchesMap := make(map[string]string, len(newBranches))
	for branch, bh := range newBranches {
		newBranchesList = append(newBranchesList, branch)
		newBranchesMap[branch] = bh.Head
	}
	sort.Strings(newBranchesList)
	graph.graphMtx.Lock()
	defer graph.graphMtx.Unlock()
	oldBranchesMap := make(map[string]string, len(graph.branches))
	for _, branch := range graph.branches {
		oldBranchesMap[branch.Name] = branch.Head
	}

	// Load new commits.
	var newCommits []*vcsinfo.LongCommit
	sklog.Infof("  Loading commits...")
	needOrphanCheck := false
	for _, branchName := range newBranchesList {
		newHead := newBranchesMap[branchName]
		oldHead := oldBranchesMap[branchName]

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
			for commit, _ := range toProcess {
				c = commit
				break
			}
			delete(toProcess, c)

			// If we've seen this commit before, we're done.
			if c == oldHead {
				continue
			}
			if _, ok := graph.commits[c]; ok {
				// If we found a previously-known commit before
				// we found the old branch head, then history
				// has changed and we need to run the orphan
				// check.
				needOrphanCheck = true
				continue
			}

			// We haven't seen this commit before; add it to newCommits.
			details, ok := commitsMap[c]
			if !ok {
				sklog.Warningf("Commit %q not found in results; performing explicit lookup.", c)
				detailsList, err := g.gs.Get(ctx, []string{c})
				if err != nil {
					return err
				}
				details = detailsList[0]
				// Write the details back to commitsMap in case
				// this commit is needed by other branches.
				commitsMap[c] = details
			}
			newCommits = append(newCommits, details)

			// Add the commit's parent(s) to the toProcess map.
			for _, p := range details.Parents {
				toProcess[p] = true
			}
			if len(details.Parents) == 0 && oldHead != "" {
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
	for i := len(newCommits) - 1; i >= 0; i-- {
		commit := newCommits[i]
		if _, ok := graph.commits[commit.Hash]; !ok {
			if err := graph.addCommit(commit); err != nil {
				return err
			}
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
				return err
			}
		}
		for hash, c := range graph.commits {
			if !visited[c] {
				sklog.Warningf("Commit %s is orphaned. Removing from the Graph.", hash)
				delete(graph.commits, hash)
			}
		}
	}

	// Update the rest of the Graph.
	graph.branches = make([]*git.Branch, 0, len(newBranchesMap))
	for name, head := range newBranchesMap {
		graph.branches = append(graph.branches, &git.Branch{
			Name: name,
			Head: head,
		})
	}
	sort.Slice(graph.branches, func(i, j int) bool {
		return graph.branches[i].Name < graph.branches[j].Name
	})
	return nil
}
