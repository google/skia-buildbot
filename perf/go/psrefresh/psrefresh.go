package psrefresh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

// OPSProvider allows access to OrdererParamSets. TraceStore implements this interface.
type OPSProvider interface {
	GetLatestTile(context.Context) (types.TileNumber, error)
	GetParamSet(ctx context.Context, tileNumber types.TileNumber) (paramtools.ReadOnlyParamSet, error)
}

// ParamSetRefresher keeps a fresh paramtools.ParamSet that represents all the
// traces stored in the two most recent tiles in the trace store.
type ParamSetRefresher struct {
	traceStore   OPSProvider
	period       time.Duration
	numParamSets int
	dfBuilder    dataframe.DataFrameBuilder
	qConfig      config.QueryConfig

	mutex      sync.Mutex // protects ps.
	ps         paramtools.ReadOnlyParamSet
	queryCache map[string]paramtools.ReadOnlyParamSet
	countCache map[string]int64
}

// NewParamSetRefresher builds a new *ParamSetRefresher.
func NewParamSetRefresher(traceStore OPSProvider, numParamSets int, dfBuilder dataframe.DataFrameBuilder, qconfig config.QueryConfig) *ParamSetRefresher {
	return &ParamSetRefresher{
		traceStore:   traceStore,
		numParamSets: numParamSets,
		dfBuilder:    dfBuilder,
		qConfig:      qconfig,
		ps:           paramtools.ReadOnlyParamSet{},
		countCache:   map[string]int64{},
		queryCache:   map[string]paramtools.ReadOnlyParamSet{},
	}
}

// Start actually starts the refreshing process.
//
// The 'period' is how often the paramset should be refreshed.
func (pf *ParamSetRefresher) Start(period time.Duration) error {
	pf.period = period
	sklog.Info("Refresher refreshing")

	if err := pf.oneStep(false); err != nil {
		return skerr.Wrapf(err, "Failed to build the initial ParamSet")
	}
	go pf.refresh()
	return nil

}

// Cache query results to reduce the UI latencey.
// This logic is specifically for the UI update for Chromeperf, where parameter
// evalutions are ordered, and only one value is allowed per parameter.
// It has two layers of loops.
// In layer 1, it use the level 1 parameter values from ps (full paramset) to
// generate queries to perform the preflightquery, which is used to retreive the
// paramset for end users in query UI. Each parameter value is used as key to
// map to the results returned by preflightquery.
// In layer 2, the queries uses the combination of the current level 1 parameter
// and each of the level 2 parameters. The key will be the concat of both parameters.
//
// E.g., if the ps is:
//
//	{"benchmark":["startup", "v8"],"bot":["linux","mac"], "test"...}
//
// and the level1 key and the level2 key are "benchmark" and "bot" (defined in config)
// we should expect the cache to have the following keys:
//
//	"startup", "startup&linux", "startup&mac", "v8", "v8&linux", "v8&mac"
func (pf *ParamSetRefresher) cacheQueryResults(ctx context.Context, ps paramtools.ParamSet) (map[string]int64, map[string]paramtools.ReadOnlyParamSet, error) {
	// get timer only for the extra caching for query results
	t := metrics2.NewTimer("QueryUICache")
	countCache := map[string]int64{}
	queryCache := map[string]paramtools.ReadOnlyParamSet{}

	fullps := ps.Freeze()

	// Get the level 1 parameter key from config.
	// Return if it is not defined and nothing will be cached.
	lv1Key := pf.qConfig.RedisConfig.Level1Key
	if lv1Key == "" {
		sklog.Debug("Level1 key not defined.")
		return countCache, queryCache, nil
	}
	sklog.Debugf("Level1 key: %s", lv1Key)

	// Get the possible values of level 1 parameter.
	// If the key is not in full paramset, it is invalid and return error.
	lv1Values, ok := fullps[lv1Key]
	if !ok {
		sklog.Errorf("No level1 key %s in paramset.", lv1Key)
		return nil, nil, errors.New("level1 key not in full paramset")
	}

	// Layer 1 looping on the level 1 parameter.
	for _, lv1Value := range lv1Values {
		if !IsValidValue(lv1Value, pf.qConfig.RedisConfig.Level1Values) {
			continue
		}
		sklog.Debugf("Query on: %s:%s", lv1Key, lv1Value)
		values := url.Values{lv1Key: []string{lv1Value}}
		pf.UpdateQueryValueWithDefaults(values)
		sklog.Debugf("Query values: %s", values)
		lv1Query, err := query.New(values)
		if err != nil {
			sklog.Errorf("Can not parse level1 query values")
			return nil, nil, err
		}
		count, lv1PS, err := pf.dfBuilder.PreflightQuery(ctx, lv1Query, fullps)
		if err != nil {
			sklog.Error("Error on preflight query on level 1 key: %s", err.Error())
			return nil, nil, err
		}
		sklog.Debugf("Preflightquery returns count: %d", count)

		countCache[lv1Value] = count
		queryCache[lv1Value] = paramtools.ReadOnlyParamSet(lv1PS)

		// Get the level 2 parameter key from config.
		// Continue if it is not defined. We should keep on caching
		// the remaining values on level 1
		lv2Key := pf.qConfig.RedisConfig.Level2Key
		if lv2Key == "" {
			sklog.Debug("Level2 key not defined.")
			continue
		}
		// Get the possible values of level 2 parameter.
		// If the key is not in full paramset, it is invalid and return error.
		lv2Values, ok := lv1PS[lv2Key]
		if !ok {
			sklog.Errorf("No level2 key %s in paramset.", lv2Key)
			return nil, nil, errors.New("level2 key error")
		}
		// Layer 2 looping on the level 2 parameter.
		for _, lv2Value := range lv2Values {
			if !IsValidValue(lv2Value, pf.qConfig.RedisConfig.Level2Values) {
				continue
			}
			sklog.Debugf("Query on: %s:%s, %s:%s", lv1Key, lv1Value, lv2Key, lv2Value)
			values := url.Values{lv1Key: []string{lv1Value}, lv2Key: []string{lv2Value}}
			pf.UpdateQueryValueWithDefaults(values)
			sklog.Debugf("Query values: %s", values)
			lv2Query, err := query.New(values)
			if err != nil {
				sklog.Errorf("Can not parse level2 query values")
				return nil, nil, err
			}

			count, lv2PS, err := pf.dfBuilder.PreflightQuery(ctx, lv2Query, fullps)
			if err != nil {
				sklog.Error("Error on preflight query on level 2 key: %s", err.Error())
				return nil, nil, err
			}
			sklog.Infof("LV2 Preflightquery returns count: %d", count)

			key := fmt.Sprintf("%s&%s", lv1Value, lv2Value)
			countCache[key] = count
			queryCache[key] = paramtools.ReadOnlyParamSet(lv2PS)
		}
	}
	t.Stop()

	return countCache, queryCache, nil
}

func (pf *ParamSetRefresher) oneStep(cacheQuery bool) error {
	sklog.Debugf("Current Cached Count: %s", pf.countCache)
	cachej, errj := json.Marshal(pf.queryCache)
	if errj == nil {
		sklog.Debugf("Current Cached size: %d", len(cachej))
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancel()

	tileKey, err := pf.traceStore.GetLatestTile(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed to get starting tile.")
	}
	ps := paramtools.NewParamSet()
	first := true
	for i := 0; i < pf.numParamSets; i++ {
		ps1, err := pf.traceStore.GetParamSet(ctx, tileKey)
		if err != nil {
			if first {
				// Only the failing on the first tile should be an error,
				// previous tiles may be empty, or invalid.
				return skerr.Wrapf(err, "Failed to get paramset from first tile.")
			}
			sklog.Warningf("Failed to get paramset from %d most recent tile: %s", i, err)
		}
		first = false
		ps.AddParamSet(ps1)
		tileKey = tileKey.Prev()
	}

	ps.Normalize()

	countCache := map[string]int64{}
	queryCache := map[string]paramtools.ReadOnlyParamSet{}
	var queryCacheErr error
	if cacheQuery && pf.qConfig.RedisConfig.Enabled {
		countCache, queryCache, queryCacheErr = pf.cacheQueryResults(ctx, ps)
	}

	pf.mutex.Lock()
	defer pf.mutex.Unlock()

	pf.ps = ps.Freeze()
	if queryCacheErr != nil {
		pf.countCache = map[string]int64{}
		pf.queryCache = map[string]paramtools.ReadOnlyParamSet{}
	} else {
		pf.countCache = countCache
		pf.queryCache = queryCache
	}
	return nil
}

func (pf *ParamSetRefresher) refresh() {
	stepFailures := metrics2.GetCounter("paramset_refresh_failures", nil)
	for range time.Tick(pf.period) {
		if err := pf.oneStep(true); err != nil {
			sklog.Errorf("Failed to refresh the ParamSet: %s", err)
			stepFailures.Inc(1)
		}
	}
}

// Get returns the fresh paramset.
func (pf *ParamSetRefresher) Get() paramtools.ReadOnlyParamSet {
	pf.mutex.Lock()
	defer pf.mutex.Unlock()
	return pf.ps
}

// GetQuery returns the cached query results.
// It generates a key from the input value q, only if q has 1 or 2 parameters.
// It returns the cached result count and result if the key is in the cache.
func (pf *ParamSetRefresher) GetQuery(q url.Values) (int64, paramtools.ReadOnlyParamSet) {
	sklog.Debugf("GetQuery on values: %s", q)
	qlen := len(q)
	if len(pf.qConfig.DefaultParamSelections) > 0 {
		sklog.Debugf("Found default params: %s: ", pf.qConfig.DefaultParamSelections)
		qlen -= len(pf.qConfig.DefaultParamSelections)
	}
	key := ""
	if qlen > 2 {
		// We don't cache query results with more than 2 parameters
		return 0, nil
	} else if qlen == 1 {
		v1, ok := q[pf.qConfig.RedisConfig.Level1Key]
		if !ok {
			return 0, nil
		}
		key = v1[0]
	} else if qlen == 2 {
		v1, ok1 := q[pf.qConfig.RedisConfig.Level1Key]
		v2, ok2 := q[pf.qConfig.RedisConfig.Level2Key]
		if !ok1 || !ok2 {
			return 0, nil
		}
		key = fmt.Sprintf("%s&%s", v1[0], v2[0])
	} else {
		return 0, nil
	}

	pf.mutex.Lock()
	defer pf.mutex.Unlock()

	sklog.Debugf("GetQuery on key: %s", key)
	count, ok1 := pf.countCache[key]
	ps, ok2 := pf.queryCache[key]
	if !ok1 || !ok2 {
		return 0, nil
	}
	return count, ps
}

// append the default values for parameters
func (pf *ParamSetRefresher) UpdateQueryValueWithDefaults(v url.Values) {
	if len(pf.qConfig.DefaultParamSelections) > 0 {
		for key, values := range pf.qConfig.DefaultParamSelections {
			v[key] = values
		}
	}
}

// check whether value is part of the list validValues
func IsValidValue(value string, validValues []string) bool {
	for _, validValue := range validValues {
		if value == validValue {
			return true
		}
	}
	return false
}
