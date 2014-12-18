package commit_cache

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/golang/glog"

	"skia.googlesource.com/buildbot.git/go/buildbot"
	"skia.googlesource.com/buildbot.git/go/gitinfo"
)

/*
	Utilities for caching commit data.
*/

// CommitData is a struct which contains information about a single commit.
type CommitData struct {
	*gitinfo.LongCommit
	Builds map[string]*buildbot.Build `json:"builds"`
}

// CommitCache is a struct used for caching commit data. Stores ALL commits in
// the repository.
type CommitCache struct {
	BranchHeads []*gitinfo.GitBranch
	Commits     []*CommitData
	repo        *gitinfo.GitInfo
	mutex       sync.RWMutex
	cacheFile   string
}

// New creates and returns a new CommitCache which watches the given repo.
// The initial Update will load ALL commits from the repository, so expect
// this to be slow.
func New(repo *gitinfo.GitInfo, cacheFile string) (*CommitCache, error) {
	c, err := fromFile(cacheFile)
	if err != nil {
		glog.Errorf("Could not deserialize cache from file, reloading commit data from scratch. Error: %v", err)
	}
	c.repo = repo
	c.cacheFile = cacheFile
	if err := c.Update(); err != nil {
		return nil, err
	}

	// Update in a loop.
	go func() {
		for _ = range time.Tick(time.Minute) {
			if err := c.Update(); err != nil {
				glog.Errorf("Failed to update commit cache: %v", err)
			}
		}
	}()
	return c, nil
}

// fromFile reads the cache file and returns a commitCache object.
func fromFile(cacheFile string) (*CommitCache, error) {
	glog.Infof("Reading commit cache from file %s", cacheFile)
	c := CommitCache{}
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		glog.Infof("Commit cache file %s does not exist. Loading commits from scratch.", cacheFile)
		return &c, nil
	}
	f, err := os.Open(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to open cache file %s: %v", cacheFile, err)
	}
	defer f.Close()
	if err := gob.NewDecoder(f).Decode(&c); err != nil {
		return nil, fmt.Errorf("Failed to read cache file %s: %v", cacheFile, err)
	}
	glog.Infof("Done reading commit cache from %s", cacheFile)
	return &c, nil
}

// toFile serializes the cache to a file. Assumes the caller holds a lock.
func (c *CommitCache) toFile() error {
	glog.Infof("Writing commit cache to file %s", c.cacheFile)
	f, err := os.Create(c.cacheFile)
	if err != nil {
		return fmt.Errorf("Failed to open/create cache file %s: %v", c.cacheFile, err)
	}
	defer f.Close()
	if err := gob.NewEncoder(f).Encode(c); err != nil {
		return fmt.Errorf("Failed to write cache file %s: %v", c.cacheFile, err)
	}
	glog.Infof("Done writing commit cache to %s", c.cacheFile)
	return nil
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
	if len(c.Commits) > 0 {
		from = c.Commits[len(c.Commits)-1].Timestamp
	}
	newCommitHashes := c.repo.From(from)
	glog.Infof("Processing %d new commits.", len(newCommitHashes))
	newCommits := make([]*CommitData, len(newCommitHashes))
	if len(newCommitHashes) > 0 {
		for i, h := range newCommitHashes {
			d, err := c.repo.Details(h)
			if err != nil {
				return fmt.Errorf("Failed to obtain commit details for %s: %v", h, err)
			}
			newCommits[i] = &CommitData{d, nil}
		}
	}
	branchHeads, err := c.repo.GetBranches()
	if err != nil {
		return fmt.Errorf("Failed to read branch information from the repo: %v", err)
	}
	// Update the cached values all at once at at the end.
	c.BranchHeads = branchHeads
	c.Commits = append(c.Commits, newCommits...)
	// Update the cache file.
	if err := c.toFile(); err != nil {
		return err
	}
	return nil
}

// NumCommits returns the number of commits contained in the cache.
func (c *CommitCache) NumCommits() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.Commits)
}

// asJson writes the given commit range along with the branch heads in JSON
// format to the given Writer. Assumes that the caller holds a read lock.
func (c *CommitCache) asJson(w io.Writer, startIdx, endIdx int) error {
	data := struct {
		Commits     []*CommitData        `json:"commits"`
		BranchHeads []*gitinfo.GitBranch `json:"branch_heads"`
		StartIdx    int                  `json:"startIdx"`
		EndIdx      int                  `json:"endIdx"`
	}{
		Commits:     c.Commits[startIdx:endIdx],
		BranchHeads: c.BranchHeads,
		StartIdx:    startIdx,
		EndIdx:      endIdx,
	}
	return json.NewEncoder(w).Encode(&data)
}

// LastNAsJson writes the last N commits along with the branch heads in JSON
// format to the given Writer.
func (c *CommitCache) LastNAsJson(w io.Writer, n int) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	end := len(c.Commits)
	start := end - n
	if start < 0 {
		start = 0
	}
	return c.asJson(w, start, end)
}

// RangeAsJson writes the given range of commits along with the branch heads
// in JSON format to the given Writer.
func (c *CommitCache) RangeAsJson(w io.Writer, startIdx, endIdx int) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if startIdx < 0 || startIdx > len(c.Commits) {
		return fmt.Errorf("startIdx is out of range [0, %d]: %d", len(c.Commits), startIdx)
	}
	if endIdx < 0 || endIdx > len(c.Commits) {
		return fmt.Errorf("endIdx is out of range [0, %d]: %d", len(c.Commits), endIdx)
	}
	if endIdx < startIdx {
		return fmt.Errorf("endIdx < startIdx: %d, %d", endIdx, startIdx)
	}
	return c.asJson(w, startIdx, endIdx)
}
