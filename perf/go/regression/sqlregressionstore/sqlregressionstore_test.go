package sqlregressionstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/regressiontest"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func TestSQLRegressionStore_SQLite(t *testing.T) {
	unittest.LargeTest(t)

	for name, subTest := range regressiontest.SubTests {
		t.Run(name, func(t *testing.T) {
			db, cleanup := sqltest.NewSQLite3DBForTests(t)
			defer cleanup()

			store, err := New(db, perfsql.SQLiteDialect)
			require.NoError(t, err)
			subTest(t, store)
		})
	}

	// SQLRegressionStore specific tests.
	for name, subTest := range SubTests {
		t.Run(name, func(t *testing.T) {
			db, cleanup := sqltest.NewSQLite3DBForTests(t)
			defer cleanup()

			store, err := New(db, perfsql.SQLiteDialect)
			require.NoError(t, err)
			subTest(t, store)
		})
	}
}

func TestSQLRegressionStore_CockroachDB(t *testing.T) {
	unittest.LargeTest(t)

	for name, subTest := range regressiontest.SubTests {
		t.Run(name, func(t *testing.T) {
			db, cleanup := sqltest.NewCockroachDBForTests(t, "alertstore")
			// If this test timeouts then comment out the cleanup(), as it may hide the
			// actual errors.
			defer cleanup()

			store, err := New(db, perfsql.CockroachDBDialect)
			require.NoError(t, err)
			subTest(t, store)
		})
	}

	// SQLRegressionStore specific tests.
	for name, subTest := range SubTests {
		t.Run(name, func(t *testing.T) {
			db, cleanup := sqltest.NewSQLite3DBForTests(t)
			defer cleanup()

			store, err := New(db, perfsql.SQLiteDialect)
			require.NoError(t, err)
			subTest(t, store)
		})
	}
}

func ReadWrite(t *testing.T, store *SQLRegressionStore) {
	ctx := context.Background()
	r := regression.NewRegression()
	// Fill with data to ensure it round-trips.
	r.HighStatus.Status = regression.NEGATIVE
	r.HighStatus.Message = "not good"
	err := store.write(ctx, 1, "1", r)
	require.NoError(t, err)
	r2, err := store.read(ctx, 1, "1")
	require.NoError(t, err)
	assert.Equal(t, r, r2)
}

func Overwrite(t *testing.T, store *SQLRegressionStore) {
	ctx := context.Background()
	r := regression.NewRegression()
	// Fill with data to ensure it round-trips.
	r.HighStatus.Status = regression.NEGATIVE
	r.HighStatus.Message = "not good"
	err := store.write(ctx, 1, "1", r)
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

func ErrorOnReadMissing(t *testing.T, store *SQLRegressionStore) {
	ctx := context.Background()
	_, err := store.read(ctx, 1, "1")
	assert.Error(t, err)
}

func ReadModifyWrite(t *testing.T, store *SQLRegressionStore) {
	ctx := context.Background()
	r := regression.NewRegression()
	// Fill with data to ensure it round-trips.
	r.HighStatus.Status = regression.NEGATIVE
	r.HighStatus.Message = "not good"
	err := store.write(ctx, 1, "1", r)
	require.NoError(t, err)
	err = store.readModifyWrite(ctx, 1, "1", func(r *regression.Regression) {
		r.HighStatus.Status = regression.POSITIVE
		r.HighStatus.Message = "my bad"
	})
	require.NoError(t, err)

	r2, err := store.read(ctx, 1, "1")
	require.NoError(t, err)
	assert.NotEqual(t, r, r2)
	assert.Equal(t, "my bad", r2.HighStatus.Message)
}

func ReadModifyWrite_StartWithEmpty(t *testing.T, store *SQLRegressionStore) {
	ctx := context.Background()
	err := store.readModifyWrite(ctx, 1, "1", func(r *regression.Regression) {
		r.HighStatus.Status = regression.POSITIVE
		r.HighStatus.Message = "my bad"
	})
	require.NoError(t, err)

	r2, err := store.read(ctx, 1, "1")
	require.NoError(t, err)
	assert.Equal(t, "my bad", r2.HighStatus.Message)
	assert.Equal(t, regression.POSITIVE, r2.HighStatus.Status)
}

// SubTestFunction is a func we will call to test one aspect of an
// implementation of regression.Store.
type SubTestFunction func(t *testing.T, store *SQLRegressionStore)

// SubTests are all the subtests we have for regression.Store.
var SubTests = map[string]SubTestFunction{
	"ReadWrite":                      ReadWrite,
	"Overwrite":                      Overwrite,
	"ErrorOnReadMissing":             ErrorOnReadMissing,
	"ReadModifyWrite":                ReadModifyWrite,
	"ReadModifyWrite_StartWithEmpty": ReadModifyWrite_StartWithEmpty,
}
