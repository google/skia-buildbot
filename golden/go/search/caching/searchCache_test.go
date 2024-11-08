package caching

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockCache "go.skia.org/infra/go/cache/mock"
	"go.skia.org/infra/go/deepequal/assertdeep"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
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
	cacheClient.On("SetValue", ctx, ByBlameKey(dks.RoundCorpus), mock.AnythingOfType("string")).Return(nil)
	cacheClient.On("SetValue", ctx, UnignoredKey(dks.RoundCorpus), mock.AnythingOfType("string")).Return(nil)
	searchCacheManager := New(cacheClient, db, []string{dks.RoundCorpus}, 5)
	err := searchCacheManager.RunCachePopulation(ctx)
	assert.Nil(t, err)
	cacheClient.AssertNumberOfCalls(t, "SetValue", 2)
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

func TestReadFromCache_Unignored_CacheHit_Success(t *testing.T) {
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
	validateCacheHit(t, cacheResults, corpus, UnignoredKey(corpus), Unignored_Corpus)
}

func TestReadFromCache_Unignored_CacheMiss_Success(t *testing.T) {
	corpus := dks.RoundCorpus
	validateCacheMiss(t, corpus, UnignoredKey(corpus), Unignored_Corpus)
}

func validateCacheHit(t *testing.T, cacheData []SearchCacheData, corpus string, cacheKey string, searchCacheType SearchCacheType) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := useKitchenSinkData(ctx, t)
	cacheClient := mockCache.NewCache(t)

	cacheClient.On("GetValue", ctx, cacheKey).Return(toJSON(cacheData))

	searchCacheManager := New(cacheClient, db, []string{corpus}, 5)
	var data []SearchCacheData
	var err error
	switch searchCacheType {
	case ByBlame_Corpus:
		data, err = searchCacheManager.GetByBlameData(ctx, corpus)
	case Unignored_Corpus:
		data, err = searchCacheManager.GetUnignoredTracesData(ctx, corpus)
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
		data, err = searchCacheManager.GetByBlameData(ctx, corpus)
	case Unignored_Corpus:
		data, err = searchCacheManager.GetUnignoredTracesData(ctx, corpus)
	default:
		assert.Fail(t, "Invalid search cache type: %v", searchCacheType)
	}
	assert.Nil(t, err)
	assert.NotNil(t, data)
	// Even when there is a cache miss, we should have data since it will fall back to db query.
	assert.True(t, len(data) > 0)
	cacheClient.AssertNumberOfCalls(t, "GetValue", 1)
}
