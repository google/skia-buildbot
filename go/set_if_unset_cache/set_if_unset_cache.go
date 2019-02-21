package set_if_unset_cache

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrNoSuchEntry = errors.New("No such entry.")
)

type Value interface{}

type CacheEntry struct {
	backingCache ICache
	mtx          sync.Mutex
	key          string
	val          Value
}

func (e *CacheEntry) getLocked(ctx context.Context) (Value, error) {
	if e.val == nil {
		if e.backingCache == nil {
			return nil, ErrNoSuchEntry
		}
		val, err := e.backingCache.Get(ctx, e.key)
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

func (e *CacheEntry) Get(ctx context.Context) (Value, error) {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	return e.getLocked(ctx)
}

func (e *CacheEntry) setLocked(ctx context.Context, value Value) error {
	if e.backingCache != nil {
		if err := e.backingCache.Set(ctx, e.key, value); err != nil {
			return err
		}
	}
	e.val = value
	return nil
}

func (e *CacheEntry) Set(ctx context.Context, value Value) error {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	return e.setLocked(ctx, value)
}

func (e *CacheEntry) SetIfUnset(ctx context.Context, getVal func(context.Context) (Value, error)) (Value, error) {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	value, err := e.getLocked(ctx)
	if err != nil && err != ErrNoSuchEntry {
		return nil, err
	} else if err == ErrNoSuchEntry {
		newVal, err := getVal(ctx)
		if err != nil {
			return nil, err
		}
		if err := e.setLocked(ctx, newVal); err != nil {
			return nil, err
		}
		value = newVal
	}
	return value, nil
}

type ICache interface {
	Get(context.Context, string) (Value, error)
	Set(context.Context, string, Value) error
	Delete(context.Context, string) error
}

type SetIfUnsetCache struct {
	mtx          sync.Mutex
	cache        map[string]*CacheEntry
	backingCache ICache
}

func New(backingCache ICache) *SetIfUnsetCache {
	return &SetIfUnsetCache{
		backingCache: backingCache,
		cache:        map[string]*CacheEntry{},
	}
}

func (c *SetIfUnsetCache) getEntry(key string) *CacheEntry {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	entry, ok := c.cache[key]
	if !ok {
		entry = &CacheEntry{
			backingCache: c.backingCache,
			key:          key,
		}
		c.cache[key] = entry
	}
	return entry
}

func (c *SetIfUnsetCache) Get(ctx context.Context, key string) (Value, error) {
	val, err := c.getEntry(key).Get(ctx)
	if err == ErrNoSuchEntry {
		// Delete the entry we just created, for cleanliness.
		c.mtx.Lock()
		defer c.mtx.Unlock()
		delete(c.cache, key)
	}
	return val, err
}

func (c *SetIfUnsetCache) Set(ctx context.Context, key string, value Value) error {
	return c.getEntry(key).Set(ctx, value)
}

func (c *SetIfUnsetCache) SetIfUnset(ctx context.Context, key string, getVal func(ctx context.Context) (Value, error)) (Value, error) {
	return c.getEntry(key).SetIfUnset(ctx, getVal)
}

func (c *SetIfUnsetCache) deleteLocked(ctx context.Context, key string) error {
	if c.backingCache != nil {
		if err := c.backingCache.Delete(ctx, key); err != nil {
			return err
		}
	}
	delete(c.cache, key)
	return nil
}

func (c *SetIfUnsetCache) Delete(ctx context.Context, key string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.deleteLocked(ctx, key)
}

// Cleanup runs the given function over every cache entry. For a given entry, if
// the function returns true, the entry is deleted.
func (c *SetIfUnsetCache) Cleanup(ctx context.Context, shouldDelete func(context.Context, string, Value) bool) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for key, entry := range c.cache {
		del := shouldDelete(ctx, key, entry.val)
		if del {
			if err := c.deleteLocked(ctx, key); err != nil {
				return nil
			}
		}
	}
	return nil
}

// ForEach runs the given function over every cache entry.
func (c *SetIfUnsetCache) ForEach(ctx context.Context, fn func(context.Context, string, Value)) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for key, entry := range c.cache {
		fn(ctx, key, entry.val)
	}
}

// Len returns the number of entries in the cache.
func (c *SetIfUnsetCache) Len() int {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return len(c.cache)
}
