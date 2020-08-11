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

	// namespace is the string to add to each key to avoid conflicts with more
	// than one application or application instance using the same memcached
	// server.
	namespace string
}

// New returns a new in-memory cache of the given size.
//
// The namespace is the string to add to each key to avoid conflicts with more
// than one application or application instance using the same memcached server.
//
// Memcached can spread cache values across multiple servers, which is why
// 'servers' is a string slice of addresses of the form "dns_name:port".
func New(servers []string, namespace string) (*Cache, error) {
	c := memcache.New(servers...)
	c.Timeout = time.Second * 5
	return &Cache{
		client:    c,
		namespace: namespace,
	}, c.Ping()
}

// Add implements the cache.Cache interface.
func (c *Cache) Add(key string) {
	err := c.client.Set(&memcache.Item{
		Key:   key + c.namespace,
		Value: []byte{1},
	})
	if err != nil {
		sklog.Errorf("Memcached failed to write: %q %s", key, err)
	}
}

// Exists implements the cache.Cache interface.
func (c *Cache) Exists(key string) bool {
	_, err := c.client.Get(key + c.namespace)
	return err == nil
}

// Confirm we implement the interface.
var _ cache.Cache = (*Cache)(nil)
