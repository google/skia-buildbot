package incremental

import (
	"fmt"
	"sync"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/task_scheduler/go/window"
)

// commitsCache is a struct used for tracking newly-landed commits.
type commitsCache struct {
	mtx            sync.Mutex
	oldBranchHeads map[string][]*gitinfo.GitBranch
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
// that repo, or if reset is true.
func (c *commitsCache) Update(w *window.Window, reset bool, n int) (map[string][]*gitinfo.GitBranch, map[string][]*vcsinfo.LongCommit, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if err := c.repos.Update(); err != nil {
		return nil, nil, fmt.Errorf("Failed to update commitsCache; failed to update repos: %s", err)
	}
	branchHeads := make(map[string][]*gitinfo.GitBranch, len(c.repos))
	rvCommits := make(map[string][]*vcsinfo.LongCommit, len(c.repos))
	rvBranchHeads := make(map[string][]*gitinfo.GitBranch, len(c.repos))
	for repoUrl, repo := range c.repos {
		bh := repo.BranchHeads()
		branchHeads[repoUrl] = bh
		var newCommits []*vcsinfo.LongCommit
		var err error
		if reset {
			newCommits, err = repo.GetLastNCommits(n)
		} else {
			newCommits, err = repo.GetNewCommits(c.oldBranchHeads[repoUrl])
		}
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to update commitsCache; failed to obtain commits: %s", err)
		}
		if reset || len(newCommits) > 0 {
			rvCommits[repoUrl] = newCommits
			rvBranchHeads[repoUrl] = bh
		}
	}
	c.oldBranchHeads = branchHeads
	return rvBranchHeads, rvCommits, nil
}
