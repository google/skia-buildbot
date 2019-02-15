package lock_cache

import (
	"errors"
	"sync"
)

var (
	ErrNoSuchEntry = errors.New("No such entry.")
)

type Key interface{}

type Value interface{}

type CacheEntry struct {
	c   *LockCache
	mtx sync.Mutex
	key Key
	val Value
}

func (e *CacheEntry) getLocked() (Value, error) {
	if e.val == nil {
		val, err := e.c.backingCache.Get(e.key)
		if err != nil {
			return nil, err
		}
		if val == nil {
			return nil, ErrNoSuchEntry
		}
		e.val = val
	}
	return e.val, nil
}

func (e *CacheEntry) Get() (Value, error) {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	return e.getLocked()
}

func (e *CacheEntry) setLocked(value Value) error {
	if err := e.c.backingCache.Set(e.key, value); err != nil {
		return err
	}
	e.val = value
	return nil
}

func (e *CacheEntry) Set(value Value) error {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	return e.setLocked(value)
}

func (e *CacheEntry) SetIfUnset(getVal func() Value) (Value, error) {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	value, err := e.getLocked()
	if err != nil && err != ErrNoSuchEntry {
		return nil, err
	} else if err == ErrNoSuchEntry {
		value = getVal()
		if err := e.setLocked(value); err != nil {
			return nil, err
		}
	}
	return value
}

type Cache interface {
	Get(Key) (Value, error)
	Set(Key, Value) error
}

type LockCache struct {
	mtx          sync.Mutex
	cache        map[Key]*CacheEntry
	backingCache Cache
}

func New(backingCache Cache) *LockCache {
	return &LockCache{
		backingCache: backingCache,
		cache:        map[Key]*CacheEntry{},
	}
}

func (c *LockCache) getEntry(key Key) *CacheEntry {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	entry, ok := c.cache[key]
	if !ok {
		entry = &CacheEntry{
			c:   c,
			key: key,
		}
		c.cache[key] = entry
	}
	return entry
}

func (c *LockCache) Get(key Key) Value {
	return c.getEntry(key).Get()
}

func (c *LockCache) Set(key Key, value Value) {
	c.getEntry(key).Set(value)
}

func (c *LockCache) SetIfUnset(key Key, getVal func() Value) {
	c.getEntry(key).SetIfUnset(getVal)
}
