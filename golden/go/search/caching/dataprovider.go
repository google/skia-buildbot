package caching

import (
	"context"
)

// cacheDataProvider provides an interface for cache data providers.
type cacheDataProvider interface {
	// GetCacheData returns the data.
	GetCacheData(ctx context.Context, firstCommitId string) (map[string]string, error)
}
