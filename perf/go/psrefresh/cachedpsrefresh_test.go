// Copyright 2024 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package psrefresh

import (
	"context"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/cache/local"
	mockCache "go.skia.org/infra/go/cache/mock"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/config"
	dfb "go.skia.org/infra/perf/go/dataframe/mocks"
	"go.skia.org/infra/perf/go/psrefresh/mocks"
	"go.skia.org/infra/perf/go/types"
)

func TestPopulateCache_Success(t *testing.T) {
	pf := getPsRefresher(nil, nil, nil)
	cache := mockCache.NewCache(t)
	cacheKey, _ := paramSetKey(url.Values{"config": []string{"8888"}}, []string{"config"})
	cache.On("SetValue", mock.Anything, cacheKey, mock.Anything).Return(nil)
	cache.On("SetValue", mock.Anything, countKey(cacheKey), mock.Anything).Return(nil)
	refresher := NewCachedParamSetRefresher(pf, cache)

	refresher.PopulateCache()
	cache.AssertNumberOfCalls(t, "SetValue", 2)
}

func TestPopulateCache_InvalidValue(t *testing.T) {
	pf := getPsRefresher(nil, nil, nil)
	pf.qConfig.CacheConfig.Level1Values = []string{"NonExistingValue"}
	cache := mockCache.NewCache(t)
	refresher := NewCachedParamSetRefresher(pf, cache)

	refresher.PopulateCache()
	cache.AssertNotCalled(t, "SetValue")
}

func TestPopulateAndRetrieveLocalCache_Success(t *testing.T) {
	ps := &paramtools.ReadOnlyParamSet{
		"config": []string{"8888"},
		"test":   []string{"t1", "t2"},
	}

	l1QueryValues := url.Values{
		"config": []string{"8888"},
	}
	l1Query, err := query.New(l1QueryValues)
	require.NoError(t, err)

	l2Query1Values := url.Values{
		"config": []string{"8888"},
		"test":   []string{"t1"},
	}
	l2Query1, err := query.New(l2Query1Values)
	require.NoError(t, err)
	l2Query2Values := url.Values{
		"config": []string{"8888"},
		"test":   []string{"t2"},
	}
	l2Query2, err := query.New(l2Query2Values)
	require.NoError(t, err)
	dfbMock := &dfb.DataFrameBuilder{}
	dfbMock.On("PreflightQuery", mock.Anything, l1Query, mock.Anything).Return(
		int64(2), paramtools.ParamSet{"test": []string{"t1", "t2"}}, nil)
	dfbMock.On("PreflightQuery", mock.Anything, l2Query1, mock.Anything).Return(
		int64(1), paramtools.ParamSet{"test": []string{}}, nil)
	dfbMock.On("PreflightQuery", mock.Anything, l2Query2, mock.Anything).Return(
		int64(1), paramtools.ParamSet{"test": []string{}}, nil)

	cacheConfig := &config.QueryCacheConfig{
		Type:      config.LocalCache,
		Level1Key: "config",
		Level2Key: "test",
		Enabled:   true,
	}
	pf := getPsRefresher(ps, cacheConfig, dfbMock)
	cache, err := local.New(5)
	require.NoError(t, err)
	refresher := NewCachedParamSetRefresher(pf, cache)
	refresher.PopulateCache()

	ctx := context.Background()

	// Check if items in level 1 have been populated.
	l1CacheKey, _ := paramSetKey(l1QueryValues, []string{"config"})
	assertCacheHit(t, ctx, cache, l1CacheKey, 2)

	// Check if items in level2 have been populated.
	l2CacheKey1, _ := paramSetKey(l2Query1Values, []string{"config", "test"})
	assertCacheHit(t, ctx, cache, l2CacheKey1, 1)
	l2CacheKey2, _ := paramSetKey(l2Query2Values, []string{"config", "test"})
	assertCacheHit(t, ctx, cache, l2CacheKey2, 1)
}

func TestPopulateAndRetrieveLocalCacheOnly1Level_Success(t *testing.T) {
	ps := &paramtools.ReadOnlyParamSet{
		"config": []string{"8888"},
		"test":   []string{"t1", "t2"},
	}

	l1QueryValues := url.Values{
		"config": []string{"8888"},
	}
	l1Query, err := query.New(l1QueryValues)
	require.NoError(t, err)

	l2Query1Values := url.Values{
		"config": []string{"8888"},
		"test":   []string{"t1"},
	}
	l2Query1, err := query.New(l2Query1Values)
	require.NoError(t, err)
	l2Query2Values := url.Values{
		"config": []string{"8888"},
		"test":   []string{"t2"},
	}
	l2Query2, err := query.New(l2Query2Values)
	require.NoError(t, err)
	dfbMock := &dfb.DataFrameBuilder{}
	dfbMock.On("PreflightQuery", mock.Anything, l1Query, mock.Anything).Return(
		int64(2), paramtools.ParamSet{"test": []string{"t1", "t2"}}, nil)
	dfbMock.On("PreflightQuery", mock.Anything, l2Query1, mock.Anything).Return(
		int64(1), paramtools.ParamSet{"test": []string{}}, nil)
	dfbMock.On("PreflightQuery", mock.Anything, l2Query2, mock.Anything).Return(
		int64(1), paramtools.ParamSet{"test": []string{}}, nil)

	// Specify only level1 key.
	cacheConfig := &config.QueryCacheConfig{
		Type:      config.LocalCache,
		Level1Key: "config",
		Enabled:   true,
	}
	pf := getPsRefresher(ps, cacheConfig, dfbMock)
	cache, err := local.New(5)
	require.NoError(t, err)
	refresher := NewCachedParamSetRefresher(pf, cache)
	refresher.PopulateCache()

	ctx := context.Background()
	// Check if items in level 1 have been populated.
	l1CacheKey, _ := paramSetKey(l1QueryValues, []string{"config"})
	assertCacheHit(t, ctx, cache, l1CacheKey, 2)

	// Level2 items are expected to not be populated since we only configured level 1.
	l2CacheKey1, _ := paramSetKey(l2Query1Values, []string{"config", "test"})
	assertCacheMiss(t, ctx, cache, l2CacheKey1)
	l2CacheKey2, _ := paramSetKey(l2Query2Values, []string{"config", "test"})
	assertCacheMiss(t, ctx, cache, l2CacheKey2)
}

func TestGetParamSetKey_Success_OnOneKey_OnRemoveDefault(t *testing.T) {
	pf := getPsRefresher(nil, nil, nil)
	cache := mockCache.NewCache(t)
	refresher := NewCachedParamSetRefresher(pf, cache)
	refresher.psRefresher.experiments = (config.Experiments{})
	refresher.psRefresher.qConfig.DefaultParamSelections = map[string][]string{"stat": {"value"}}
	refresher.psRefresher.qConfig.CacheConfig.Level1Key = "benchmark"
	refresher.psRefresher.qConfig.CacheConfig.Level2Key = "bot"
	q := url.Values{"benchmark": []string{"abc"}, "stat": []string{"value"}}
	key, err := refresher.getParamSetKey(q)
	assert.Nil(t, err)
	assert.Equal(t, "benchmark=[abc]", key)
}

func TestGetParamSetKey_Success_OnTwoKeys_OnRemoveDefault(t *testing.T) {
	pf := getPsRefresher(nil, nil, nil)
	cache := mockCache.NewCache(t)
	refresher := NewCachedParamSetRefresher(pf, cache)
	refresher.psRefresher.experiments = (config.Experiments{})
	refresher.psRefresher.qConfig.DefaultParamSelections = map[string][]string{"stat": {"value"}}
	refresher.psRefresher.qConfig.CacheConfig.Level1Key = "benchmark"
	refresher.psRefresher.qConfig.CacheConfig.Level2Key = "bot"
	q := url.Values{"benchmark": []string{"abc"}, "stat": []string{"value"}, "bot": []string{"def"}}
	key, err := refresher.getParamSetKey(q)
	assert.Nil(t, err)
	assert.Equal(t, "benchmark=[abc]&bot=[def]", key)
}

func TestGetParamSetKey_Fail(t *testing.T) {
	pf := getPsRefresher(nil, nil, nil)
	cache := mockCache.NewCache(t)
	refresher := NewCachedParamSetRefresher(pf, cache)
	refresher.psRefresher.experiments = (config.Experiments{})
	refresher.psRefresher.qConfig.DefaultParamSelections = map[string][]string{"stat": {"value"}}
	refresher.psRefresher.experiments.RemoveDefaultStatValue = true
	refresher.psRefresher.qConfig.CacheConfig.Level1Key = "benchmark"
	refresher.psRefresher.qConfig.CacheConfig.Level2Key = "bot"
	q := url.Values{"benchmark": []string{"abc"}, "stat": []string{"value"}}
	_, err := refresher.getParamSetKey(q)
	assert.NotNil(t, err, "Key bot not present in query values %v", q)
}

func TestGetParamSetKey_Success_OnOneKey(t *testing.T) {
	pf := getPsRefresher(nil, nil, nil)
	cache := mockCache.NewCache(t)
	refresher := NewCachedParamSetRefresher(pf, cache)
	refresher.psRefresher.experiments = (config.Experiments{})
	refresher.psRefresher.qConfig.DefaultParamSelections = map[string][]string{"stat": {"value"}}
	refresher.psRefresher.experiments.RemoveDefaultStatValue = true
	refresher.psRefresher.qConfig.CacheConfig.Level1Key = "benchmark"
	refresher.psRefresher.qConfig.CacheConfig.Level2Key = "bot"
	q := url.Values{"benchmark": []string{"abc"}}
	key, err := refresher.getParamSetKey(q)
	assert.Nil(t, err)
	assert.Equal(t, "benchmark=[abc]", key)
}

func TestGetParamSetKey_Success_OnTwoKeys(t *testing.T) {
	pf := getPsRefresher(nil, nil, nil)
	cache := mockCache.NewCache(t)
	refresher := NewCachedParamSetRefresher(pf, cache)
	refresher.psRefresher.experiments = (config.Experiments{})
	refresher.psRefresher.qConfig.DefaultParamSelections = map[string][]string{"stat": {"value"}}
	refresher.psRefresher.experiments.RemoveDefaultStatValue = true
	refresher.psRefresher.qConfig.CacheConfig.Level1Key = "benchmark"
	refresher.psRefresher.qConfig.CacheConfig.Level2Key = "bot"
	q := url.Values{"benchmark": []string{"abc"}, "bot": []string{"def"}}
	key, err := refresher.getParamSetKey(q)
	assert.Nil(t, err)
	assert.Equal(t, "benchmark=[abc]&bot=[def]", key)
}

func TestGetParamSetKey_Success_OnMoreThanTwoKeys(t *testing.T) {
	pf := getPsRefresher(nil, nil, nil)
	cache := mockCache.NewCache(t)
	refresher := NewCachedParamSetRefresher(pf, cache)
	refresher.psRefresher.experiments = (config.Experiments{})
	refresher.psRefresher.qConfig.DefaultParamSelections = map[string][]string{"stat": {"value"}}
	refresher.psRefresher.experiments.RemoveDefaultStatValue = true
	refresher.psRefresher.qConfig.CacheConfig.Level1Key = "benchmark"
	refresher.psRefresher.qConfig.CacheConfig.Level2Key = "bot"
	q := url.Values{"benchmark": []string{"abc"}, "bot": []string{"def"}, "stat": []string{"value"}}
	key, err := refresher.getParamSetKey(q)
	assert.Nil(t, err)
	assert.Equal(t, "", key)
}

func TestGetParamSetForQuery_MissingCountTriggersFallback(t *testing.T) {
	cache := mockCache.NewCache(t)

	qValues := url.Values{"config": []string{"8888"}}
	q, err := query.New(qValues)
	require.NoError(t, err)

	// Determine the cache key that will be requested
	cacheKey, err := paramSetKey(qValues, []string{"config"})
	require.NoError(t, err)

	// 1. Simulate Cache HIT for the ParamSet
	validParamSetJSON := `{"config":["8888"]}`
	cache.On("GetValue", mock.Anything, cacheKey).Return(validParamSetJSON, nil)

	// 2. Simulate Cache MISS for the Count
	cache.On("GetValue", mock.Anything, countKey(cacheKey)).Return("", nil)

	// 3. Set up the Database Fallback (DataFrameBuilder Mock)
	// If the fallback works correctly, it will hit this mock. We return a distinct
	// count (99) and a distinct paramset so we can assert the fallback was used.
	dfbMock := &dfb.DataFrameBuilder{}
	expectedFallbackPS := paramtools.ParamSet{"config": []string{"8888", "fallback_hit"}}
	dfbMock.On("PreflightQuery", mock.Anything, mock.Anything, mock.Anything).Return(
		int64(99), expectedFallbackPS, nil)

	cacheConfig := &config.QueryCacheConfig{
		Level1Key: "config",
		Enabled:   true,
	}

	// Initialize our refreshers
	pf := getPsRefresher(nil, cacheConfig, dfbMock)
	refresher := NewCachedParamSetRefresher(pf, cache)

	// 4. Execute the query
	ctx := context.Background()
	count, ps, err := refresher.GetParamSetForQuery(ctx, q, qValues)

	// 5. Assertions
	require.NoError(t, err)

	// We should get 99 (from the DB fallback), NOT 0 (from the bug)
	assert.Equal(t, int64(99), count, "Expected count to come from DB fallback, not default to 0")

	// We should get the fallback paramset, NOT the cached one
	assert.Equal(t, expectedFallbackPS, ps, "Expected paramset to come from DB fallback")

	// Ensure our mocks were actually called as expected
	cache.AssertExpectations(t)
	dfbMock.AssertExpectations(t)
}

func assertCacheHit(t *testing.T, ctx context.Context, cache *local.Cache, psCacheKey string, expectedCount int) {
	val, err := cache.GetValue(ctx, psCacheKey)
	assert.Nil(t, err)
	assert.NotNil(t, val, "Value expected in cache.")
	countStr, err := cache.GetValue(ctx, countKey(psCacheKey))
	assert.Nil(t, err)
	assert.NotNil(t, countStr, "Count expected in cache.")
	count, err := strconv.ParseInt(countStr, 10, 64)
	assert.Nil(t, err, "Int value expected, was %s", countStr)
	assert.Equal(t, int64(expectedCount), count)
}

func assertCacheMiss(t *testing.T, ctx context.Context, cache *local.Cache, psCacheKey string) {
	val, err := cache.GetValue(ctx, psCacheKey)
	assert.Nil(t, err)
	assert.Empty(t, val, "Expected key %s to be missing in cache", psCacheKey)

	countVal, err := cache.GetValue(ctx, countKey(psCacheKey))
	assert.Nil(t, err)
	assert.Empty(t, countVal, "Expected count key for %s to be missing in cache", psCacheKey)
}

func getPsRefresher(ps *paramtools.ReadOnlyParamSet, cacheConfig *config.QueryCacheConfig, dfbMock *dfb.DataFrameBuilder) *defaultParamSetRefresher {
	op := &mocks.OPSProvider{}
	tileNumber := types.TileNumber(100)
	op.On("GetLatestTile", testutils.AnyContext).Return(tileNumber, nil)

	if ps == nil {
		ps = &paramtools.ReadOnlyParamSet{
			"config": []string{"8888", "565"},
		}
	}

	op.On("GetParamSet", testutils.AnyContext, tileNumber).Return(*ps, nil)

	if dfbMock == nil {
		dfbMock = &dfb.DataFrameBuilder{}
		dfbMock.On("PreflightQuery", mock.Anything, mock.Anything, mock.Anything).Return(
			int64(10), paramtools.ParamSet{"config": []string{"8888"}}, nil)
	}

	if cacheConfig == nil {
		cacheConfig = &config.QueryCacheConfig{
			Level1Key:    "config",
			Level1Values: []string{"8888"},
			Enabled:      true,
		}
	}
	qConfig := config.QueryConfig{
		CacheConfig: *cacheConfig,
	}
	pf := NewDefaultParamSetRefresher(op, 1, dfbMock, qConfig, config.Experiments{})
	_ = pf.Start(time.Minute)
	return pf
}
