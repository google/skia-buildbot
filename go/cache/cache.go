// Package cache defines an interface for an LRU cache.
package cache

import "context"

// Cache in an interface for an LRU cache.
type Cache interface {
	// Add adds a value to the cache.
	Add(key string)

	// Exists returns true  if the key is found in the cache.
	Exists(key string) bool

	// SetValue sets the value for the given key in the cache.
	SetValue(ctx context.Context, key string, value string) error

	// GetValue returns the value for the corresponding key from the cache.
	GetValue(ctx context.Context, key string) (string, error)
}
