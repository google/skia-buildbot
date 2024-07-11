package psrefresh

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
)

// CachedParamSetRefresher provides a struct to refresh paramsets and store them in the given cache.
type CachedParamSetRefresher struct {
	psRefresher *defaultParamSetRefresher
	cache       cache.Cache
}

// NewCachedParamSetRefresher returns a new instance of the CachedParamSetRefresher.
func NewCachedParamSetRefresher(psRefresher *defaultParamSetRefresher, cache cache.Cache) *CachedParamSetRefresher {
	return &CachedParamSetRefresher{
		psRefresher: psRefresher,
		cache:       cache,
	}
}

// Populate the cache with the paramsets.
func (c *CachedParamSetRefresher) PopulateCache() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour*2)
	defer cancel()
	cacheConfig := c.psRefresher.qConfig.CacheConfig

	fullps := c.psRefresher.GetAll()

	// Get the level 1 parameter key from config.
	// Return if it is not defined and nothing will be cached.
	lv1Key := cacheConfig.Level1Key
	if lv1Key == "" {
		sklog.Debug("Level1 key not defined.")
		return
	}

	sklog.Info("Starting populating the query cache.")
	// Populate Level 1 onwards.
	c.populateLevels(ctx, lv1Key, cacheConfig.Level1Values, fullps)
	sklog.Info("Finished populating the query cache.")
}

// populateChildLevel adds the child level filtered paramset data into the cache.
func (c *CachedParamSetRefresher) populateChildLevel(ctx context.Context, parentKey string, parentValue string, paramSet paramtools.ReadOnlyParamSet, childLevelKey string, childLevelValues []string) error {
	availableValues, ok := paramSet[childLevelKey]
	if !ok {
		return skerr.Fmt("No level key %s in paramset.", childLevelKey)
	}

	if len(childLevelValues) == 0 {
		// If no values are provided, let's look at all values.
		childLevelValues = availableValues
	}
	for _, value := range childLevelValues {
		if slices.Contains(availableValues, value) {
			qValues := url.Values{parentKey: []string{parentValue}, childLevelKey: []string{value}}
			c.psRefresher.UpdateQueryValueWithDefaults(qValues)
			lv2Query, err := query.New(qValues)
			if err != nil {
				sklog.Errorf("Can not parse child query values")
				return err
			}

			count, filteredPS, err := c.psRefresher.dfBuilder.PreflightQuery(ctx, lv2Query, paramSet)
			if err != nil {
				sklog.Error("Error on preflight query on level 2 key: %s", err.Error())
				return err
			}
			sklog.Infof("Child level Preflightquery returns count: %d", count)
			childParamset := paramtools.ReadOnlyParamSet(filteredPS)
			cacheValue, err := childParamset.ToString()
			if err != nil {
				sklog.Errorf("Error converting paramset to json: %v", err)
				return err
			}
			psCacheKey, ok := paramSetKey(qValues, []string{parentKey, childLevelKey})
			if !ok {
				sklog.Errorf("Error creating child psCacheKey for %s, %s for values %v", parentKey, childLevelKey, qValues)
				return skerr.Fmt("Error creating psCacheKey for %s, %s for values %v", parentKey, childLevelKey, qValues)
			}

			sklog.Infof("Adding %s: %s to child cache", psCacheKey, cacheValue)
			c.addToCache(ctx, psCacheKey, cacheValue, count)
		}
	}

	return nil
}

// populateLevels adds the specified level paramset and count data into the cache.
// If there are child levels specified, it will further drill down and populate the child levels as well.
func (c *CachedParamSetRefresher) populateLevels(ctx context.Context, levelKey string, levelValues []string, fullPS paramtools.ReadOnlyParamSet) {
	sklog.Infof("Populating cache for key %s", levelKey)
	availableValues, ok := fullPS[levelKey]
	if !ok {
		sklog.Errorf("No level key %s in paramset.", levelKey)
		return
	}
	if len(levelValues) == 0 {
		// If no values are provided, let's look at all values.
		levelValues = availableValues
	}
	// Range through the values provided.
	for _, value := range levelValues {
		// If the provided value is actually available in the paramset.
		if slices.Contains(availableValues, value) {
			qValues := url.Values{levelKey: []string{value}}
			c.psRefresher.UpdateQueryValueWithDefaults(qValues)
			query, err := query.New(qValues)
			if err != nil {
				sklog.Errorf("Can not parse query values: %v", err)
				return
			}
			count, filteredPS, err := c.psRefresher.dfBuilder.PreflightQuery(ctx, query, fullPS)
			if err != nil {
				sklog.Error("Error on preflight query on level 1 (%s=%s): %v", levelKey, value, err)
				return
			}
			sklog.Debugf("Preflightquery returns count: %d", count)
			paramSet := paramtools.ReadOnlyParamSet(filteredPS)
			cacheValue, err := paramSet.ToString()
			if err != nil {
				sklog.Errorf("Error converting paramset to json: %v", err)
				return
			}

			psCacheKey, ok := paramSetKey(qValues, []string{levelKey})
			if !ok {
				sklog.Errorf("Error creating psCacheKey for %s for values %v", levelKey, qValues)
				return
			}
			c.addToCache(ctx, psCacheKey, cacheValue, count)

			// Now let's populate the relevant child levels for this param.
			if c.psRefresher.qConfig.CacheConfig.Level2Key != "" {
				err := c.populateChildLevel(ctx, levelKey, value, paramSet, c.psRefresher.qConfig.CacheConfig.Level2Key, c.psRefresher.qConfig.CacheConfig.Level2Values)
				if err != nil {
					sklog.Errorf("Error while populating child level for parent %s", levelKey)
				}
			}
		}
	}
}

// addToCache adds the ps data and count to the cache
func (c *CachedParamSetRefresher) addToCache(ctx context.Context, psCacheKey string, psCacheValue string, count int64) {
	sklog.Infof("Adding filtered paramsets for key %s into the cache.", psCacheKey)
	err := c.cache.SetValue(ctx, psCacheKey, psCacheValue)
	if err != nil {
		sklog.Errorf("Error setting the value in cache: %v", err)
	}

	countKey := countKey(psCacheKey)
	sklog.Infof("Adding count data for key %s into the cache.", countKey)
	err = c.cache.SetValue(ctx, countKey, strconv.FormatInt(count, 10))
	if err != nil {
		sklog.Errorf("Error setting the count value in cache: %v", err)
	}
}

// GetAll returns the entire ParamSet for the instance.
func (c *CachedParamSetRefresher) GetAll() paramtools.ReadOnlyParamSet {
	return c.psRefresher.GetAll()
}

// GetParamSetForQuery returns the trace count and paramset for the given query.
func (c *CachedParamSetRefresher) GetParamSetForQuery(ctx context.Context, query *query.Query, q url.Values) (int64, paramtools.ParamSet, error) {
	count, filteredPS, err := c.getParamSetForQueryInternal(ctx, query, q)
	if err != nil || filteredPS == nil {
		// If there was an error getting the data from cache or data was not found in cache
		// let's give it a try to get it from the db.
		return c.psRefresher.GetParamSetForQuery(ctx, query, q)
	}

	return count, filteredPS, err
}

func (c *CachedParamSetRefresher) getParamSetForQueryInternal(ctx context.Context, query *query.Query, q url.Values) (int64, paramtools.ParamSet, error) {
	sklog.Debugf("GetParamSetForQuery on values: %s", q)
	qlen := len(q)
	if len(c.psRefresher.qConfig.DefaultParamSelections) > 0 {
		sklog.Debugf("Found default params: %s: ", c.psRefresher.qConfig.DefaultParamSelections)
		qlen -= len(c.psRefresher.qConfig.DefaultParamSelections)
	}
	key := ""
	ok := false
	switch qlen {
	case 1:
		key, ok = paramSetKey(q, []string{c.psRefresher.qConfig.CacheConfig.Level1Key})
		if !ok {
			return 0, nil, nil
		}
	case 2:
		key, ok = paramSetKey(q, []string{c.psRefresher.qConfig.CacheConfig.Level1Key, c.psRefresher.qConfig.CacheConfig.Level2Key})
		if !ok {
			return 0, nil, nil
		}
	default:
		// We don't cache query results with more than 2 parameters,
		// so let's do a full search instead.
		return c.psRefresher.GetParamSetForQuery(ctx, query, q)
	}

	cacheValue, err := c.cache.GetValue(ctx, key)
	if err != nil {
		return 0, nil, err
	}

	if cacheValue != "" {
		paramset, err := paramtools.FromString(cacheValue)

		if err != nil {
			return 0, nil, err
		}
		countStr, err := c.cache.GetValue(ctx, countKey(key))
		var count int64
		if countStr != "" {
			count, err = strconv.ParseInt(countStr, 10, 64)
		}

		sklog.Infof("Cache hit for paramset key %s", key)
		return count, paramset, err
	}

	// If nothing has been found in cache, let's default to getting it from the regular refresher.
	sklog.Infof("Cache miss for paramset key %s", key)
	return c.psRefresher.GetParamSetForQuery(ctx, query, q)
}

// Start the refresher.
func (c *CachedParamSetRefresher) Start(period time.Duration) error {
	err := c.psRefresher.Start(period)
	if c.psRefresher.qConfig.CacheConfig.Type == config.LocalCache {
		sklog.Infof("Starting the refresh routine on %v", period)
		c.StartRefreshRoutine(period)
	}
	return err
}

// StartRefreshRoutine starts a goroutine to refresh the paramsets in the cache.
func (c *CachedParamSetRefresher) StartRefreshRoutine(refreshPeriod time.Duration) {
	c.PopulateCache()
	go func() {
		for range time.Tick(refreshPeriod) {
			c.PopulateCache()
		}
	}()
}

// paramSetKey returns a string key to be used for paramset data in the cache.
func paramSetKey(q url.Values, paramKeys []string) (string, bool) {
	paramSetStrings := []string{}
	for _, key := range paramKeys {
		paramVal, ok := q[key]
		if !ok {
			sklog.Errorf("Key %s not present in query values %v", key, q)
			return "", ok
		}
		paramSetStrings = append(paramSetStrings, fmt.Sprintf("%s=%s", key, paramVal))
	}

	return strings.Join(paramSetStrings, "&"), true
}

// countKey returns a key to store the count value in the cache.
func countKey(psCacheKey string) string {
	return fmt.Sprintf("count_%s", psCacheKey)
}

var _ ParamSetRefresher = (*CachedParamSetRefresher)(nil)
