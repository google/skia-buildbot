package providers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mock_cache "go.skia.org/infra/go/cache/mock"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/search/common"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/web/frontend"
)

// useKitchenSinkData returns a db that has the kitchen sink data loaded and enough time is passed
// for AS OF SYSTEM TIME queries to work.
func useKitchenSinkData(ctx context.Context, t *testing.T) *pgxpool.Pool {
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, dks.Build()))
	return db
}

func populateMockCache(t *testing.T, cache *mock_cache.Cache, commitIdMap map[schema.CommitID]int) {
	for commitId := range commitIdMap {
		mockCommit := frontend.Commit{
			ID:      string(commitId),
			Subject: "Commit: " + string(commitId),
		}
		commitJsonBytes, err := json.Marshal(mockCommit)
		require.NoError(t, err)

		cache.On("GetValue", testutils.AnyContext, commitKey(commitId)).Return(string(commitJsonBytes), nil)
	}
}

func Test_GetCommits_CacheHit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := useKitchenSinkData(ctx, t)

	cache := mock_cache.NewCache(t)

	// Add commit info to the context.
	ctx, err := common.AddCommitsData(ctx, db, 10)
	require.NoError(t, err)

	commitIDs := common.GetCommitToIdxMap(ctx)
	populateMockCache(t, cache, commitIDs)

	commitsProvider := NewCommitsProvider(db, cache, 10)

	commits, err := commitsProvider.GetCommits(ctx)
	require.NoError(t, err)

	assert.Equal(t, 10, len(commits))
	// Assert that the cache was called
	cache.AssertNumberOfCalls(t, "GetValue", 10)
}

func Test_Commits_CacheMiss(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := useKitchenSinkData(ctx, t)

	cache := mock_cache.NewCache(t)

	// Return a cache miss for all cases.
	cache.On("GetValue", testutils.AnyContext, mock.Anything).Return("", nil)

	// Add commit info to the context.
	ctx, err := common.AddCommitsData(ctx, db, 10)
	require.NoError(t, err)
	commitsProvider := NewCommitsProvider(db, cache, 10)

	// Cache is not populated, but we should still get commits from the database.
	commits, err := commitsProvider.GetCommits(ctx)
	require.NoError(t, err)

	assert.Equal(t, 10, len(commits))
	// Assert that the cache was called
	cache.AssertNumberOfCalls(t, "GetValue", 10)
}

func Test_Commits_CacheError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := useKitchenSinkData(ctx, t)

	cache := mock_cache.NewCache(t)

	// Return a cache miss for all cases.
	cache.On("GetValue", testutils.AnyContext, mock.Anything).Return("", skerr.Fmt("Unexpected error occurred"))

	// Add commit info to the context.
	ctx, err := common.AddCommitsData(ctx, db, 10)
	require.NoError(t, err)
	commitsProvider := NewCommitsProvider(db, cache, 10)

	// Cache GET is going to return an error, but we should still get commits from the database.
	commits, err := commitsProvider.GetCommits(ctx)
	require.NoError(t, err)

	assert.Equal(t, 10, len(commits))
	// Assert that the cache was called
	cache.AssertNumberOfCalls(t, "GetValue", 10)
}
