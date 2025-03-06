package sqlreversekeymapstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func setUp(t *testing.T) (chromeperf.ReverseKeyMapStore, pool.Pool) {
	db := sqltest.NewSpannerDBForTests(t, "reversekeymap")
	store := New(db, config.SpannerDataStoreType)
	return store, db
}

func TestCreate(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()

	v, err := store.Create(ctx, "new_string", "key", "old_string")
	require.NoError(t, err)
	assert.Equal(t, "old_string", v)

	count_cmd := "SELECT COUNT(*) FROM ReverseKeyMap"
	count := 0
	err = db.QueryRow(ctx, count_cmd).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestCreate_MissingValue(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "new_string", "", "old_string")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Value, key and old value are all required")
}

func TestCreate_Collision(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	v, err := store.Create(ctx, "new_string", "key", "old_string")
	require.NoError(t, err)
	assert.Equal(t, "old_string", v)
	v2, err := store.Create(ctx, "new_string", "key", "old_string_2")
	require.NoError(t, err)
	assert.Equal(t, "", v2)
}

func TestGet(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "new_string", "key", "old_string")
	require.NoError(t, err)

	v, err := store.Get(ctx, "new_string", "key")
	require.NoError(t, err)
	assert.Equal(t, "old_string", v)
}

func TestGet_EmptyKey_Error(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "new_string", "key", "old_string")
	require.NoError(t, err)

	v, err := store.Get(ctx, "new_string", "")
	assert.Equal(t, "", v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No value or key is given")
}

func TestGet_NotFound(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "new_string", "key", "old_string")
	require.NoError(t, err)

	v, err := store.Get(ctx, "new_string_x", "key")
	require.NoError(t, err)
	assert.Equal(t, "", v)
}
