// Package regressiontest has common utility funcs for testing the regression
// package.
package regressiontest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
)

// Store_SetAndTriage tests that an instance of the regression.Store interface
// operates correctly.
func Store_SetAndTriage(t *testing.T, store regression.Store) {
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
	r.SetLow("foo", df, cl)
	_, ok := r.ByAlertID["foo"]
	assert.True(t, ok)
	assert.False(t, r.Triaged())

	// Test store.
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
	err = testutils.EventuallyConsistent(time.Second, func() error {
		ranges, err = store.Range(ctx, begin, end)
		assert.NoError(t, err)
		key := ""
		for key = range ranges {
			break
		}
		if ranges[key].ByAlertID["foo"].LowStatus.Status == regression.POSITIVE {
			return nil
		}
		return testutils.TryAgainErr
	})
	assert.NoError(t, err)

	count, err = store.CountUntriaged(ctx)
	assert.Equal(t, count, 0)

	ranges, err = store.Range(ctx, begin, end)
	assert.NoError(t, err)
	assert.Len(t, ranges, 1)

	// Try triaging a regression that doesn't exist.
	err = store.TriageHigh(ctx, c, "bar", tr)
	assert.Error(t, err)

	ranges, err = store.Range(ctx, begin, end)
	assert.NoError(t, err)
	assert.Len(t, ranges, 1)

	count, err = store.CountUntriaged(ctx)
	assert.Equal(t, count, 0)

	lookup := func(c *cid.CommitID) (*cid.CommitDetail, error) {
		return &cid.CommitDetail{
			CommitID: cid.CommitID{
				Offset: 2,
			},
			Timestamp: 1479235651 + 10,
		}, nil
	}
	err = store.Write(ctx, map[string]*regression.Regressions{"master-000002": ranges["master-000001"]}, lookup)
	assert.NoError(t, err)
	ranges, err = store.Range(ctx, begin, end)
	assert.NoError(t, err)
	assert.Len(t, ranges, 2)
	_, ok = ranges["master-000002"]
	assert.True(t, ok)
}

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

func Store_SetLowAndTriage(t *testing.T, store regression.Store) {
	ctx, r, c, df, cl := getTestVars()

	// Can be moved out of here.
	r.SetLow("foo", df, cl)
	_, ok := r.ByAlertID["foo"]
	assert.True(t, ok)
	assert.False(t, r.Triaged())

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
	err = testutils.EventuallyConsistent(time.Second, func() error {
		ranges, err = store.Range(ctx, begin, end)
		assert.NoError(t, err)
		key := ""
		for key = range ranges {
			break
		}
		if ranges[key].ByAlertID["foo"].LowStatus.Status == regression.POSITIVE {
			return nil
		}
		return testutils.TryAgainErr
	})
	assert.NoError(t, err)

	count, err = store.CountUntriaged(ctx)
	assert.Equal(t, count, 0)

	ranges, err = store.Range(ctx, begin, end)
	assert.NoError(t, err)
	assert.Len(t, ranges, 1)
}

func Store_TriageNonExistentRegression(t *testing.T, store regression.Store) {
	ctx, r, c, df, cl := getTestVars()

	tr := regression.TriageStatus{
		Status:  regression.POSITIVE,
		Message: "bad",
	}
	// Try triaging a regression that doesn't exist.
	err := store.TriageHigh(ctx, c, "bar", tr)
	assert.Error(t, err)

}

func Store_WriteRegression(t *testing.T, store regression.Store) {
	ctx, r, c, df, cl := getTestVars()

	lookup := func(c *cid.CommitID) (*cid.CommitDetail, error) {
		return &cid.CommitDetail{
			CommitID: cid.CommitID{
				Offset: 2,
			},
			Timestamp: 1479235651 + 10,
		}, nil
	}
	err := store.Write(ctx, map[string]*regression.Regressions{"master-000002": ranges["master-000001"]}, lookup)
	assert.NoError(t, err)
	ranges, err = store.Range(ctx, begin, end)
	assert.NoError(t, err)
	assert.Len(t, ranges, 2)
	_, ok = ranges["master-000002"]
	assert.True(t, ok)
}
