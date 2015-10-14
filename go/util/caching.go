package util

import (
	"encoding/json"
	"reflect"
	"sync"

	"github.com/golang/groupcache/lru"
)

// Generic LRUCache interface.
type LRUCache interface {
	// Add adds a key-value pair to the cache.
	Add(key, value interface{}) bool

	// Get returns a key value for the given cache. If ok is true
	// the fetch was succesfull.
	Get(key interface{}) (value interface{}, ok bool)

	// Len returns the current size of the cache.
	Len() int

	// Remove removes a key value pair from the cache.
	Remove(key interface{})

	// Keys returns all the keys in the LRUCache.
	Keys() []interface{}
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

type jsonCodec struct {
	targetType reflect.Type
	isSlice    bool
}

// JSONCodec implements the LRUCodec interface by serializing and
// deserializing instances of the underlying type of 'instance'.
// Generally it's assumed that 'instance' is a struct, a pointer to
// a struct or a slice.
func JSONCodec(instance interface{}) LRUCodec {
	targetType := reflect.TypeOf(instance)
	if targetType.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
	}
	isSlice := targetType.Kind() == reflect.Slice
	return &jsonCodec{
		targetType: targetType,
		isSlice:    isSlice,
	}
}

// See LRUCodec interface.
func (j *jsonCodec) Encode(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

// See LRUCodec interface.
func (j *jsonCodec) Decode(byteData []byte) (interface{}, error) {
	ret := reflect.New(j.targetType).Interface()
	err := json.Unmarshal(byteData, ret)
	if err != nil {
		return nil, err
	} else if j.isSlice {
		return reflect.ValueOf(ret).Elem().Interface(), nil
	}

	return ret, nil
}
