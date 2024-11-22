package cache

import (
	"context"
	"sort"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/perf/go/chromeperf"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// cacheSize is the size of the lru cache.
const cacheSize = 25000

// How often to cleanup the lru cache.
const cacheCleanupPeriod = time.Minute

// TTL of the lru cache items.
const cacheItemTTL = 10 * time.Minute

// How often to wipe out the invalidation map.
const invalidationCleanupPeriod = 24 * time.Hour

// store implements anomalies.Store.
type store struct {
	// testsCache contains data for the tests-minRevision-maxRevision requests.
	testsCache *lru.Cache

	// revisionCache contains data for anomalies around a revision requests.
	revisionCache *lru.Cache

	// invalidationMap that let's us know which traces have been modified and therefore,
	// invalidated from the cache. This map is deleted every 24 hours to prevent it from growing
	// too big. We should go back and revisit if this period is too short or too long.
	// Pros:
	// - Solution is simple.
	// - Minimal additional memory.
	// - Minimal additional runtime, all O(1) operations.
	// Cons:
	// - Not accurate. Modifying an anomaly, will invalidate all anomalies that share the same
	//   trace.
	// - One can get unlucky by modifying an anomaly and the cleanUp happens right after.
	// We landed on this approach as other solutions would require significantly more memory or
	// were less accurate. This can be re-evaluated later on.
	invalidationMap map[string]bool

	// metrics
	numEntriesInCache metrics2.Int64Metric

	ChromePerf chromeperf.AnomalyApiClient
}

type commitAnomalyMapCacheEntry struct {
	addTime          time.Time // When this entry was added.
	commitAnomalyMap chromeperf.CommitNumberAnomalyMap
}

// New returns a new anomalies.Store instance with LRU cache.
func New(chromePerf chromeperf.AnomalyApiClient) (*store, error) {
	testsCache, err := lru.New(cacheSize)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create anomaly store tests cache.")
	}

	revisionCache, err := lru.New(cacheSize)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create anomaly store revisions cache.")
	}

	invalidationMap := make(map[string]bool)

	// cleanup the lru cache periodically.
	go func() {
		for range time.Tick(cacheCleanupPeriod) {
			cleanupCache(testsCache)
			cleanupCache(revisionCache)
		}
	}()

	// Clean up invalidation map. Re-assigning it to a new map will leave the old map with no references
	// and the garbage collector will eventually clean it.
	go func() {
		for range time.Tick(invalidationCleanupPeriod) {
			sklog.Debug("[SkiaTriage] Clearing Invalidation map with size: %d", len(invalidationMap))
			invalidationMap = make(map[string]bool)
		}
	}()

	ret := &store{
		testsCache:        testsCache,
		revisionCache:     revisionCache,
		invalidationMap:   invalidationMap,
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

		oldestCommitAnomalyMapCacheEntry, _ := iOldestCommitAnomalyMapCacheEntry.(commitAnomalyMapCacheEntry)
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
func (as *store) GetAnomalies(ctx context.Context, traceNames []string, startCommitPosition int, endCommitPosition int) (chromeperf.AnomalyMap, error) {
	// Get anomalies from cache
	traceNamesMissingFromCache := make([]string, 0)
	result := chromeperf.AnomalyMap{}
	numEntriesInCache := 0
	for _, traceName := range traceNames {
		iCommitAnomalyMapCacheEntry, hitCache := as.testsCache.Get(getAnomalyCacheKey(traceName, startCommitPosition, endCommitPosition))
		_, invalidated := as.invalidationMap[traceName]
		// If Trace exists in the invalidationMap, the trace has been modified and needs to be re-fetched from Chromeperf.
		if !hitCache || invalidated {
			traceNamesMissingFromCache = append(traceNamesMissingFromCache, traceName)
			continue
		}
		commitAnomalyMapCacheEntry, _ := iCommitAnomalyMapCacheEntry.(commitAnomalyMapCacheEntry)
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
			cacheKey := getAnomalyCacheKey(traceName, startCommitPosition, endCommitPosition)
			sklog.Debugf("[SkiaTriage] Adding entry to testsCache with key: %s", cacheKey)
			// Add anomalies to cache
			as.testsCache.Add(cacheKey, commitAnomalyMapCacheEntry{
				addTime:          time.Now(),
				commitAnomalyMap: commitNumberAnomalyMap,
			})
		}
	}

	return result, nil
}

// GetAnomaliesTimeBased implements anomalies.Store
// Retrieves anomalies for each trace within the begin and end times.
func (as *store) GetAnomaliesInTimeRange(ctx context.Context, traceNames []string, startTime time.Time, endTime time.Time) (chromeperf.AnomalyMap, error) {
	result := chromeperf.AnomalyMap{}
	if len(traceNames) == 0 {
		return result, nil
	}

	sort.Strings(traceNames)

	chromePerfAnomalies, err := as.ChromePerf.GetAnomaliesTimeBased(ctx, traceNames, startTime, endTime)
	if err != nil {
		sklog.Errorf("Failed to get chrome perf anomalies: %s", err)
	} else {
		for traceName, commitNumberAnomalyMap := range chromePerfAnomalies {
			result[traceName] = commitNumberAnomalyMap
		}
	}

	return result, nil
}

// GetAnomaliesAroundRevision implements anomalies.Store
// It fetches anomalies that occured around the specified revision number.
func (as *store) GetAnomaliesAroundRevision(ctx context.Context, revision int) ([]chromeperf.AnomalyForRevision, error) {
	iAnomalies, ok := as.revisionCache.Get(revision)
	if ok {
		return iAnomalies.([]chromeperf.AnomalyForRevision), nil
	} else {
		result, err := as.ChromePerf.GetAnomaliesAroundRevision(ctx, revision)
		if err != nil {
			return nil, err
		}
		as.revisionCache.Add(revision, result)
		return result, nil
	}
}

// InvalidateTestsCacheForTraceName implements anomalies.Store
// Invalidates a specific traceName from the tests cache.
func (as *store) InvalidateTestsCacheForTraceName(ctx context.Context, traceName string) {
	sklog.Debugf("[SkiaTriage] Adding entry to invalidationMap with trace name: %s", traceName)
	as.invalidationMap[traceName] = true
	sklog.Debug("[SkiaTriage] Invalidation map size: %d", len(as.invalidationMap))
}

// Anomaly cache key is "traceName:startCommitPosition:endCommitPosition"
func getAnomalyCacheKey(traceName string, startCommitPosition int, endCommitPosition int) string {
	return traceName + ":" + strconv.Itoa(startCommitPosition) + ":" + strconv.Itoa(endCommitPosition)
}
