package atomic_miss_cache

/*
	atomic_miss_cache implements a cache which allows the caller to set a
	value for a given key only if it is unset, while locking the individual
	entry. This is convenient when obtaining the value for a key on cache
	miss is expensive and should only be done if absolutely necessary.
*/

import (
	"context"
	"errors"
	"sync"
)

var (
	// ErrNoSuchEntry is returned when there is no Value for the given key.
	ErrNoSuchEntry = errors.New("No such entry.")

	ErrNoNilValue = errors.New("Illegal value; Value cannot be nil.")
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
	if value == nil {
		return ErrNoNilValue
	}
	if e.backingCache != nil {
		if err := e.backingCache.Set(ctx, e.key, value); err != nil {
			return err
		}
	}
	e.val = value
	return nil
}

// Set sets the value for this cacheEntry. Writes through to the backingCache
// if it exists. The given Value should not be nil.
func (e *cacheEntry) Set(ctx context.Context, value Value) error {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	return e.setLocked(ctx, value)
}

// SetIfUnset checks for the existence of a value for this cacheEntry, reading
// through to the backingCache if it exists and the value is not found in the
// cacheEntry. If no value is found, calls getVal to obtain a value to store. If
// getVal returns no error, the value is stored in the cacheEntry and is written
// through to the backingCache. Returns the existing or new value. getVal should
// not return nil with no error. getVal is run with the cacheEntry locked, so it
// should not call any methods on cacheEntry, or a deadlock will occur.
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
	// Get returns the value for the given key or ErrNoSuchEntry. Get should
	// not return nil without an error.
	Get(context.Context, string) (Value, error)

	// Set sets the value for the given key.
	Set(context.Context, string, Value) error

	// Delete deletes the value for the given key. This is used for cleanup
	// purposes and can be a no-op for backing caches which implement
	// persistent storage.
	Delete(context.Context, string) error
}

// AtomicMissCache implements a cache which allows the caller to set a value for
// a given key only if it is unset, while locking the individual entry. This is
// convenient when obtaining the value for a key on cache miss is expensive and
// should only be done if absolutely necessary.
type AtomicMissCache struct {
	mtx          sync.Mutex
	cache        map[string]*cacheEntry
	backingCache ICache
}

// New returns a AtomicMissCache instance which uses the given optional
// backingCache to read and write through. The returned AtomicMissCache is
// goroutine-safe if the given backingCache is goroutine-safe.
func New(backingCache ICache) *AtomicMissCache {
	return &AtomicMissCache{
		backingCache: backingCache,
		cache:        map[string]*cacheEntry{},
	}
}

// getEntry is a helper function which retrieves the given entry from the cache,
// creating an empty cacheEntry if it does not yet exist. If a new entry is
// created but no value is set, it will be removed on the next call to Cleanup.
func (c *AtomicMissCache) getEntry(key string) *cacheEntry {
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
	return entry
}

// Get returns the stored value for the given key in the cache. If there is no
// value in the cache, the backingCache is checked, if it exists. If no value is
// found in the backingCache, ErrNoSuchEntry is returned.
func (c *AtomicMissCache) Get(ctx context.Context, key string) (Value, error) {
	return c.getEntry(key).Get(ctx)
}

// Set sets the value for the given key in the cache. Writes through to the
// backingCache if it exists. The given Value should not be nil.
func (c *AtomicMissCache) Set(ctx context.Context, key string, value Value) error {
	return c.getEntry(key).Set(ctx, value)
}

// SetIfUnset checks for the existence of a value for the given key, reading
// through to the backingCache if it exists and the value is not found in the
// cache. If no value is found, calls getVal to obtain a value to store. If
// getVal returns no error, the value is stored in the cache and is written
// through to the backingCache. Returns the existing or new value. getVal should
// not return nil with no error. getVal is run with the cache entry locked, so
// it should not attempt to retrieve the same entry, or a deadlock will occur.
func (c *AtomicMissCache) SetIfUnset(ctx context.Context, key string, getVal func(ctx context.Context) (Value, error)) (Value, error) {
	return c.getEntry(key).SetIfUnset(ctx, getVal)
}

// deleteLocked deletes the value for the given key in the cache. Also deletes
// the value in the backingCache, if it exists. Assumes that the caller holds
// c.mtx AND the mutex of the cache entry in question, if it exists.
func (c *AtomicMissCache) deleteLocked(ctx context.Context, key string) error {
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
func (c *AtomicMissCache) Delete(ctx context.Context, key string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	e, ok := c.cache[key]
	if ok {
		e.mtx.Lock()
		defer e.mtx.Unlock()
	}
	return c.deleteLocked(ctx, key)
}

// Cleanup runs the given function over every cache entry. For a given entry, if
// the function returns true, the entry is deleted. Also deletes the entry in
// the backingCache, if it exists. shouldDelete should not call any methods on
// the AtomicMissCache.
func (c *AtomicMissCache) Cleanup(ctx context.Context, shouldDelete func(context.Context, string, Value) bool) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for key, entry := range c.cache {
		entry.mtx.Lock()
		if entry.val == nil || shouldDelete(ctx, key, entry.val) {
			// If there is no Value for this entry, then it's left
			// over from a previous incomplete Get or Set.
			if err := c.deleteLocked(ctx, key); err != nil {
				return err
			}
		}
		entry.mtx.Unlock()
	}
	return nil
}

// ForEach runs the given function over every cache entry. The function should
// not call any methods on the AtomicMissCache. Only iterates entries in the
// local cache; does not load cached entries from the backingCache.
func (c *AtomicMissCache) ForEach(ctx context.Context, fn func(context.Context, string, Value)) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for key, entry := range c.cache {
		entry.mtx.Lock()
		// We may have entries with no value because we create missing
		// entries in calls to Get (or calls to Set which do not finish
		// successfully). The caller is not interested in these phantom
		// entries.
		if entry.val != nil {
			fn(ctx, key, entry.val)
		}
		entry.mtx.Unlock()
	}
}
