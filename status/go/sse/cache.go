package sse

import (
	"regexp"
	"strings"
	"sync"

	"go.skia.org/infra/status/go/rpc"
)

// requestCache stores sets of inputs and outputs for task query and filtering
// requests. It is intended to be shared between multiple clients for a single
// update. It uses a two-level locking system to solve the race condition
// between multiple concurrent clients which would ordinarily call Get(), find
// no entry, perform expensive computation, and then call Put().
type requestCache struct {
	m   map[requestCacheKey]*requestCacheEntry
	mtx sync.Mutex
}

func newRequestCache() *requestCache {
	return &requestCache{
		m: map[requestCacheKey]*requestCacheEntry{},
	}
}

type requestCacheKey struct {
	repoURL          string
	displayedCommits string
	taskFilter       taskFilter
	taskSearch       string
}

type requestCacheEntry struct {
	tasks         []*rpc.Task
	hideTaskSpecs []string
	ready         bool
	mtx           sync.Mutex
}

func (c *requestCache) key(repoURL string, displayedCommits []*rpc.LongCommit, taskFilter taskFilter, taskSearch *regexp.Regexp) requestCacheKey {
	taskSearchStr := ""
	if taskSearch != nil {
		taskSearchStr = taskSearch.String()
	}
	hashes := make([]string, 0, len(displayedCommits))
	for _, commit := range displayedCommits {
		hashes = append(hashes, commit.Hash)
	}
	return requestCacheKey{
		repoURL:          repoURL,
		displayedCommits: strings.Join(hashes, ","),
		taskFilter:       taskFilter,
		taskSearch:       taskSearchStr,
	}
}

// GetOrCache retrieves the cache entry associated with the given inputs, if it
// exists. Otherwise, it calls the given function to produce the outputs, stores
// them as a new entry in the cache, and returns the outputs. The function call
// is mutexed to ensure that it is only called once, even if multiple clients
// call GetOrCache concurrently with the same inputs.
func (c *requestCache) GetOrCache(repoURL string, displayedCommits []*rpc.LongCommit, taskFilter taskFilter, taskSearch *regexp.Regexp, fn func() ([]*rpc.Task, []string, error)) ([]*rpc.Task, []string, error) {
	// Retrieve the cache entry, creating it if it does not exist.
	key := c.key(repoURL, displayedCommits, taskFilter, taskSearch)
	c.mtx.Lock()
	entry, ok := c.m[key]
	if !ok {
		entry = &requestCacheEntry{}
		c.m[key] = entry
	}
	c.mtx.Unlock()

	entry.mtx.Lock()
	defer entry.mtx.Unlock()
	if !entry.ready {
		tasks, hideTaskSpecs, err := fn()
		if err != nil {
			return nil, nil, err
		}
		entry.tasks = tasks
		entry.hideTaskSpecs = hideTaskSpecs
		entry.ready = true
	}
	return entry.tasks, entry.hideTaskSpecs, nil
}
