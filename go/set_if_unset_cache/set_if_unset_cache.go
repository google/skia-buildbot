package set_if_unset_cache

/*
	set_if_unset_cache implements a cache which allows the caller to set a
	value for a given key only if it is unset, while locking the individual
	entry. This is convenient when obtaining the value for a key is
	expensive and should only be done if absolutely necessary.
*/

import (
	"context"
	"errors"
	"sync"
)

var (
	// ErrNoSuchEntry is returned when there is no Value for the given key.
	ErrNoSuchEntry = errors.New("No such entry.")
)

// Value represents a value stored in the cache.
type Value interface{}

// cacheEntry represents a single entry (ie. a key/value pair) in the cache.
// Access is controlled by a mutex to prevent concurrent access, so that the
// value can be set if missing.
type cacheEntry struct {
	backingCache ICache
	mtx          sync.Mutex
	key          string
	val          Value
}

// getLocked returns the stored value for this cacheEntry. If there is no value
// in the cache, the backingCache is checked, if it exists. If no value is found
// in the backingCache, ErrNoSuchEntry is returned. Assumes that the caller
// holds e.mtx.
func (e *cacheEntry) getLocked(ctx context.Context) (Value, error) {
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

// Get returns the stored value for this cacheEntry. If there is no value in the
// cache, the backingCache is checked, if it exists. If no value is found in the
// backingCache, ErrNoSuchEntry is returned.
func (e *cacheEntry) Get(ctx context.Context) (Value, error) {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	return e.getLocked(ctx)
}

// setLocked sets the value for this cacheEntry. Writes through to the
// backingCache if it exists. Assumes that the caller holds e.mtx.
func (e *cacheEntry) setLocked(ctx context.Context, value Value) error {
	if e.backingCache != nil {
		if err := e.backingCache.Set(ctx, e.key, value); err != nil {
			return err
		}
	}
	e.val = value
	return nil
}

// Set sets the value for this cacheEntry. Writes through to the backingCache
// if it exists.
func (e *cacheEntry) Set(ctx context.Context, value Value) error {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	return e.setLocked(ctx, value)
}

// SetIfUnset checks for the existence of a value for this cacheEntry, reading
// through to the backingCache if it exists and the value is not found in the
// cacheEntry. If no value is found, calls getVal to obtain a value to store. If
// getVal returns no error, the value is stored in the cacheEntry and is written
// through to the backingCache. Returns the existing or new value.
func (e *cacheEntry) SetIfUnset(ctx context.Context, getVal func(context.Context) (Value, error)) (Value, error) {
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

// ICache is an interface which defines the behaviors of a backing cache.
type ICache interface {
	// Get returns the value for the given key or ErrNoSuchEntry.
	Get(context.Context, string) (Value, error)

	// Set sets the value for the given key.
	Set(context.Context, string, Value) error

	// Delete deletes the value for the given key. This is used for cleanup
	// purposes and can be a no-op for backing caches which implement
	// persistent storage.
	Delete(context.Context, string) error
}

// SetIfUnsetCache implements a cache which allows the caller to set a value for
// a given key only if it is unset, while locking the individual entry. This is
// convenient when obtaining the value for a key is expensive and should only be
// done if absolutely necessary.
type SetIfUnsetCache struct {
	mtx          sync.Mutex
	cache        map[string]*cacheEntry
	backingCache ICache
}

// New returns a SetIfUnsetCache instance which uses the given optional
// backingCache to read and write through.
func New(backingCache ICache) *SetIfUnsetCache {
	return &SetIfUnsetCache{
		backingCache: backingCache,
		cache:        map[string]*cacheEntry{},
	}
}

// getEntry is a helper function which retrieves the given entry from the cache,
// creating an empty cacheEntry if it does not yet exist. Returns the cacheEntry
// and a bool which is true if the cacheEntry is new. If a Get or Set operation
// fails on an new cacheEntry, the caller should delete the it to keep things
// clean.
func (c *SetIfUnsetCache) getEntry(key string) (*cacheEntry, bool) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	entry, ok := c.cache[key]
	if !ok {
		entry = &cacheEntry{
			backingCache: c.backingCache,
			key:          key,
		}
		c.cache[key] = entry
	}
	return entry, !ok
}

// Get returns the stored value for the given key in the cache. If there is no
// value in the cache, the backingCache is checked, if it exists. If no value is
// found in the backingCache, ErrNoSuchEntry is returned.
func (c *SetIfUnsetCache) Get(ctx context.Context, key string) (Value, error) {
	e, isNew := c.getEntry(key)
	val, err := e.Get(ctx)
	if isNew {
		// Delete the entry we just created, for cleanliness.
		// TODO(borenet): This is racy; what if another thread inserted
		// a value for this entry after Get returned and before we lock
		// c.mtx?
		c.mtx.Lock()
		defer c.mtx.Unlock()
		delete(c.cache, key)
	}
	return val, err
}

// Set sets the value for the given key in the cache. Writes through to the
// backingCache if it exists.
func (c *SetIfUnsetCache) Set(ctx context.Context, key string, value Value) error {
	e, isNew := c.getEntry(key)
	err := e.Set(ctx, value)
	if err != nil && isNew {
		// Delete the entry we just created, for cleanliness.
		// TODO(borenet): This is racy; what if another thread inserted
		// a value for this entry after Set returned and before we lock
		// c.mtx?
		c.mtx.Lock()
		defer c.mtx.Unlock()
		delete(c.cache, key)
	}
	return err
}

// SetIfUnset checks for the existence of a value for the given key, reading
// through to the backingCache if it exists and the value is not found in the
// cache. If no value is found, calls getVal to obtain a value to store. If
// getVal returns no error, the value is stored in the cache and is written
// through to the backingCache. Returns the existing or new value.
func (c *SetIfUnsetCache) SetIfUnset(ctx context.Context, key string, getVal func(ctx context.Context) (Value, error)) (Value, error) {
	e, isNew := c.getEntry(key)
	val, err := e.SetIfUnset(ctx, getVal)
	if err != nil && isNew {
		// Delete the entry we just created, for cleanliness.
		// TODO(borenet): This is racy; what if another thread inserted
		// a value for this entry after SetIfUnset returned and before
		// we lock c.mtx?
		c.mtx.Lock()
		defer c.mtx.Unlock()
		delete(c.cache, key)
	}
	return val, err
}

// deleteLocked deletes the value for the given key in the cache. Also deletes
// the value in the backingCache, if it exists. Assumes that the caller holds
// c.mtx.
func (c *SetIfUnsetCache) deleteLocked(ctx context.Context, key string) error {
	if c.backingCache != nil {
		if err := c.backingCache.Delete(ctx, key); err != nil {
			return err
		}
	}
	delete(c.cache, key)
	return nil
}

// Delete deletes the value for the given key in the cache. Also deletes the
// value in the backingCache, if it exists.
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
