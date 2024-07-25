// Cache populator provides tooling to populate the cache for Gold.
// It consists of multiple CacheDataProvider implementations that are invoked by the cachePopulator
// to get the key-value pairs to be written into the cache.
package cachepopulation

import (
	"context"

	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/sklog"
)

// cachePopulator provides a struct for cache population.
type cachePopulator struct {
	// Cache client to interact with the cache.
	cacheClient cache.Cache

	// cacheDataProviders are a list of data providers to use for getting the data for writing to the cache.
	cacheDataProviders []CacheDataProvider
}

// NewCachePopulator returns a new cachePopulator instance.
func NewCachePopulator(cacheClient cache.Cache, jobs []CacheDataProvider) *cachePopulator {
	return &cachePopulator{
		cacheClient:        cacheClient,
		cacheDataProviders: jobs,
	}
}

// Start starts the cache population process.
func (cp *cachePopulator) Start(ctx context.Context) {
	for _, job := range cp.cacheDataProviders {
		// Run cache population job.
		cacheKey, cacheValue, err := job.GetData(ctx)
		if err != nil {
			sklog.Errorf("Error running cache population job: %v", err)
			continue
		}

		// Set the result json in the cache.
		err = cp.cacheClient.SetValue(ctx, cacheKey, cacheValue)
		if err != nil {
			sklog.Errorf("Error writing to cache: %v", err)
		}
	}
}
