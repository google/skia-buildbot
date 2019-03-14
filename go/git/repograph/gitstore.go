package repograph

import (
	"context"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

func (r *Graph) updateGitstore(ctx context.Context, returnNewCommits bool) ([]*vcsinfo.LongCommit, error) {
	r.gitstoreMtx.Lock()
	defer r.gitstoreMtx.Unlock()
	now := time.Now()
	from := r.gitstoreLastUpdate.Add(-10 * time.Minute)
	indexCommits, err := r.gitstore.RangeByTime(ctx, from, now.Add(time.Second), "")
	if err != nil {
		return nil, err
	}
	sklog.Infof("Got %d IndexCommits", len(indexCommits))
	hashes := make([]string, 0, len(indexCommits))
	for _, c := range indexCommits {
		hashes = append(hashes, c.Hash)
	}
	commits, err := r.gitstore.Get(ctx, hashes)
	if err != nil {
		return nil, err
	}
	commitsMap := make(map[string]*vcsinfo.LongCommit, len(commits))
	for _, c := range commits {
		commitsMap[c.Hash] = c
	}
	newBranches, err := r.gitstore.GetBranches(ctx)
	if err != nil {
		return nil, err
	}
	// Ignore the empty branch, which contains all commits.
	delete(newBranches, "")
	r.graphMtx.Lock()
	defer r.graphMtx.Unlock()
	oldBranches := make(map[string]string, len(r.branches))
	for _, bh := range r.branches {
		oldBranches[bh.Name] = bh.Head
	}

	var newCommits []*vcsinfo.LongCommit
	for branchName, newBranch := range newBranches {
		oldBranchHead := oldBranches[branchName]

		// Shortcut: if the branch is up-to-date, skip it.
		if newBranch.Head == oldBranchHead {
			continue
		}

		// Trace back in time from the new branch head until we find the
		// old branch head, any other commit we already have, or a
		// commit with no parents.
		toProcess := map[string]bool{newBranch.Head: true}
		for len(toProcess) > 0 {
			// Choose a commit to process.
			var c string
			for commit, _ := range toProcess {
				c = commit
				break
			}
			delete(toProcess, c)

			// If we've seen this commit before, we're done.
			if c == oldBranchHead {
				continue
			}
			if _, ok := r.commits[c]; ok {
				continue
			}

			// We haven't seen this commit before; add it to newCommits.
			details, ok := commitsMap[c]
			if !ok {
				sklog.Warningf("Commit %q not found in results; performing explicit lookup.", c)
				detailsList, err := r.gitstore.Get(ctx, []string{c})
				if err != nil {
					return nil, err
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
		}
	}

	// Add newCommits in reverse order to ensure that all parents are added
	// before their children.
	for i := len(newCommits) - 1; i >= 0; i-- {
		commit := newCommits[i]
		if _, ok := r.commits[commit.Hash]; !ok {
			if err := r.addCommit(ctx, commit); err != nil {
				return nil, err
			}
		}
	}

	// Update the rest of the Graph.
	r.gitstoreLastUpdate = now
	r.branches = make([]*git.Branch, 0, len(newBranches))
	for name, bh := range newBranches {
		r.branches = append(r.branches, &git.Branch{
			Name: name,
			Head: bh.Head,
		})
	}
	return newCommits, nil
}
