package commit_cache

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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

var (
	// Patterns indicating which bots to skip.
	BOT_BLACKLIST = []string{
		".*-Trybot",
		".*Housekeeper.*",
	}
)

func init() {
	gob.Register([]interface{}{})
	gob.Register(map[string]interface{}{})
}

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

// skipBot determines whether the given bot should be skipped.
func skipBot(b string) bool {
	for _, p := range BOT_BLACKLIST {
		m, err := regexp.MatchString(p, b)
		if err != nil {
			glog.Fatal(err)
		}
		if m {
			return true
		}
	}
	return false
}

// CommitData is a struct which contains information about a single commit.
// Changing its structure will require completely rebuilding the cache.
type CommitData struct {
	*gitinfo.LongCommit
	Builds map[string]*buildbot.BuildSummary `json:"builds"`
}

// cacheBlock is an independently-managed slice of the commit cache. Changing
// its structure will require completely rebuilding the cache.
type cacheBlock struct {
	BlockNum     int
	CacheFile    string
	Commits      []*CommitData
	mutex        sync.RWMutex
	parent       *CommitCache
	storedBuilds map[int]bool
}

// fromFile reads the cache file and returns a commitCache object.
func fromFile(cacheFile string, parent *CommitCache, expectFull bool) (*cacheBlock, error) {
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
	// Validate the block size.
	if len(b.Commits) > BLOCK_SIZE {
		return nil, fmt.Errorf("Serialized cache block contains more than %d commits! Did the block size change?", BLOCK_SIZE)
	}
	if len(b.Commits) < BLOCK_SIZE {
		// This happens if the BLOCK_SIZE has increased OR if this is
		// the last block, which is allowed not to be full.
		if expectFull {
			return nil, fmt.Errorf("Serialized cache block contains fewer than %d commits! Did the block size change?", BLOCK_SIZE)
		}
		// Ensure that the commit slice has the required capacity.
		commits := make([]*CommitData, len(b.Commits), BLOCK_SIZE)
		n := copy(commits, b.Commits)
		if n != len(b.Commits) {
			return nil, fmt.Errorf("Failed to re-slice commit data; Copied %d of %d items.", n, len(b.Commits))
		}
		b.Commits = commits
	}
	b.parent = parent
	b.storedBuilds = map[int]bool{}
	for _, c := range b.Commits {
		for _, build := range c.Builds {
			if build.Finished {
				b.storedBuilds[build.Id] = true
			}
		}
	}
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

// UpdateBuilds reloads all build data for commits in this block.
func (b *cacheBlock) UpdateBuilds() error {
	b.mutex.RLock()
	glog.Infof("UpdateBuilds(%d)", b.BlockNum)
	hashes := make([]string, 0, len(b.Commits))
	for _, c := range b.Commits {
		hashes = append(hashes, c.Hash)
	}
	b.mutex.RUnlock()
	builds, err := buildbot.GetBuildsForCommits(hashes, b.storedBuilds)
	if err != nil {
		return err
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	for _, c := range b.Commits {
		beforeBuilds := len(c.Builds)
		for _, build := range builds[c.Hash] {
			// Store the IDs of the loaded builds *before* filtering
			// to ensure that we don't have to load them again.
			if build.IsFinished() {
				b.storedBuilds[build.Id] = true
			}
			// Filter out unwanted builders.
			if !skipBot(build.Builder) {
				if c.Builds == nil {
					c.Builds = map[string]*buildbot.BuildSummary{}
				}
				c.Builds[build.Builder] = build.GetSummary()
			}
		}
		glog.Infof("Found %d new builds (total %d) for %s", len(c.Builds)-beforeBuilds, len(c.Builds), c.Hash)
	}
	return b.toFile()
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
	blocks      []*cacheBlock
	BranchHeads []*gitinfo.GitBranch
	cacheDir    string
	mutex       sync.RWMutex
	repo        *gitinfo.GitInfo
	requestSize int
}

// New creates and returns a new CommitCache which watches the given repo.
// The initial update will load ALL commits from the repository, so expect
// this to be slow.
func New(repo *gitinfo.GitInfo, cacheDir string, requestSize int) (*CommitCache, error) {
	c := &CommitCache{
		cacheDir:    cacheDir,
		blocks:      []*cacheBlock{},
		repo:        repo,
		requestSize: requestSize,
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

	// Load the blocks from file.
	for i, f := range cacheFiles {
		b, err := fromFile(f, c, i != len(cacheFiles)-1)
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

	// Update the cache.
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
		BlockNum:     len(c.blocks),
		CacheFile:    cacheFileName(c.cacheDir, len(c.blocks)),
		Commits:      make([]*CommitData, 0, BLOCK_SIZE),
		parent:       c,
		storedBuilds: map[int]bool{},
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
			newCommits[i] = &CommitData{d, map[string]*buildbot.BuildSummary{}}
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
	// Only update the blocks needed to cover the average requestSize.
	blocksToUpdate := c.requestSize/BLOCK_SIZE + 1
	var wg sync.WaitGroup
	errs := make([]error, blocksToUpdate)
	for i := 0; i < blocksToUpdate; i++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			blockIdx := len(c.blocks) - blocksToUpdate + j
			errs[j] = c.blocks[blockIdx].UpdateBuilds()
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
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
