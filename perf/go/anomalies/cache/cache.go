package cache

import (
	"context"
	"sort"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/perf/go/anomalies"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// cacheSize is the size of the lru cache.
const cacheSize = 25000

// How often to cleanup the lru cache.
const cacheCleanupPeriod = time.Minute

// TTL of the lru cache items.
const cacheItemTTL = 10 * time.Minute

// store implements anomalies.Store.
type store struct {
	cache *lru.Cache

	// metrics
	numEntriesInCache metrics2.Int64Metric

	ChromePerf anomalies.Store
}

type commitAnomalyMapCacheEntry struct {
	addTime          time.Time // When this entry was added.
	commitAnomalyMap anomalies.CommitNumberAnomalyMap
}

// New returns a new anomalies.Store instance with LRU cache.
func New(chromePerf anomalies.Store) (*store, error) {
	cache, err := lru.New(cacheSize)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create anomaly store cache.")
	}

	// cleanup the lru cache periodically.
	go func() {
		for range time.Tick(cacheCleanupPeriod) {
			cleanupCache(cache)
		}
	}()

	ret := &store{
		cache:             cache,
		numEntriesInCache: metrics2.GetInt64Metric("anomaly_store_num_entries_in_cache"),
		ChromePerf:        chromePerf,
	}
	return ret, nil
}

// cleanupCache cleans up the oldest cache items based on the cache item TTL.
func cleanupCache(cache *lru.Cache) {
	for {
		if cache.Len() == 0 {
			return
		}

		_, iOldestCommitAnomalyMapCacheEntry, ok := cache.GetOldest()
		if !ok {
			sklog.Warningf("Failed to get the oldest cache item.")
			return
		}

		oldestCommitAnomalyMapCacheEntry, ok := iOldestCommitAnomalyMapCacheEntry.(commitAnomalyMapCacheEntry)
		if oldestCommitAnomalyMapCacheEntry.addTime.Before(time.Now().Add(-cacheItemTTL)) {
			cache.RemoveOldest()
			sklog.Info("Cache item %s was removed from anomaly store cache.", oldestCommitAnomalyMapCacheEntry)
		} else {
			return
		}
	}
}

// GetAnomalies implements anomalies.Store
// It fetches anomalies from cache at first, then calls chrome perf API to fetch the anomlies missing from cache.
func (as *store) GetAnomalies(ctx context.Context, traceNames []string, startCommitPosition int, endCommitPosition int) (anomalies.AnomalyMap, error) {
	// Get anomalies from cache
	traceNamesMissingFromCache := make([]string, 0)
	result := anomalies.AnomalyMap{}
	numEntriesInCache := 0
	for _, traceName := range traceNames {
		iCommitAnomalyMapCacheEntry, ok := as.cache.Get(getAnomalyCacheKey(traceName, startCommitPosition, endCommitPosition))
		if !ok {
			traceNamesMissingFromCache = append(traceNamesMissingFromCache, traceName)
			continue
		}
		commitAnomalyMapCacheEntry, ok := iCommitAnomalyMapCacheEntry.(commitAnomalyMapCacheEntry)
		result[traceName] = commitAnomalyMapCacheEntry.commitAnomalyMap
		numEntriesInCache++
	}
	as.numEntriesInCache.Update(int64(numEntriesInCache))

	if len(traceNamesMissingFromCache) == 0 {
		return result, nil
	}

	// Get anomalies from Chrome Perf
	sort.Strings(traceNamesMissingFromCache)
	chromePerfAnomalies, err := as.ChromePerf.GetAnomalies(ctx, traceNamesMissingFromCache, startCommitPosition, endCommitPosition)
	if err != nil {
		sklog.Errorf("Failed to get chrome perf anomalies: %s", err)
	} else {
		for traceName, commitNumberAnomalyMap := range chromePerfAnomalies {
			result[traceName] = commitNumberAnomalyMap

			// Add anomalies to cache
			as.cache.Add(getAnomalyCacheKey(traceName, startCommitPosition, endCommitPosition), commitAnomalyMapCacheEntry{
				addTime:          time.Now(),
				commitAnomalyMap: commitNumberAnomalyMap,
			})
		}
	}

	return result, nil
}

// Anomaly cache key is "traceName:startCommitPosition:endCommitPosition"
func getAnomalyCacheKey(traceName string, startCommitPosition int, endCommitPosition int) string {
	return traceName + ":" + strconv.Itoa(startCommitPosition) + ":" + strconv.Itoa(endCommitPosition)
}
