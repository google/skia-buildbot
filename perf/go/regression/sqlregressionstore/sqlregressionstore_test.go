package sqlregressionstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/regressiontest"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func TestSQLRegressionStore_CockroachDB(t *testing.T) {
	unittest.LargeTest(t)

	// Common regressiontest tests.
	for name, subTest := range regressiontest.SubTests {
		t.Run(name, func(t *testing.T) {
			db, cleanup := sqltest.NewCockroachDBForTests(t, "regstore")
			// If this test times out then comment out the cleanup(), as it may hide the
			// actual errors.
			defer cleanup()

			store, err := New(db)
			require.NoError(t, err)
			subTest(t, store)
		})
	}

	// SQLRegressionStore specific tests.
	for name, subTest := range subTests {
		t.Run(name, func(t *testing.T) {
			db, cleanup := sqltest.NewCockroachDBForTests(t, "regstore")
			// If this test times out then comment out the cleanup(), as it may hide the
			// actual errors.
			defer cleanup()

			store, err := New(db)
			require.NoError(t, err)
			subTest(t, store)
		})
	}
}

const (
	expectedCommitNumber = 2
	expectedAlertID      = "1"
)

// writeRead tests *SQLRegressionStore Read and Write methods.
func writeRead(t *testing.T, store *SQLRegressionStore) {
	ctx := context.Background()
	r := regression.NewRegression()
	// Fill with data to ensure it round-trips.
	r.HighStatus.Status = regression.Negative
	r.HighStatus.Message = "not good"
	err := store.write(ctx, expectedCommitNumber, expectedAlertID, r)
	require.NoError(t, err)
	r2, err := store.read(ctx, expectedCommitNumber, expectedAlertID)
	require.NoError(t, err)
	assert.Equal(t, r, r2)
}

func writeRead_OverwriteValueGetsUpdated(t *testing.T, store *SQLRegressionStore) {
	ctx := context.Background()
	r := regression.NewRegression()
	// Fill with data to ensure it round-trips.
	r.HighStatus.Status = regression.Negative
	r.HighStatus.Message = "not good"
	err := store.write(ctx, expectedCommitNumber, expectedAlertID, r)
	require.NoError(t, err)
	r2, err := store.read(ctx, expectedCommitNumber, expectedAlertID)
	require.NoError(t, err)
	assert.Equal(t, r, r2)

	// Now overwrite that value and confirm it changes.
	r.HighStatus.Status = regression.Positive
	r.HighStatus.Message = "my bad"
	err = store.write(ctx, expectedCommitNumber, expectedAlertID, r)
	require.NoError(t, err)
	r2, err = store.read(ctx, expectedCommitNumber, expectedAlertID)
	require.NoError(t, err)
	assert.Equal(t, r, r2)
}

func errorOnReadMissing(t *testing.T, store *SQLRegressionStore) {
	ctx := context.Background()
	_, err := store.read(ctx, expectedCommitNumber, expectedAlertID)
	assert.Error(t, err)
}

func readModifyWrite(t *testing.T, store *SQLRegressionStore) {
	ctx := context.Background()
	r := regression.NewRegression()
	// Fill with data to ensure it round-trips.
	r.HighStatus.Status = regression.Negative
	r.HighStatus.Message = "not good"
	err := store.write(ctx, expectedCommitNumber, expectedAlertID, r)
	require.NoError(t, err)
	err = store.readModifyWrite(ctx, expectedCommitNumber, expectedAlertID, false, func(r *regression.Regression) {
		r.HighStatus.Status = regression.Positive
		r.HighStatus.Message = "my bad"
	})
	require.NoError(t, err)

	r2, err := store.read(ctx, expectedCommitNumber, expectedAlertID)
	require.NoError(t, err)
	assert.NotEqual(t, r, r2)
	assert.Equal(t, "my bad", r2.HighStatus.Message)
}

func readModifyWrite_StartWithEmpty(t *testing.T, store *SQLRegressionStore) {
	ctx := context.Background()
	err := store.readModifyWrite(ctx, expectedCommitNumber, expectedAlertID, false /* mustExist */, func(r *regression.Regression) {
		r.HighStatus.Status = regression.Positive
		r.HighStatus.Message = "my bad"
	})
	require.NoError(t, err)

	r2, err := store.read(ctx, expectedCommitNumber, expectedAlertID)
	require.NoError(t, err)
	assert.Equal(t, "my bad", r2.HighStatus.Message)
	assert.Equal(t, regression.Positive, r2.HighStatus.Status)
}

func readModifyWrite_StartWithEmptyAndFailOnMustExist(t *testing.T, store *SQLRegressionStore) {
	ctx := context.Background()
	err := store.readModifyWrite(ctx, expectedCommitNumber, expectedAlertID, true /* mustExist */, func(r *regression.Regression) {
		r.HighStatus.Status = regression.Positive
		r.HighStatus.Message = "my bad"
	})
	require.Error(t, err) // This means we passed through the tx rollback logic.
}

// subTestFunction is a func we will call to test one aspect of an
// implementation of *SQLRegressionStore.
type subTestFunction func(t *testing.T, store *SQLRegressionStore)

// subTests are all the subtests we have for *SQLRegressionStore.
var subTests = map[string]subTestFunction{
	"writeRead":                                        writeRead,
	"writeRead_OverwriteValueGetsUpdated":              writeRead_OverwriteValueGetsUpdated,
	"errorOnReadMissing":                               errorOnReadMissing,
	"readModifyWrite":                                  readModifyWrite,
	"readModifyWrite_StartWithEmpty":                   readModifyWrite_StartWithEmpty,
	"readModifyWrite_StartWithEmptyAndFailOnMustExist": readModifyWrite_StartWithEmptyAndFailOnMustExist,
}
