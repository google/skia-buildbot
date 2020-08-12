// Package memcached implements cache.Cache via memcached.
package memcached

import (
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/cache"
	"go.skia.org/infra/perf/go/cache/local"
)

// The largest Perf instance right now has roughly 5 million traces, and we need
// a cache entry for each trace id. There are other cache entries for the
// ParamSets, but they are insignificant in number. Since we can be writing to
// two tiles at the same time when commits cross a tile boundary we multiply by
// two, and then double the value again for future growth.
//
// The use of the cache is dominated by marking if a tracename has already been
// written to the Postings table. The keys for those cache entries are mostly
// hex encoded md5 hashes, which are 32 bytes long, so this will use roughly
// 640MB when full.
const localCacheSize = 20 * 1000 * 1000

// Cache implements the cache.Cache interface.
type Cache struct {
	client *memcache.Client

	// We have an in memory cache because that will save round-trips to
	// memcached.
	localCache cache.Cache

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
	localCache, err := local.New(localCacheSize)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to build local cache in memcached.")
	}
	return &Cache{
		client:     c,
		localCache: localCache,
		namespace:  namespace,
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
	// Always store it locally regardless of memcached error, failure to do that
	// would mean that if memcached went down then we'd have no caching at all.
	c.localCache.Add(key)
}

// Exists implements the cache.Cache interface.
func (c *Cache) Exists(key string) bool {
	if c.localCache.Exists(key) {
		return true
	}
	_, err := c.client.Get(key + c.namespace)
	exists := err == nil
	if exists {
		c.localCache.Add(key)
	}
	return exists
}

// Confirm we implement the interface.
var _ cache.Cache = (*Cache)(nil)
