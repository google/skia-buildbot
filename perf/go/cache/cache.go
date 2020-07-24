// Package cache defines an interface for an LRU cache.
package cache

// Cache in an interface for an LRU cache.
type Cache interface {
	// Add adds a value to the cache.
	Add(key string, value interface{})

	// Get looks up a key's value from the cache, returning the value and true
	// if found, otherwise the returned bool is false.
	Get(key string) (interface{}, bool)

	// Exists returns true  if the key is found in the cache.
	Exists(key string) bool
}
