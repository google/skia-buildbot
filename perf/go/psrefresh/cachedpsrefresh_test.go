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
	pf := NewDefaultParamSetRefresher(op, 1, dfbMock, qConfig)
	_ = pf.Start(time.Minute)
	return pf
}
