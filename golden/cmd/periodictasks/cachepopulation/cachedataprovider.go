package cachepopulation

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/golden/go/config"
)

// CacheDataProvider provides an interface to define cache data providers.
// The data returned is used to populate the cache for their respective purpose.
type CacheDataProvider interface {
	// GetData returns a key and value to insert into the cache.
	// If there was any error during execution, the error object will be returned.
	GetData(context.Context) (string, string, error)
}

// GetAllCacheDataProviders returns a list of all available cache data providers.
func GetAllCacheDataProviders(db *pgxpool.Pool, searchCacheconfig config.SearchCacheConfig) []CacheDataProvider {
	return []CacheDataProvider{
		newSearchCacheDataProvider(db, searchCacheconfig),
	}
}

// toJSON creates a json string from an object.
func toJSON(obj interface{}) (string, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
