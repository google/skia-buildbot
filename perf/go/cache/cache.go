package cache

// Cache in an interface for an LRU cache.
type Cache interface {
	Add(key string, value interface{})
	Get(key string) (interface{}, bool)
	Exists(key string) bool
}
