package window

import (
	"sync"
	"time"
)

// Cache entries should implement Entry.
type Entry interface {
	Repository() (string, error)
	Timestamp() (time.Time, error)
}

// Implement Cache to build a cache which uses a Window.
type Cache interface {
	Insert(Entry) error
	Evict(Entry) error
	AllEntries() []Entry
}

// Implementation of a cache which uses a Window to determine which entries to
// keep and which to evict.
type WindowCache struct {
	caches []Cache
	mtx    sync.RWMutex
	w      *Window
}

func NewWindowCache(w *Window, caches ...Cache) *WindowCache {
	return &WindowCache{
		caches: caches,
		mtx:    sync.RWMutex{},
		w:      w,
	}
}

func (c *WindowCache) Insert(entries []Entry) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for _, e := range entries {
		for _, cache := range c.caches {
			if err := cache.Insert(e); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *WindowCache) Expire() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	entries := c.caches[0].AllEntries()
	for _, e := range entries {
		ts, err := e.Timestamp()
		if err != nil {
			return err
		}
		repo, err := e.Repository()
		if err != nil {
			return nil
		}
		if !c.w.TestTime(repo, ts) {
			for _, cache := range c.caches {
				if err := cache.Evict(e); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *WindowCache) RLock() {
	c.mtx.RLock()
}

func (c *WindowCache) RUnlock() {
	c.mtx.RUnlock()
}
