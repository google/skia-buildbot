package commit_cache

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/golang/glog"

	"skia.googlesource.com/buildbot.git/go/gitinfo"
)

/*
	Utilities for caching commit data.
*/

// CommitCache is a struct used for caching commit data. Stores ALL commits in
// the repository.
type CommitCache struct {
	branchHeads []*gitinfo.GitBranch
	commits     []*gitinfo.LongCommit
	repo        *gitinfo.GitInfo
	mutex       sync.RWMutex
}

// New creates and returns a new CommitCache which watches the given repo.
// The initial Update will load ALL commits from the repository, so expect
// this to be slow.
func New(repo *gitinfo.GitInfo) (*CommitCache, error) {
	// Initially load the past week's worth of commits.
	c := CommitCache{
		repo: repo,
	}
	if err := c.Update(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Update syncs the source code repository and loads any new commits.
func (c *CommitCache) Update() error {
	glog.Info("Reloading commits.")
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if err := c.repo.Update(true, true); err != nil {
		return fmt.Errorf("Failed to update the repo: %v", err)
	}
	from := time.Time{}
	if len(c.commits) > 0 {
		from = c.commits[len(c.commits)-1].Timestamp
	}
	newCommitHashes := c.repo.From(from)
	glog.Infof("Processing %d new commits.", len(newCommitHashes))
	newCommits := make([]*gitinfo.LongCommit, len(newCommitHashes))
	if len(newCommitHashes) > 0 {
		for i, h := range newCommitHashes {
			d, err := c.repo.Details(h)
			if err != nil {
				return fmt.Errorf("Failed to obtain commit details for %s: %v", h, err)
			}
			newCommits[i] = d
		}
	}
	branchHeads, err := c.repo.GetBranches()
	if err != nil {
		return fmt.Errorf("Failed to read branch information from the repo: %v", err)
	}
	// Update the cached values all at once at at the end.
	c.branchHeads = branchHeads
	c.commits = append(c.commits, newCommits...)
	return nil
}

// NumCommits returns the number of commits contained in the cache.
func (c *CommitCache) NumCommits() int {
	return len(c.commits)
}

// asJson writes the given commit range along with the branch heads in JSON
// format to the given Writer.
func (c *CommitCache) asJson(w io.Writer, startIdx, endIdx int) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	data := struct {
		Commits     []*gitinfo.LongCommit `json:"commits"`
		BranchHeads []*gitinfo.GitBranch  `json:"branch_heads"`
		StartIdx    int                   `json:"startIdx"`
		EndIdx      int                   `json:"endIdx"`
	}{
		Commits:     c.commits[startIdx:endIdx],
		BranchHeads: c.branchHeads,
		StartIdx:    startIdx,
		EndIdx:      endIdx,
	}
	return json.NewEncoder(w).Encode(&data)
}

// LastNAsJson writes the last N commits along with the branch heads in JSON
// format to the given Writer.
func (c *CommitCache) LastNAsJson(w io.Writer, n int) error {
	c.mutex.RLock()
	end := len(c.commits)
	c.mutex.RUnlock()
	start := end - n
	if start < 0 {
		start = 0
	}
	return c.asJson(w, start, end)
}

// RangeAsJson writes the given range of commits along with the branch heads
// in JSON format to the given Writer.
func (c *CommitCache) RangeAsJson(w io.Writer, startIdx, endIdx int) error {
	if startIdx < 0 || startIdx > len(c.commits) {
		return fmt.Errorf("startIdx is out of range [0, %d]: %d", len(c.commits), startIdx)
	}
	if endIdx < 0 || endIdx > len(c.commits) {
		return fmt.Errorf("endIdx is out of range [0, %d]: %d", len(c.commits), endIdx)
	}
	if endIdx < startIdx {
		return fmt.Errorf("endIdx < startIdx: %d, %d", endIdx, startIdx)
	}
	return c.asJson(w, startIdx, endIdx)
}
