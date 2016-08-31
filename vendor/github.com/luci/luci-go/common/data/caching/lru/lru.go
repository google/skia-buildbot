// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package lru provides least-recently-used (LRU) cache.
package lru

import (
	"container/list"
	"sync"
)

// snapshot is a snapshot of the contents of the Cache.
type snapshot map[interface{}]interface{}

type pair struct {
	k, v interface{}
}

// Heuristic is a callback function that is run after every LRU mutation to
// determine how many elements (if any) to drop. It is invoked with the
// current number of elements in the cache and the least-recently-used item.
//
// Heuristic will be called repeatedly until it returns true, indicating
// that the cache is sufficiently pruned. Every time it returns false, the
// least-recently-used element in the cache will be evicted.
//
// Heuristic is called while the cache holds its write lock, meaning that no
// locking cache methods may be called during this callback.
type Heuristic func(int, interface{}) bool

func sizeHeuristic(maxElements int) Heuristic {
	return func(curElements int, _v interface{}) bool {
		return curElements <= maxElements
	}
}

// Config is a configuration structure for the cache.
type Config struct {
	// Locker is the read/write Locker implementation to use for this cache. If
	// nil, the cache will not lock around operations and will not be
	// goroutine-safe.
	Locker Locker

	// Heuristic is the LRU heuristic to use.
	//
	// If nil, the cache will never prune elements.
	Heuristic Heuristic
}

// New instantiates a new cache from the current configuration.
func (cfg Config) New() *Cache {
	if cfg.Locker == nil {
		cfg.Locker = nopLocker{}
	}

	return newCache(&cfg)
}

// Cache is a goroutine-safe least-recently-used (LRU) cache implementation. The
// cache stores key-value mapping entries up to a size limit. If more items are
// added past that limit, the entries that have have been referenced least
// recently will be evicted.
//
// This cache uses a read-write mutex, allowing multiple simultaneous
// non-mutating readers (Peek), but only one mutating reader/writer (Get, Put,
// Mutate).
type Cache struct {
	// config is the installed configuration.
	config *Config

	cache map[interface{}]*list.Element // Map of elements.
	lru   list.List                     // List of least-recently-used elements.
}

// New creates a new goroutine-safe Cache instance retains a maximum number of
// elements.
func New(size int) *Cache {
	cfg := Config{
		Locker:    &sync.RWMutex{},
		Heuristic: sizeHeuristic(size),
	}
	return cfg.New()
}

// NewWithoutLock creates a new Cache instance retains a maximum number of
// elements. This instance does not perform any locking, and therefore is not
// goroutine-safe.
func NewWithoutLock(size int) *Cache {
	cfg := Config{
		Heuristic: sizeHeuristic(size),
	}
	return cfg.New()
}

func newCache(cfg *Config) *Cache {
	c := Cache{
		config: cfg,
		cache:  make(map[interface{}]*list.Element),
	}
	c.lru.Init()
	return &c
}

// Peek fetches the element associated with the supplied key without updating
// the element's recently-used standing.
//
// Peek uses the cache Locker's read lock.
func (c *Cache) Peek(key interface{}) interface{} {
	c.config.Locker.RLock()
	defer c.config.Locker.RUnlock()

	if e := c.cache[key]; e != nil {
		return e.Value.(*pair).v
	}
	return nil
}

// Get fetches the element associated with the supplied key, updating its
// recently-used standing.
//
// Get uses the cache Locker's read/write lock.
func (c *Cache) Get(key interface{}) interface{} {
	c.config.Locker.Lock()
	defer c.config.Locker.Unlock()

	if e := c.cache[key]; e != nil {
		c.lru.MoveToFront(e)
		return e.Value.(*pair).v
	}
	return nil
}

// Put adds a new value to the cache. The value in the cache will be replaced
// regardless of whether an item with the same key already existed.
//
// Put uses the cache Locker's read/write lock.
//
// Returns whether not a value already existed for the key.
//
// The new item will be considered most recently used.
func (c *Cache) Put(key, value interface{}) (existed bool) {
	c.Mutate(key, func(current interface{}) interface{} {
		existed = (current != nil)
		return value
	})
	return
}

// Mutate adds a value to the cache, using a generator to create the value.
//
// Mutate uses the cache Locker's read/write lock.
//
// The generator will receive the current value, or nil if there is no current
// value. It returns the new value, or nil to remove this key from the cache.
//
// The generator is called while the cache's lock is held. This means that
// the generator MUST NOT call any cache methods during its execution, as
// doing so will result in deadlock/panic.
//
// Returns the value that was put in the cache, which is the value returned
// by the generator.
//
// The key will be considered most recently used regardless of whether it was
// put.
func (c *Cache) Mutate(key interface{}, gen func(interface{}) interface{}) (value interface{}) {
	c.config.Locker.Lock()
	defer c.config.Locker.Unlock()

	e := c.cache[key]
	if e != nil {
		value = e.Value.(*pair).v
	}
	value = gen(value)
	if value == nil {
		if e != nil {
			delete(c.cache, key)
			c.lru.Remove(e)
		}
		return
	}

	if e == nil {
		// The key doesn't currently exist. Create a new one and place it at the
		// front.
		e = c.lru.PushFront(&pair{key, value})
		c.cache[key] = e
		c.pruneLocked()
	} else {
		// The element already exists. Visit it.
		c.lru.MoveToFront(e)
		e.Value.(*pair).v = value
	}
	return
}

// Remove removes an entry from the cache. If the key is present, its current
// value will be returned; otherwise, nil will be returned.
//
// Remove uses the cache Locker's read/write lock.
func (c *Cache) Remove(key interface{}) interface{} {
	c.config.Locker.Lock()
	defer c.config.Locker.Unlock()

	if e, ok := c.cache[key]; ok {
		delete(c.cache, key)
		c.lru.Remove(e)
		return e.Value.(*pair).v
	}
	return nil
}

// Purge clears the full contents of the cache.
//
// Purge uses the cache Locker's read/write lock.
func (c *Cache) Purge() {
	c.config.Locker.Lock()
	defer c.config.Locker.Unlock()

	c.cache = make(map[interface{}]*list.Element)
	c.lru.Init()
}

// Len returns the number of entries in the cache.
//
// Len uses the cache Locker's read lock.
func (c *Cache) Len() int {
	c.config.Locker.RLock()
	defer c.config.Locker.RUnlock()
	return len(c.cache)
}

// keys returns a list of keys in the cache.
func (c *Cache) keys() []interface{} {
	c.config.Locker.RLock()
	defer c.config.Locker.RUnlock()

	var keys []interface{}
	if len(c.cache) > 0 {
		keys = make([]interface{}, 0, len(c.cache))
		for k := range c.cache {
			keys = append(keys, k)
		}
	}
	return keys
}

// snapshot returns a snapshot map of the cache's entries.
func (c *Cache) snapshot() (ss snapshot) {
	c.config.Locker.RLock()
	defer c.config.Locker.RUnlock()

	if len(c.cache) > 0 {
		ss = make(snapshot)
		for k, e := range c.cache {
			ss[k] = e.Value.(*pair).v
		}
	}
	return
}

// pruneLocked prunes LRU elements until its heuristic is satisfied. Its write
// lock must be held by the caller.
func (c *Cache) pruneLocked() {
	h := c.config.Heuristic
	if h == nil {
		return
	}

	for e := c.lru.Back(); e != nil; e = c.lru.Back() {
		pair := e.Value.(*pair)
		if h(len(c.cache), pair.v) {
			break
		}

		delete(c.cache, pair.k)
		c.lru.Remove(e)
	}
}
