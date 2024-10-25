package caching

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/skerr"
)

// SearchCacheManager provides a struct to handle the cache operations for gold search.
type SearchCacheManager struct {
	cacheClient  cache.Cache
	db           *pgxpool.Pool
	corpora      []string
	commitWindow int
}

// New returns a new instance of the SearchCacheManager.
func New(cacheClient cache.Cache, db *pgxpool.Pool, corpora []string, commitWindow int) *SearchCacheManager {
	return &SearchCacheManager{
		cacheClient:  cacheClient,
		db:           db,
		corpora:      corpora,
		commitWindow: commitWindow,
	}
}

// getCacheDataProviders returns a list of cacheDataProviders
func (s SearchCacheManager) getCacheDataProviders() []cacheDataProvider {
	return []cacheDataProvider{
		NewByBlameDataProvider(s.db, s.corpora, s.commitWindow),
	}
}

// RunCachePopulation gets the cache data from the providers and stores it in the cache instance.
func (s SearchCacheManager) RunCachePopulation(ctx context.Context) error {
	providers := s.getCacheDataProviders()
	for _, prov := range providers {
		data, err := prov.GetCacheData(ctx)
		if err != nil {
			return skerr.Wrapf(err, "Error while running cache population with provider %s", prov)
		}

		for key, val := range data {
			err := s.cacheClient.SetValue(ctx, key, val)
			if err != nil {
				return skerr.Wrapf(err, "Error while setting value in cache.")
			}
		}
	}

	return nil
}
