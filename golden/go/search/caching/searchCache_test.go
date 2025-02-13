package caching

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/cache/local"
	mockCache "go.skia.org/infra/go/cache/mock"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/search/common"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

// useKitchenSinkData returns a db that has the kitchen sink data loaded .
func useKitchenSinkData(ctx context.Context, t *testing.T) *pgxpool.Pool {
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	return db
}

func TestPopulateCache_WithData(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := useKitchenSinkData(ctx, t)
	cacheClient := mockCache.NewCache(t)
	cacheClient.On("SetValue", testutils.AnyContext, ByBlameKey(dks.RoundCorpus), mock.AnythingOfType("string")).Return(nil)
	cacheClient.On("SetValue", testutils.AnyContext, MatchingUntriagedTracesKey(dks.RoundCorpus), mock.AnythingOfType("string")).Return(nil)
	cacheClient.On("SetValue", testutils.AnyContext, MatchingPositiveTracesKey(dks.RoundCorpus), mock.AnythingOfType("string")).Return(nil)
	// The negative and ignored traces do not have any values in this data set, so no SetValue calls expected for those.
	searchCacheManager := New(cacheClient, db, []string{dks.RoundCorpus}, 5)
	err := searchCacheManager.RunCachePopulation(ctx)
	assert.Nil(t, err)
	cacheClient.AssertNumberOfCalls(t, "SetValue", 3)
}

func TestPopulateCache_NoData(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := useKitchenSinkData(ctx, t)
	cacheClient := mockCache.NewCache(t)
	// Provide a random corpus
	searchCacheManager := New(cacheClient, db, []string{"Random"}, 5)
	err := searchCacheManager.RunCachePopulation(ctx)
	assert.Nil(t, err)
}

func TestReadFromCache_ByBlame_CacheHit_Success(t *testing.T) {
	corpus := dks.RoundCorpus
	cacheResults := []SearchCacheData{
		{
			TraceID:    []byte("trace1"),
			GroupingID: []byte("group1"),
			Digest:     []byte("d1"),
		},
		{
			TraceID:    []byte("trace2"),
			GroupingID: []byte("group2"),
			Digest:     []byte("d2"),
		},
	}
	validateCacheHit(t, cacheResults, corpus, ByBlameKey(corpus), ByBlame_Corpus)
}

func TestReadFromCache_ByBlame_CacheMiss_Success(t *testing.T) {
	corpus := dks.RoundCorpus
	validateCacheMiss(t, corpus, ByBlameKey(corpus), ByBlame_Corpus)
}

func TestDigestsForGrouping_Success(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := useKitchenSinkData(ctx, t)
	cacheClient, err := local.New(10)
	require.NoError(t, err)
	searchCacheManager := New(cacheClient, db, []string{dks.RoundCorpus}, 5)
	groupingID := schema.GroupingID([]byte("testGroupingID"))
	paramSet := map[string][]string{
		"corpus": {dks.RoundCorpus},
		"key1":   {"value1"},
	}
	traceKeys := paramtools.NewParamSet()
	traceKeys.AddParamSet(paramSet)
	digestBytes := []schema.DigestBytes{
		schema.DigestBytes([]byte("digestBytes")),
	}
	err = searchCacheManager.SetDigestsForGrouping(ctx, groupingID, traceKeys, digestBytes)
	require.NoError(t, err)

	digestBytesFromCache, err := searchCacheManager.GetDigestsForGrouping(ctx, groupingID, traceKeys)
	require.NoError(t, err)
	assert.Equal(t, digestBytes, digestBytesFromCache)
}

func TestDigestsForGrouping_CacheMiss(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := useKitchenSinkData(ctx, t)
	cacheClient, err := local.New(10)
	require.NoError(t, err)
	searchCacheManager := New(cacheClient, db, []string{dks.RoundCorpus}, 5)
	groupingID := schema.GroupingID([]byte("testGroupingID"))
	paramSet := map[string][]string{
		"corpus": {dks.RoundCorpus},
		"key1":   {"value1"},
	}
	traceKeys := paramtools.NewParamSet()
	traceKeys.AddParamSet(paramSet)

	digestBytesFromCache, err := searchCacheManager.GetDigestsForGrouping(ctx, groupingID, traceKeys)
	require.NoError(t, err)
	assert.Nil(t, digestBytesFromCache)
}

func validateCacheHit(t *testing.T, cacheData []SearchCacheData, corpus string, cacheKey string, searchCacheType SearchCacheType) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := useKitchenSinkData(ctx, t)
	cacheClient := mockCache.NewCache(t)

	cacheClient.On("GetValue", ctx, cacheKey).Return(common.ToJSON(cacheData))

	searchCacheManager := New(cacheClient, db, []string{corpus}, 5)
	var data []SearchCacheData
	var err error
	switch searchCacheType {
	case ByBlame_Corpus:
		data, err = searchCacheManager.GetByBlameData(ctx, "0", corpus)
	default:
		assert.Fail(t, "Invalid search cache type: %v", searchCacheType)
	}

	assert.Nil(t, err)
	assert.NotNil(t, data)
	assertdeep.Equal(t, cacheData, data)
	cacheClient.AssertNumberOfCalls(t, "GetValue", 1)
}

func validateCacheMiss(t *testing.T, corpus string, cacheKey string, searchCacheType SearchCacheType) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := useKitchenSinkData(ctx, t)
	cacheClient := mockCache.NewCache(t)
	cacheClient.On("GetValue", ctx, cacheKey).Return("", nil)

	searchCacheManager := New(cacheClient, db, []string{corpus}, 5)
	var data []SearchCacheData
	var err error
	switch searchCacheType {
	case ByBlame_Corpus:
		data, err = searchCacheManager.GetByBlameData(ctx, "0", corpus)
	default:
		assert.Fail(t, "Invalid search cache type: %v", searchCacheType)
	}
	assert.Nil(t, err)
	assert.NotNil(t, data)
	// Even when there is a cache miss, we should have data since it will fall back to db query.
	assert.True(t, len(data) > 0)
	cacheClient.AssertNumberOfCalls(t, "GetValue", 1)
}
