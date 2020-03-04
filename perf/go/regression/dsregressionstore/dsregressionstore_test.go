package dsregressionstore

import (
	"context"
	"testing"
	"time"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/regressiontest"
)

// getDetailLookupForTests returns a lookup function that will work with the
// tests we run against the datastore, that is we are faking CommitIDs and
// timestamps so they align with what the tests use.
func getDetailLookupForTests() regression.DetailLookup {
	now := time.Now()
	lookupValues := []*cid.CommitDetail{}
	for i := 0; i < 4; i++ {
		lookupValues = append(lookupValues, &cid.CommitDetail{
			CommitID: cid.CommitID{
				Offset: i,
			},
			// Use time.Duration(i-1) to ensure that we catch commitNumber=1,
			// which will be written with a timestamp of ~now, but not exactly,
			// so we fudge the numbers here.
			Timestamp: now.Add(time.Duration(i-1) * time.Minute).Unix(),
		})
	}

	lookup := func(ctx context.Context, c *cid.CommitID) (*cid.CommitDetail, error) {
		return lookupValues[c.Offset], nil
	}
	return lookup
}

// TestDS test storing regressions in Google Cloud Datastore.
func TestDS(t *testing.T) {
	// Cloud Datastore Emulator emulates even the eventual consistency of Cloud
	// Datastore, so we only run this as a Manual test, otherwise this test is
	// flaky on the waterfall.
	unittest.ManualTest(t)

	for name, subTest := range regressiontest.SubTests {
		t.Run(name, func(t *testing.T) {
			cleanup := testutil.InitDatastore(t, ds.REGRESSION)
			defer cleanup()
			store := NewRegressionStoreDS(regressiontest.GetDetailLookupForTests())
			subTest(t, store)
		})
	}
}
