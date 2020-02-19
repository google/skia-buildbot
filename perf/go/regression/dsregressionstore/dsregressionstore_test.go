package dsregressionstore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
)

// TestDS test storing regressions in the datastore.
func TestDS(t *testing.T) {
	unittest.ManualTest(t)

	cleanup := testutil.InitDatastore(t, ds.REGRESSION)
	defer cleanup()

	ctx := context.Background()
	st := NewRegressionStoreDS()

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
	isNew, err := st.SetLow(ctx, c, "foo", df, cl)
	assert.True(t, isNew)
	assert.NoError(t, err)

	// Overwrite a regression.
	isNew, err = st.SetLow(ctx, c, "foo", df, cl)
	assert.False(t, isNew)
	assert.NoError(t, err)

	// Confirm new regression is present.
	ranges, err := st.Range(ctx, begin, end)
	assert.NoError(t, err)
	assert.Len(t, ranges, 1)

	count, err := st.CountUntriaged(ctx)
	assert.Equal(t, count, 1)

	// Triage existing regression.
	tr := regression.TriageStatus{
		Status:  regression.POSITIVE,
		Message: "bad",
	}
	err = st.TriageLow(ctx, c, "foo", tr)
	assert.NoError(t, err)

	// Confirm regression is triaged.
	err = testutils.EventuallyConsistent(time.Second, func() error {
		ranges, err = st.Range(ctx, begin, end)
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

	count, err = st.CountUntriaged(ctx)
	assert.Equal(t, count, 0)

	ranges, err = st.Range(ctx, begin, end)
	assert.NoError(t, err)
	assert.Len(t, ranges, 1)

	// Try triaging a regression that doesn't exist.
	err = st.TriageHigh(ctx, c, "bar", tr)
	assert.Error(t, err)

	ranges, err = st.Range(ctx, begin, end)
	assert.NoError(t, err)
	assert.Len(t, ranges, 1)

	count, err = st.CountUntriaged(ctx)
	assert.Equal(t, count, 0)

	lookup := func(c *cid.CommitID) (*cid.CommitDetail, error) {
		return &cid.CommitDetail{
			CommitID: cid.CommitID{
				Offset: 2,
			},
			Timestamp: 1479235651 + 10,
		}, nil
	}
	err = st.Write(ctx, map[string]*regression.Regressions{"master-000002": ranges["master-000001"]}, lookup)
	assert.NoError(t, err)
	ranges, err = st.Range(ctx, begin, end)
	assert.NoError(t, err)
	assert.Len(t, ranges, 2)
	_, ok = ranges["master-000002"]
	assert.True(t, ok)
}
