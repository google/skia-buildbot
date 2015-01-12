package commit_cache

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/skia-dev/glog"

	"skia.googlesource.com/buildbot.git/go/buildbot"
	"skia.googlesource.com/buildbot.git/go/gitinfo"
)

/*
	Utilities for caching commit data.
*/

const (
	// Number of commits to store in a given cache block. Changing this
	// will require completely rebuilding the cache.
	BLOCK_SIZE = 25

	// Pattern for cache file names.
	CACHE_FILE_PATTERN = "commit_cache.%d.gob"
)

// cacheFileName returns the name of a cache file, based on the cache directory
// and the block number.
func cacheFileName(cacheDir string, blockNum int) string {
	return filepath.Join(cacheDir, fmt.Sprintf(CACHE_FILE_PATTERN, blockNum))
}

// blockIdx returns the index of the block and the index of the commit
// within that block for the given commit index.
func blockIdx(commitIdx int) (int, int) {
	return commitIdx / BLOCK_SIZE, commitIdx % BLOCK_SIZE
}

// CommitData is a struct which contains information about a single commit.
// Changing its structure will require completely rebuilding the cache.
type CommitData struct {
	*gitinfo.LongCommit
	Builds map[string]*buildbot.Build `json:"builds"`
}

// cacheBlock is an independently-managed slice of the commit cache. Changing
// its structure will require completely rebuilding the cache.
type cacheBlock struct {
	BlockNum  int
	mutex     sync.RWMutex
	Commits   []*CommitData
	CacheFile string
}

// fromFile reads the cache file and returns a commitCache object.
func fromFile(cacheFile string) (*cacheBlock, error) {
	glog.Infof("Reading commit cache from file %s", cacheFile)
	b := cacheBlock{}
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("Commit cache file %s does not exist.", cacheFile)
	}
	f, err := os.Open(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to open cache file %s: %v", cacheFile, err)
	}
	defer f.Close()
	if err := gob.NewDecoder(f).Decode(&b); err != nil {
		return nil, fmt.Errorf("Failed to read cache file %s: %v", cacheFile, err)
	}
	b.CacheFile = cacheFile
	glog.Infof("Done reading cache block %d (%d commits) from %s", b.BlockNum, len(b.Commits), cacheFile)
	return &b, nil
}

// toFile serializes the cache block to a file.
func (b *cacheBlock) toFile() error {
	glog.Infof("Writing commit cache block %d (%d commits) to file %s", b.BlockNum, len(b.Commits), b.CacheFile)
	f, err := os.Create(b.CacheFile)
	if err != nil {
		return fmt.Errorf("Failed to open/create cache file %s: %v", b.CacheFile, err)
	}
	defer f.Close()
	if err := gob.NewEncoder(f).Encode(b); err != nil {
		return fmt.Errorf("Failed to write cache file %s: %v", b.CacheFile, err)
	}
	glog.Infof("Done writing commit cache to %s", b.CacheFile)
	return nil
}

// NumCommits gives the number of commits in this block.
func (b *cacheBlock) NumCommits() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return len(b.Commits)
}

// Get returns the CommitData at the given index within this block.
func (b *cacheBlock) Get(idx int) (*CommitData, error) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	if idx < 0 || idx >= len(b.Commits) {
		return nil, fmt.Errorf("Index out of range: %d not in [%d, %d)", idx, 0, len(b.Commits))
	}
	return b.Commits[idx], nil
}

// Slice returns a slice of CommitDatas from this block.
func (b *cacheBlock) Slice(startIdx, endIdx int) ([]*CommitData, error) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	if startIdx < 0 || startIdx > endIdx || endIdx > len(b.Commits) {
		return nil, fmt.Errorf("Index out of range: (%d < 0 || %d > %d || %d > %d)", startIdx, startIdx, endIdx, endIdx, len(b.Commits))
	}
	return b.Commits[startIdx:endIdx], nil
}

// NewCommits copies a portion of the new commits into this block and returns
// the number of commits which were copied.
func (b *cacheBlock) NewCommits(newCommits []*CommitData) (int, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	// Figure out how many commits we can copy.
	oldLen := len(b.Commits)
	n := BLOCK_SIZE - oldLen
	if n > len(newCommits) {
		n = len(newCommits)
	}
	// Extend the slice to contain the new commits.
	b.Commits = b.Commits[:oldLen+n]
	// Copy over the new commits.
	actuallyCopied := copy(b.Commits[oldLen:], newCommits[:n])
	if n != actuallyCopied {
		return actuallyCopied, fmt.Errorf("Wanted to copy %d but copied %d", n, actuallyCopied)
	}
	return n, b.toFile()
}

// CommitCache is a struct used for caching commit data. Stores ALL commits in
// the repository.
type CommitCache struct {
	BranchHeads []*gitinfo.GitBranch
	blocks      []*cacheBlock
	repo        *gitinfo.GitInfo
	mutex       sync.RWMutex
	cacheDir    string
}

// New creates and returns a new CommitCache which watches the given repo.
// The initial update will load ALL commits from the repository, so expect
// this to be slow.
func New(repo *gitinfo.GitInfo, cacheDir string) (*CommitCache, error) {
	c := &CommitCache{
		repo:     repo,
		cacheDir: cacheDir,
		blocks:   []*cacheBlock{},
	}
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		if err = os.MkdirAll(cacheDir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("Failed to create cache dir: %v", err)
		}
	}
	cacheFiles := []string{}
	for {
		fileName := cacheFileName(cacheDir, len(cacheFiles))
		if _, err := os.Stat(fileName); os.IsNotExist(err) {
			break
		}
		cacheFiles = append(cacheFiles, fileName)
	}

	for _, f := range cacheFiles {
		b, err := fromFile(f)
		if err != nil {
			return nil, fmt.Errorf("Could not load cache block: %v", err)
		}
		c.blocks = append(c.blocks, b)
	}

	if len(cacheFiles) == 0 {
		if err := c.appendBlock(); err != nil {
			return nil, err
		}
	}

	if err := c.update(); err != nil {
		return nil, err
	}

	// Update in a loop.
	go func() {
		for _ = range time.Tick(time.Minute) {
			if err := c.update(); err != nil {
				glog.Errorf("Failed to update commit cache: %v", err)
			}
		}
	}()
	return c, nil
}

// numCommits returns the number of commits in the cache. Assumes the caller
// holds a lock.
func (c *CommitCache) numCommits() int {
	if c.blocks == nil {
		return 0
	}
	b := len(c.blocks)
	if b == 0 {
		return 0
	}
	// Assume all blocks are full, except the last.
	return (b-1)*BLOCK_SIZE + c.blocks[b-1].NumCommits()
}

// NumCommits returns the number of commits in the cache.
func (c *CommitCache) NumCommits() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.numCommits()
}

// Get returns the CommitData at the given index.
func (c *CommitCache) Get(idx int) (*CommitData, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	b, i := blockIdx(idx)
	if b >= len(c.blocks) {
		return nil, fmt.Errorf("Index out of range: %d not in [%d, %d)", b, 0, len(c.blocks))
	}
	return c.blocks[b].Get(i)
}

// Slice returns a slice of CommitDatas from the cache.
func (c *CommitCache) Slice(startIdx, endIdx int) ([]*CommitData, error) {
	c.mutex.RLock()
	c.mutex.RUnlock()
	n := c.numCommits()
	if startIdx < 0 || startIdx > endIdx || endIdx > n {
		return nil, fmt.Errorf("Index out of range: (%d < 0 || %d > %d || %d > %d)", startIdx, startIdx, endIdx, endIdx, n)
	}
	startBlock, startSubIdx := blockIdx(startIdx)
	endBlock, endSubIdx := blockIdx(endIdx)
	rv := make([]*CommitData, 0, endIdx-startIdx+1)
	for b := startBlock; b <= endBlock; b++ {
		var i, j int
		if b == startBlock {
			i = startSubIdx
		} else {
			i = 0
		}
		if b == endBlock {
			j = endSubIdx
		} else {
			j = c.blocks[b].NumCommits()
		}
		s, err := c.blocks[b].Slice(i, j)
		if err != nil {
			return nil, fmt.Errorf("Failed to slice block: %v", err)
		}
		rv = append(rv, s...)
	}
	return rv, nil
}

// appendBlock adds a new block to the cache. Assumes the caller holds a lock.
func (c *CommitCache) appendBlock() error {
	if c.blocks == nil {
		c.blocks = []*cacheBlock{}
	}
	if len(c.blocks) > 0 && c.blocks[len(c.blocks)-1].NumCommits() != BLOCK_SIZE {
		return fmt.Errorf("Failed to add a new cache block; current last block is not full.")
	}
	c.blocks = append(c.blocks, &cacheBlock{
		BlockNum:  len(c.blocks),
		CacheFile: cacheFileName(c.cacheDir, len(c.blocks)),
		Commits:   make([]*CommitData, 0, BLOCK_SIZE),
	})
	return nil
}

// update syncs the source code repository and loads any new commits.
func (c *CommitCache) update() error {
	glog.Info("Reloading commits.")
	if err := c.repo.Update(true, true); err != nil {
		return fmt.Errorf("Failed to update the repo: %v", err)
	}
	from := time.Time{}
	n := c.NumCommits()
	if n > 0 {
		last, err := c.Get(n - 1)
		if err != nil {
			return fmt.Errorf("Failed to get last commit: %v", err)
		}
		from = last.Timestamp
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
	glog.Infof("Updating the cache.")
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.BranchHeads = branchHeads
	for i := 0; i < len(newCommits); {
		n, err := c.blocks[len(c.blocks)-1].NewCommits(newCommits[i:])
		if err != nil {
			return fmt.Errorf("Failed to insert new commits into block: %v", err)
		}
		i += n
		if i == len(newCommits) {
			break
		}
		if err := c.appendBlock(); err != nil {
			return fmt.Errorf("Failed to update the cache: %v", err)
		}
	}
	glog.Infof("Finished updating the cache.")
	return nil
}

// RangeAsJson writes the given commit range along with the branch heads in JSON
// format to the given Writer. Assumes that the caller holds a read lock.
func (c *CommitCache) RangeAsJson(w io.Writer, startIdx, endIdx int) error {
	commits, err := c.Slice(startIdx, endIdx)
	if err != nil {
		return err
	}
	data := struct {
		Commits     []*CommitData        `json:"commits"`
		BranchHeads []*gitinfo.GitBranch `json:"branch_heads"`
		StartIdx    int                  `json:"startIdx"`
		EndIdx      int                  `json:"endIdx"`
	}{
		Commits:     commits,
		BranchHeads: c.BranchHeads,
		StartIdx:    startIdx,
		EndIdx:      endIdx,
	}
	return json.NewEncoder(w).Encode(&data)
}

// LastNAsJson writes the last N commits along with the branch heads in JSON
// format to the given Writer.
func (c *CommitCache) LastNAsJson(w io.Writer, n int) error {
	end := c.numCommits()
	start := end - n
	if start < 0 {
		start = 0
	}
	return c.RangeAsJson(w, start, end)
}
