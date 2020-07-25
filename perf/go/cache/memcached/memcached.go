// Package memcached implements cache.Cache via memcached.
package memcached

import (
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/cache"
)

// Cache implements the cache.Cache interface.
type Cache struct {
	client *memcache.Client
}

// New returns a new in-memory cache of the given size.
func New(server ...string) (*Cache, error) {
	c := memcache.New(server...)
	c.Timeout = time.Second * 5
	return &Cache{
		client: c,
	}, c.Ping()
}

// Add implements the cache.Cache interface.
func (c *Cache) Add(key string, value string) {
	err := c.client.Add(&memcache.Item{
		Key:   key,
		Value: []byte(value),
	})
	if err != nil {
		sklog.Errorf("Memcached failed to write: %s", err)
	}
}

// Get implements the cache.Cache interface.
func (c *Cache) Get(key string) (string, bool) {
	item, err := c.client.Get(key)
	if err != nil {
		if err != memcache.ErrCacheMiss {
			sklog.Errorf("Memcached failed to get: %s", err)
		}
		return "", false
	}
	return string(item.Value), true
}

// Exists implements the cache.Cache interface.
func (c *Cache) Exists(key string) bool {
	_, err := c.client.Get(key)
	return err == nil
}

// Confirm we implement the interface.
var _ cache.Cache = (*Cache)(nil)
