package caching

import (
	"context"

	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/sql/schema"
)

// cacheDataProvider provides an interface for cache data providers.
type cacheDataProvider interface {
	// GetCacheData returns the data.
	GetCacheData(ctx context.Context, firstCommitId string) (map[string]string, error)

	// SetDatabaseType sets the database type for the current configuration.
	SetDatabaseType(dbType config.DatabaseType)

	// SetPublicTraces sets the given traces as the publicly visible ones.
	SetPublicTraces(traces map[schema.MD5Hash]struct{})
}
