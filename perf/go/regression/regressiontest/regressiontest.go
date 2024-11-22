// Package regressiontest has common utility funcs for testing the regression
// package.
package regressiontest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

var (
	// timestamps is a list of timestamps used for each commit in the tests
	// below.
	timestamps = []int64{
		1580000000,
		1580000000 + 100,
		1580000000 + 200,
		1580000000 + 300}
)

// getTestVars returns vars needed by all the subtests below.
func getTestVars() (context.Context, types.CommitNumber) {
	ctx := context.Background()
	c := types.CommitNumber(1)

	return ctx, c
}

// SetLowAndTriage tests that the implementation of the regression.Store
// interface operates correctly on the happy path.
func SetLowAndTriage(t *testing.T, store regression.Store) {
	ctx, c := getTestVars()

	// Args to Set* that are then serialized to the datastore.
	df := &frame.FrameResponse{
		Msg: "Looks like a regression",
	}
	cl := &clustering2.ClusterSummary{
		Num: 50,
	}

	// TODO(jcgregorio) Break up into finer grained tests and add more tests.

	// Create a new regression.
	isNew, _, err := store.SetLow(ctx, c, "1", df, cl)
	assert.True(t, isNew)
	require.NoError(t, err)

	// Overwrite a regression, which is allowed, and that it changes the
	// returned 'isNew' value.
	isNew, _, err = store.SetLow(ctx, c, "1", df, cl)
	assert.False(t, isNew)
	require.NoError(t, err)

	// Confirm new regression is present.
	ranges, err := store.Range(ctx, 1, 3)
	require.NoError(t, err)
	require.Len(t, ranges, 1)

	// Triage existing regression.
	tr := regression.TriageStatus{
		Status:  regression.Positive,
		Message: "bad",
	}
	err = store.TriageLow(ctx, c, "1", tr)
	require.NoError(t, err)

	// Confirm regression is triaged.
	ranges, err = store.Range(ctx, 1, 3)
	require.NoError(t, err)
	assert.Len(t, ranges, 1)
	key := types.BadCommitNumber
	for key = range ranges {
		break
	}
	assert.Equal(t, regression.Positive, ranges[key].ByAlertID["1"].LowStatus.Status)

	ranges, err = store.Range(ctx, 1, 3)
	require.NoError(t, err)
	assert.Len(t, ranges, 1)
}

// Range_Exact tests that Range returns values when begin=end.
func Range_Exact(t *testing.T, store regression.Store) {
	ctx, c := getTestVars()

	// Args to Set* that are then serialized to the datastore.
	df := &frame.FrameResponse{
		Msg: "Looks like a regression",
	}
	cl := &clustering2.ClusterSummary{
		Num: 50,
	}

	// Create a new regression.
	isNew, _, err := store.SetLow(ctx, c, "1", df, cl)
	assert.True(t, isNew)
	require.NoError(t, err)

	// Confirm new regression is present.
	ranges, err := store.Range(ctx, 1, 1)
	require.NoError(t, err)
	require.Len(t, ranges, 1)
}

// TriageNonExistentRegression tests that the implementation of the
// regression.Store interface fails as expected when triaging an unknown
// regression.
func TriageNonExistentRegression(t *testing.T, store regression.Store) {
	ctx, c := getTestVars()

	tr := regression.TriageStatus{
		Status:  regression.Positive,
		Message: "bad",
	}
	// Try triaging a regression that doesn't exist.
	err := store.TriageHigh(ctx, c, "12", tr)
	assert.Error(t, err)
}

// Write tests that the implementation of the regression.Store interface can
// bulk write Regressions.
func Write(t *testing.T, store regression.Store) {
	ctx := context.Background()

	reg := &regression.AllRegressionsForCommit{
		ByAlertID: map[string]*regression.Regression{
			"1": regression.NewRegression(),
		},
	}
	err := store.Write(ctx, map[types.CommitNumber]*regression.AllRegressionsForCommit{2: reg})
	require.NoError(t, err)
	ranges, err := store.Range(ctx, 1, 3)
	assert.NoError(t, err)
	assert.Len(t, ranges, 1)
	assert.Equal(t, reg, ranges[2])
}

// DeleteByCommit tests that the implementation of the regression.Store interface can
// delete an inserted regression
func DeleteByCommit(t *testing.T, store regression.Store) {
	ctx := context.Background()

	reg := &regression.AllRegressionsForCommit{
		ByAlertID: map[string]*regression.Regression{
			"fake-alert-id": regression.NewRegression(),
		},
	}
	err := store.Write(ctx, map[types.CommitNumber]*regression.AllRegressionsForCommit{2: reg})
	require.NoError(t, err, "error writing regression")

	regressions, err := store.Range(ctx, 1, 2)
	require.NoError(t, err, "error fetching regression")
	require.Len(t, regressions, 1)

	err = store.DeleteByCommit(ctx, types.CommitNumber(2), nil)
	require.NoError(t, err)
	regressions, err = store.Range(ctx, 1, 3)
	require.NoError(t, err, "error fetching regression")
	require.Len(t, regressions, 0, "regression was not deleted")
}

// GetOldestCommit tests that the implementation of the regression.Store interface can
// get the lowest commit number
func GetOldestCommit(t *testing.T, store regression.Store) {
	ctx := context.Background()

	reg := &regression.AllRegressionsForCommit{
		ByAlertID: map[string]*regression.Regression{
			"fake-alert-id": regression.NewRegression(),
		},
	}
	for i := 0; i < 5; i++ {
		err := store.Write(ctx, map[types.CommitNumber]*regression.AllRegressionsForCommit{
			types.CommitNumber(i): reg,
		})
		require.NoError(t, err, "error writing regression %d", i)
	}

	commitNumber, err := store.GetOldestCommit(ctx)
	require.NoError(t, err, "error fetching regression")
	require.Equal(t, types.CommitNumber(0), *commitNumber)

	err = store.DeleteByCommit(ctx, types.CommitNumber(0), nil)
	require.NoError(t, err)

	commitNumber, err = store.GetOldestCommit(ctx)
	require.NoError(t, err, "error fetching regression")
	require.Equal(t, types.CommitNumber(1), *commitNumber)
}

// SubTestFunction is a func we will call to test one aspect of an
// implementation of regression.Store.
type SubTestFunction func(t *testing.T, store regression.Store)

// SubTests are all the subtests we have for regression.Store.
var SubTests = map[string]SubTestFunction{
	"SetLowAndTriage":             SetLowAndTriage,
	"Range_Exact":                 Range_Exact,
	"TriageNonExistentRegression": TriageNonExistentRegression,
	"TestWrite":                   Write,
	"TestDeleteByCommit":          DeleteByCommit,
	"TestGetOldestCommit":         GetOldestCommit,
}
