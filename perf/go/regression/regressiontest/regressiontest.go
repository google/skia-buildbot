// Package regressiontest has common utility funcs for testing the regression
// package.
package regressiontest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/types"
)

// getTestVars returns vars needed by all the subtests below.
func getTestVars() (context.Context, *cid.CommitDetail) {
	ctx := context.Background()
	c := &cid.CommitDetail{
		CommitID: cid.CommitID{
			Offset: 1,
		},
		Timestamp: 1479235651,
	}

	return ctx, c
}

// SetLowAndTriage tests that the implementation of the regression.Store
// interface operates correctly on the happy path.
func SetLowAndTriage(t *testing.T, store regression.Store) {
	ctx, c := getTestVars()

	// Args to Set* that are then serialized to the datastore.
	df := &dataframe.FrameResponse{
		Msg: "Looks like a regression",
	}
	cl := &clustering2.ClusterSummary{
		Num: 50,
	}

	// TODO(jcgregorio) Break up into finer grained tests and add more tests.
	now := time.Unix(c.Timestamp, 0)
	begin := now.Add(-time.Hour).Unix()
	end := now.Add(time.Hour).Unix()

	// Create a new regression.
	isNew, err := store.SetLow(ctx, c, "foo", df, cl)
	assert.True(t, isNew)
	require.NoError(t, err)

	// Overwrite a regression, which is allowed, and that it changes the
	// returned 'isNew' value.
	isNew, err = store.SetLow(ctx, c, "foo", df, cl)
	assert.False(t, isNew)
	require.NoError(t, err)

	// Confirm new regression is present.
	ranges, err := store.Range(ctx, begin, end)
	require.NoError(t, err)
	assert.Len(t, ranges, 1)
	b, err := ranges[types.CommitNumber(1)].JSON()
	require.NoError(t, err)
	assert.Equal(t, "{\"by_query\":{\"foo\":{\"low\":{\"centroid\":null,\"shortcut\":\"\",\"param_summaries2\":null,\"step_fit\":null,\"step_point\":null,\"num\":50},\"high\":null,\"frame\":{\"dataframe\":null,\"skps\":null,\"msg\":\"Looks like a regression\"},\"low_status\":{\"status\":\"untriaged\",\"message\":\"\"},\"high_status\":{\"status\":\"\",\"message\":\"\"}}}}", string(b))

	// Triage existing regression.
	tr := regression.TriageStatus{
		Status:  regression.POSITIVE,
		Message: "bad",
	}
	err = store.TriageLow(ctx, c, "foo", tr)
	require.NoError(t, err)

	// Confirm regression is triaged.
	ranges, err = store.Range(ctx, begin, end)
	require.NoError(t, err)
	assert.Len(t, ranges, 1)
	key := types.BadCommitNumber
	for key = range ranges {
		break
	}
	assert.Equal(t, regression.POSITIVE, ranges[key].ByAlertID["foo"].LowStatus.Status)

	ranges, err = store.Range(ctx, begin, end)
	require.NoError(t, err)
	assert.Len(t, ranges, 1)
}

// TriageNonExistentRegression tests that the implementation of the
// regression.Store interface fails as expected when triaging an unknown
// regression.
func TriageNonExistentRegression(t *testing.T, store regression.Store) {
	ctx, c := getTestVars()

	tr := regression.TriageStatus{
		Status:  regression.POSITIVE,
		Message: "bad",
	}
	// Try triaging a regression that doesn't exist.
	err := store.TriageHigh(ctx, c, "bar", tr)
	assert.Error(t, err)
}

// Write tests that the implementation of the regression.Store interface can
// bulk write Regressions.
func Write(t *testing.T, store regression.Store) {
	ctx, c := getTestVars()

	now := time.Unix(c.Timestamp, 0)
	begin := now.Add(-time.Hour).Unix()
	end := now.Add(time.Hour).Unix()

	lookup := func(c *cid.CommitID) (*cid.CommitDetail, error) {
		return &cid.CommitDetail{
			CommitID: cid.CommitID{
				Offset: 2,
			},
			Timestamp: 1479235651 + 10,
		}, nil
	}
	reg := &regression.AllRegressionsForCommit{
		ByAlertID: map[string]*regression.Regression{
			"foo": regression.NewRegression(),
		},
	}
	err := store.Write(ctx, map[types.CommitNumber]*regression.AllRegressionsForCommit{2: reg}, lookup)
	require.NoError(t, err)
	ranges, err := store.Range(ctx, begin, end)
	assert.NoError(t, err)
	assert.Len(t, ranges, 1)
	assert.Equal(t, reg, ranges[2])
}

// SubTestFunction is a func we will call to test one aspect of an
// implementation of regression.Store.
type SubTestFunction func(t *testing.T, store regression.Store)

// SubTests are all the subtests we have for regression.Store.
var SubTests = map[string]SubTestFunction{
	"SetLowAndTriage":             SetLowAndTriage,
	"TriageNonExistentRegression": TriageNonExistentRegression,
	"TestWrite":                   Write,
}
