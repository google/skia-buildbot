package sqlanomalygroupstore

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/anomalygroup"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func setUp(t *testing.T) (anomalygroup.Store, pool.Pool) {
	db := sqltest.NewCockroachDBForTests(t, "anomalygroups")
	store, err := New(db)
	require.NoError(t, err)
	return store, db
}

func TestCreate(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "report")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	count_cmd := "SELECT COUNT(*) FROM AnomalyGroups"
	count := 0
	err = db.QueryRow(ctx, count_cmd).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestCreate_EmptyStrings(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "", "rev-abc", "domain-a", "benchmark-a", 100, 200, "report")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty strings")
}

func TestCreate_InvalidCommits(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 300, 200, "report")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "smaller than the start")
}

func TestCreate_NegativeCommits(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", -100, 200, "report")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "negative commit")
}

func TestLoadByID(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "report")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	group, err2 := store.LoadById(ctx, new_group_id)
	require.NoError(t, err2)
	assert.Equal(t, "report", group.GroupAction)
}

func TestLoadByID_BadID(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "report")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	bad_id := new_group_id[:len(new_group_id)-1]
	_, err = store.LoadById(ctx, bad_id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UUID")
}

func TestLoadByID_NoRow(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "report")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	_, err = store.LoadById(ctx, uuid.NewString())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no rows")
}
