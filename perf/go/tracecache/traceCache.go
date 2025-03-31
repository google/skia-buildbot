package tracecache

import (
	"context"
	"encoding/json"
	"fmt"

	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/types"
)

// TraceCache provides a struct to cache trace information.
type TraceCache struct {
	cacheClient cache.Cache
}

// New returns a new TraceCache that can write and read from
// the provided cache instance.
func New(cache cache.Cache) *TraceCache {
	return &TraceCache{
		cacheClient: cache,
	}
}

// CacheTraceIds adds the provided traceIds to the cache.
func (t *TraceCache) CacheTraceIds(ctx context.Context, tileNumber types.TileNumber, q *query.Query, traceIds []paramtools.Params) error {
	cacheKey := traceIdCacheKey(tileNumber, *q)
	cacheValue, err := toJSON(traceIds)
	if err != nil {
		return err
	}

	return t.cacheClient.SetValue(ctx, cacheKey, cacheValue)
}

// GetTraceIds returns the traceIds for the given tile number and query from the cache.
func (t *TraceCache) GetTraceIds(ctx context.Context, tileNumber types.TileNumber, q *query.Query) ([]paramtools.Params, error) {
	cacheKey := traceIdCacheKey(tileNumber, *q)
	cacheJson, err := t.cacheClient.GetValue(ctx, cacheKey)
	if err != nil {
		return nil, err
	}

	if cacheJson == "" {
		return nil, nil
	}

	var traceIds []paramtools.Params
	err = json.Unmarshal([]byte(cacheJson), &traceIds)
	if err != nil {
		return nil, err
	}

	return traceIds, nil
}

// traceIdCacheKey returns a string key to use in the cache.
func traceIdCacheKey(tileNumber types.TileNumber, q query.Query) string {
	return fmt.Sprintf("%d_%s", tileNumber, q.KeyValueString())
}

// toJSON creates a json string from an object.
func toJSON(obj interface{}) (string, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
