package dsregressionstore

import (
	"context"
	"testing"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/regression/regressiontest"
)

// TestDS test storing regressions in Google Cloud Datastore.
func TestDS(t *testing.T) {
	// Cloud Datastore Emulator emulates even the eventual consistency of Cloud
	// Datastore, so we only run this as a Manual test, otherwise this test is
	// flaky on the waterfall.
	unittest.ManualTest(t)

	lookup := func(ctx context.Context, c *cid.CommitID) (*cid.CommitDetail, error) {
		return &cid.CommitDetail{
			CommitID: cid.CommitID{
				Offset: 2,
			},
			Timestamp: 1479235651 + 10,
		}, nil
	}

	for name, subTest := range regressiontest.SubTests {
		t.Run(name, func(t *testing.T) {
			cleanup := testutil.InitDatastore(t, ds.REGRESSION)
			defer cleanup()
			store := NewRegressionStoreDS(lookup)
			subTest(t, store)
		})
	}
}
