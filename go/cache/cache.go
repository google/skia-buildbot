package cache

import (
	"sync"

	"github.com/golang/groupcache/lru"
)

// Generic LRU cache interface.
type LRU interface {
	// Add adds a key-value pair to the cache.
	Add(key, value interface{}) bool

	// Get returns a key value for the given cache. If ok is true
	// the fetch was successful.
	Get(key interface{}) (value interface{}, ok bool)

	// Len returns the current size of the cache.
	Len() int

	// Remove removes a key value pair from the cache.
	Remove(key interface{})

	// Keys returns all the keys in the LRU.
	Keys() []interface{}
}

type MemLRUCache struct {
	cache *lru.Cache
	keys  map[string]bool
	mutex sync.RWMutex
}

func NewMemLRUCache(maxEntries int) *MemLRUCache {
	ret := &MemLRUCache{
		cache: lru.New(maxEntries),
		keys:  map[string]bool{},
	}

	ret.cache.OnEvicted = func(key lru.Key, value interface{}) {
		delete(ret.keys, getStringKey(key))
	}

	return ret
}

func getStringKey(key interface{}) string {
	k := ""
	switch key := key.(type) {
	case string:
		k = key
	case []byte:
		if key != nil {
			k = string(key)
		}
	}
	return k
}

func (m *MemLRUCache) Add(key, value interface{}) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.cache.Add(key, value)
	m.keys[getStringKey(key)] = true
	return true
}

func (m *MemLRUCache) Get(key interface{}) (value interface{}, ok bool) {
	m.mutex.RLock()
	m.mutex.RUnlock()
	return m.cache.Get(key)
}

func (m *MemLRUCache) Len() int {
	m.mutex.RLock()
	m.mutex.RUnlock()
	return m.cache.Len()
}

func (m *MemLRUCache) Remove(key interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.cache.Remove(key)
	delete(m.keys, getStringKey(key))
}

func (m *MemLRUCache) Keys() []interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	ret := make([]interface{}, 0, len(m.keys))
	for k := range m.keys {
		ret = append(ret, k)
	}
	return ret
}
