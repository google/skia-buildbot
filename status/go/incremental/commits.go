package incremental

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/task_scheduler/go/window"
)

// commitsCache is a struct used for tracking newly-landed commits.
type commitsCache struct {
	mtx sync.Mutex
	// map[repo URL][branch name]*git.Branch
	oldBranchHeads map[string]map[string]*git.Branch
	repos          repograph.Map
}

// newCommitsCache returns a commitsCache instance.
func newCommitsCache(repos repograph.Map) *commitsCache {
	return &commitsCache{
		repos: repos,
	}
}

// Return any new commits for each repo, or the last N if reset is true. Branch
// heads will be provided for a given repo only if there are new commits for
// that repo, or if reset is true. The returned commits may be in any order and
// are not sorted by timestamp.
func (c *commitsCache) Update(ctx context.Context, w window.Window, reset bool, n int) (map[string][]*git.Branch, map[string][]*vcsinfo.LongCommit, error) {
	defer metrics2.FuncTimer().Stop()
	c.mtx.Lock()
	defer c.mtx.Unlock()

	newCommitsAllRepos, _, err := c.repos.UpdateAndReturnCommitDiffs(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to update commitsCache; failed to update repos: %s", err)
	}
	updatedBranchHeads := make(map[string]map[string]*git.Branch, len(c.repos))
	rvCommits := make(map[string][]*vcsinfo.LongCommit, len(c.repos))
	rvBranchHeads := make(map[string][]*git.Branch, len(c.repos))
	for repoUrl, repo := range c.repos {
		// Update the branch heads for this repo.
		bh := repo.BranchHeads()
		bhMap := make(map[string]*git.Branch, len(bh))
		for _, h := range bh {
			bhMap[h.Name] = h
		}
		updatedBranchHeads[repoUrl] = bhMap

		newCommits := newCommitsAllRepos[repoUrl]

		// If reset is specified, we don't care about changes; we return
		// all commits in range.
		if reset {
			var err error
			newCommits, err = repo.GetLastNCommits(n)
			if err != nil {
				return nil, nil, fmt.Errorf("Failed to update commitsCache; failed to obtain commits from %s: %s", repoUrl, err)
			}
		}
		// Add any new commits to the return value. The branch heads get
		// updated if there are any new commits OR if the branch heads
		// have changed (eg. in the case of a reset or empty merge).
		if len(newCommits) > 0 {
			// Only add new commits which are in the window.
			filtered := make([]*vcsinfo.LongCommit, 0, len(newCommits))
			for _, c := range newCommits {
				if w.TestTime(repoUrl, c.Timestamp) {
					filtered = append(filtered, c)
				}
			}
			rvCommits[repoUrl] = filtered
			rvBranchHeads[repoUrl] = bh
		} else if !reflect.DeepEqual(c.oldBranchHeads[repoUrl], bhMap) {
			rvBranchHeads[repoUrl] = bh
		}
	}
	c.oldBranchHeads = updatedBranchHeads
	return rvBranchHeads, rvCommits, nil
}
