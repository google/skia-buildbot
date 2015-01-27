package util

import (
	"sync"

	"github.com/golang/groupcache/lru"
)

// Generic LRUCache interface.
type LRUCache interface {
	// Add adds a key-value pair to the cache.
	Add(key, value interface{})

	// Get returns a key value for the given cache. If ok is true
	// the fetch was succesfull.
	Get(key interface{}) (value interface{}, ok bool)

	// Len returns the current size of the cache.
	Len() int

	// Remove removes a key value pair from the cache.
	Remove(key interface{})
}

// LRUCodec converts serializes/deserializes an instance of a type to/from
// byte arrays. Encode and Decode have to be the inverse of each other.
type LRUCodec interface {
	// Encode serializes the given value to a byte array (inverse of Decode).
	Encode(interface{}) ([]byte, error)

	// Decode deserializes the byte array to an instance of the type that
	// was passed to Encode in a prior call.
	Decode([]byte) (interface{}, error)
}

type MemLRUCache struct {
	cache *lru.Cache
	mutex sync.RWMutex
}

func NewMemLRUCache(maxEntries int) *MemLRUCache {
	return &MemLRUCache{
		cache: lru.New(maxEntries),
	}
}

func (m *MemLRUCache) Add(key, value interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.cache.Add(key, value)
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
}
