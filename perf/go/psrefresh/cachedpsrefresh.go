package psrefresh

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/cache"
)

// CachedParamSetRefresher provides a struct to refresh paramsets and store them in the given cache.
type CachedParamSetRefresher struct {
	psRefresher *ParamSetRefresher
	cache       cache.Cache
}

// NewCachedParamSetRefresher returns a new instance of the CachedParamSetRefresher.
func NewCachedParamSetRefresher(psRefresher *ParamSetRefresher, cache cache.Cache) *CachedParamSetRefresher {
	return &CachedParamSetRefresher{
		psRefresher: psRefresher,
		cache:       cache,
	}
}

// Populate the cache with the paramsets.
func (c *CachedParamSetRefresher) PopulateCache(ctx context.Context) {
	cacheConfig := c.psRefresher.qConfig.RedisConfig

	fullps := c.psRefresher.Get()

	// Get the level 1 parameter key from config.
	// Return if it is not defined and nothing will be cached.
	lv1Key := cacheConfig.Level1Key
	if lv1Key == "" {
		sklog.Debug("Level1 key not defined.")
		return
	}

	// Get the possible values of level 1 parameter.
	// If the key is not in full paramset, it is invalid and return error.
	lv1Values, ok := fullps[lv1Key]
	if !ok {
		sklog.Errorf("No level1 key %s in paramset.", lv1Key)
		return
	}

	// Layer 1 looping on the level 1 parameter.
	for _, lv1Value := range lv1Values {
		if !ShouldCacheValue(lv1Value, cacheConfig.Level1Values) {
			continue
		}
		values := url.Values{lv1Key: []string{lv1Value}}
		c.psRefresher.UpdateQueryValueWithDefaults(values)
		lv1Query, err := query.New(values)
		if err != nil {
			sklog.Errorf("Can not parse level1 query values")
			return
		}
		count, lv1PS, err := c.psRefresher.dfBuilder.PreflightQuery(ctx, lv1Query, fullps)
		if err != nil {
			sklog.Error("Error on preflight query on level 1 key: %s", err.Error())
			return
		}
		sklog.Debugf("Preflightquery returns count: %d", count)
		paramSet := paramtools.ReadOnlyParamSet(lv1PS)
		b, err := json.Marshal(paramSet)
		if err != nil {
			sklog.Errorf("Error converting paramset to json: %v", err)
			return
		}
		err = c.cache.SetValue(ctx, lv1Value, string(b))
		if err != nil {
			sklog.Errorf("Error setting the value in cache: %v", err)
		}
	}
}

// StartRefreshRoutine starts a goroutine to refresh the paramsets in the cache.
func (c *CachedParamSetRefresher) StartRefreshRoutine(ctx context.Context, refreshPeriod time.Duration) {
	go func() {
		for range time.Tick(refreshPeriod) {
			c.PopulateCache(ctx)
		}
	}()
}
