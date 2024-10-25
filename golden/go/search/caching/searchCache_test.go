package caching

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockCache "go.skia.org/infra/go/cache/mock"
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
	searchCacheManager := New(cacheClient, db, []string{dks.RoundCorpus}, 5)
	err := searchCacheManager.RunCachePopulation(ctx)
	assert.Nil(t, err)
	cacheClient.AssertNumberOfCalls(t, "SetValue", 1)
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
