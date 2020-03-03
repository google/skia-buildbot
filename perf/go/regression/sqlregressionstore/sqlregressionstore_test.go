package sqlregressionstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/regression"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func TestReadWrite_SQLite(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()

	db, cleanup := sqltest.NewSQLite3DBForTests(t)
	defer cleanup()

	store, err := New(db, perfsql.SQLiteDialect)
	require.NoError(t, err)

	r := regression.NewRegression()
	// Fill with data to ensure it round-trips.
	r.HighStatus.Status = regression.NEGATIVE
	r.HighStatus.Message = "not good"
	err = store.write(ctx, 1, "1", r)
	require.NoError(t, err)
	r2, err := store.read(ctx, 1, "1")
	require.NoError(t, err)
	assert.Equal(t, r, r2)
}

func TestReadWrite_SQLite_Overwrite(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()

	db, cleanup := sqltest.NewSQLite3DBForTests(t)
	defer cleanup()

	store, err := New(db, perfsql.SQLiteDialect)
	require.NoError(t, err)

	r := regression.NewRegression()
	// Fill with data to ensure it round-trips.
	r.HighStatus.Status = regression.NEGATIVE
	r.HighStatus.Message = "not good"
	err = store.write(ctx, 1, "1", r)
	require.NoError(t, err)
	r2, err := store.read(ctx, 1, "1")
	require.NoError(t, err)
	assert.Equal(t, r, r2)

	r.HighStatus.Status = regression.POSITIVE
	r.HighStatus.Message = "my bad"
	err = store.write(ctx, 1, "1", r)
	require.NoError(t, err)
	r2, err = store.read(ctx, 1, "1")
	require.NoError(t, err)
	assert.Equal(t, r, r2)
}

func TestRead_SQLite_ErrorOnMissing(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()

	db, cleanup := sqltest.NewSQLite3DBForTests(t)
	defer cleanup()

	store, err := New(db, perfsql.SQLiteDialect)
	require.NoError(t, err)

	_, err = store.read(ctx, 1, "1")
	assert.Error(t, err)
}
