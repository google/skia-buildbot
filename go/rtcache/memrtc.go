package rtcache

import (
	"context"

	lru "github.com/hashicorp/golang-lru"
	"go.skia.org/infra/go/skerr"
)

// MemReadThroughCache implements the ReadThroughCache interface.
type MemReadThroughCache struct {
	rtf   ReadThroughFunc // worker function to create the items.
	cache *lru.Cache      // caches the items in RAM. Thread-safe.

	activeReadsCh chan struct{} // limits the number of concurrent ReadThroughFunc executions.
}

// New returns a new instance of ReadThroughCache that is stored in RAM.
// maxConcurrentReadThroughCalls defines the number of concurrent calls to the ReadThroughFunc when
// requested items are not cached.
func New(rtf ReadThroughFunc, maxCachedItems int, maxConcurrentReadThroughCalls int) (*MemReadThroughCache, error) {
	// if maxCachedItems is <= 0 then we don't cache at all. But lru.Cache will not
	// limit the cache if the size is 0. So we cache 1 element.
	if maxCachedItems <= 0 {
		maxCachedItems = 1
	}

	lruCache, err := lru.New(maxCachedItems)
	if err != nil {
		return nil, skerr.Wrapf(err, "making LRU with %d items", maxCachedItems)
	}

	ret := &MemReadThroughCache{
		rtf:   rtf,
		cache: lruCache,

		activeReadsCh: make(chan struct{}, maxConcurrentReadThroughCalls),
	}
	return ret, nil
}

// Get implements the ReadThroughCache interface.
func (m *MemReadThroughCache) Get(ctx context.Context, id string) (interface{}, error) {
	// Check the cache first
	if result, ok := m.cache.Get(id); ok {
		return result, nil
	}

	m.activeReadsCh <- struct{}{}
	defer func() {
		<-m.activeReadsCh
	}()
	ret, err := m.rtf(ctx, id)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	m.cache.Add(id, ret)

	return ret, nil
}

// Keys implements the ReadThroughCache interface.
func (m *MemReadThroughCache) Keys() []string {
	keys := m.cache.Keys()

	// Convert to strings.
	ret := make([]string, len(keys))
	for idx, key := range keys {
		ret[idx] = key.(string)
	}
	return ret
}

// Remove implements the ReadThroughCache interface.
func (m *MemReadThroughCache) Remove(ids []string) {
	for _, id := range ids {
		m.cache.Remove(id)
	}
}

// Contains implements the ReadThroughCache interface.
func (m *MemReadThroughCache) Contains(id string) bool {
	return m.cache.Contains(id)
}

var _ ReadThroughCache = (*MemReadThroughCache)(nil)
