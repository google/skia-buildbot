// Package local implements cache.Cache with an in-memory cache.
package local

import (
	"context"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/skerr"
)

// Cache implements the cache.Cache interface.
type Cache struct {
	cache *lru.Cache
}

// New returns a new in-memory cache of the given size.
func New(size int) (*Cache, error) {
	c, err := lru.New(size)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create local cache of size: %d", size)
	}
	return &Cache{
		cache: c,
	}, nil
}

// Add implements the cache.Cache interface.
func (c *Cache) Add(key string) {
	_ = c.cache.Add(key, 1)
}

// Exists implements the cache.Cache interface.
func (c *Cache) Exists(key string) bool {
	return c.cache.Contains(key)
}

func (c *Cache) SetValue(ctx context.Context, key string, value string) error {
	_ = c.cache.Add(key, value)
	return nil
}

func (c *Cache) SetValueWithExpiry(ctx context.Context, key string, value string, expiry time.Duration) error {
	// TTL is not currently supported in local cache.
	return c.SetValue(ctx, key, value)
}

func (c *Cache) GetValue(ctx context.Context, key string) (string, error) {
	value, ok := c.cache.Get(key)
	if !ok {
		return "", nil
	}
	return value.(string), nil
}

// Confirm we implement the interface.
var _ cache.Cache = (*Cache)(nil)
