// Package cache defines an interface for an LRU cache.
package cache

// Cache in an interface for an LRU cache.
type Cache interface {
	// Add adds a value to the cache.
	Add(key string)

	// Exists returns true  if the key is found in the cache.
	Exists(key string) bool
}
