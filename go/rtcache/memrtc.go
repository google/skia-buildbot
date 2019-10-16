package rtcache

import (
	"context"

	lru "github.com/hashicorp/golang-lru"
	"go.skia.org/infra/go/skerr"
)

// MemReadThroughCache implements the ReadThroughCache interface.
type MemReadThroughCache struct {
	workerFn ReadThroughFunc // worker function to create the items.
	cache    *lru.Cache      // caches the items in RAM. Thread-safe.

	activeWorkerCh chan struct{} // limits the number of concurrent workerFn executions.
}

// New returns a new instance of ReadThroughCache that is stored in RAM.
// nWorkers defines the number of concurrent workers that call wokerFn when
// requested items are not in RAM.
func New(workerFn ReadThroughFunc, maxSize int, nWorkers int) (ReadThroughCache, error) {
	// if maxSize is <= 0 then we don't cache at all. But lru.Cache will not
	// limit the cache if the size is 0. So we cache 1 element.
	if maxSize <= 0 {
		maxSize = 1
	}

	lruCache, err := lru.New(maxSize)
	if err != nil {
		return nil, skerr.Wrapf(err, "making LRU with %d items", maxSize)
	}

	ret := &MemReadThroughCache{
		workerFn: workerFn,
		cache:    lruCache,

		activeWorkerCh: make(chan struct{}, nWorkers),
	}
	return ret, nil
}

// Get implements the ReadThroughCache interface.
func (m *MemReadThroughCache) Get(ctx context.Context, id string) (interface{}, error) {
	// Check the cache first
	if result, ok := m.cache.Get(id); ok {
		return result, nil
	}

	m.activeWorkerCh <- struct{}{}
	defer func() {
		<-m.activeWorkerCh
	}()
	ret, err := m.workerFn(ctx, id)
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
