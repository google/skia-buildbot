// Package regressiontest has common utility funcs for testing the regression
// package.
package regressiontest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
)

// getTestVars returns vars needed by all the subtests below.
func getTestVars() (context.Context, *regression.Regressions, *cid.CommitDetail, *dataframe.FrameResponse, *clustering2.ClusterSummary) {
	ctx := context.Background()

	r := regression.New()
	c := &cid.CommitDetail{
		CommitID: cid.CommitID{
			Offset: 1,
		},
		Timestamp: 1479235651,
	}

	// Test Regressions.
	df := &dataframe.FrameResponse{}
	cl := &clustering2.ClusterSummary{}

	return ctx, r, c, df, cl
}

// Store_SetLowAndTriage tests that the implementation of the regression.Store
// interface operates correctly on the happy path.
func Store_SetLowAndTriage(t *testing.T, store regression.Store) {
	ctx, _, c, df, cl := getTestVars()

	now := time.Unix(c.Timestamp, 0)
	begin := now.Add(-time.Hour).Unix()
	end := now.Add(time.Hour).Unix()

	// Create a new regression.
	isNew, err := store.SetLow(ctx, c, "foo", df, cl)
	assert.True(t, isNew)
	assert.NoError(t, err)

	// Overwrite a regression.
	isNew, err = store.SetLow(ctx, c, "foo", df, cl)
	assert.False(t, isNew)
	assert.NoError(t, err)

	// Confirm new regression is present.
	ranges, err := store.Range(ctx, begin, end)
	assert.NoError(t, err)
	assert.Len(t, ranges, 1)

	count, err := store.CountUntriaged(ctx)
	assert.Equal(t, count, 1)

	// Triage existing regression.
	tr := regression.TriageStatus{
		Status:  regression.POSITIVE,
		Message: "bad",
	}
	err = store.TriageLow(ctx, c, "foo", tr)
	assert.NoError(t, err)

	// Confirm regression is triaged.
	ranges, err = store.Range(ctx, begin, end)
	assert.NoError(t, err)
	key := ""
	for key = range ranges {
		break
	}
	assert.Equal(t, regression.POSITIVE, ranges[key].ByAlertID["foo"].LowStatus.Status)

	count, err = store.CountUntriaged(ctx)
	assert.Equal(t, count, 0)

	ranges, err = store.Range(ctx, begin, end)
	assert.NoError(t, err)
	assert.Len(t, ranges, 1)
}

// Store_TriageNonExistentRegression tests that the implementation of the
// regression.Store interface fails as expected when triaging an unknown
// regression.
func Store_TriageNonExistentRegression(t *testing.T, store regression.Store) {
	ctx, _, c, _, _ := getTestVars()

	tr := regression.TriageStatus{
		Status:  regression.POSITIVE,
		Message: "bad",
	}
	// Try triaging a regression that doesn't exist.
	err := store.TriageHigh(ctx, c, "bar", tr)
	assert.Error(t, err)
}

// SubTestFunction is a func we will call to test one aspect of an
// implementation of regression.Store.
type SubTestFunction func(t *testing.T, store regression.Store)

// SubTests are all the subtests we have for regression.Store.
var SubTests = map[string]SubTestFunction{
	"Store_SetLowAndTriage":             Store_SetLowAndTriage,
	"Store_TriageNonExistentRegression": Store_TriageNonExistentRegression,
}
