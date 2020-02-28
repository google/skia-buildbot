package dsregressionstore

import (
	"testing"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/regression/regressiontest"
)

// TestDS test storing regressions in Google Cloud Datastore.
func TestDS(t *testing.T) {
	unittest.LargeTest(t)

	for name, subTest := range regressiontest.SubTests {
		t.Run(name, func(t *testing.T) {
			cleanup := testutil.InitDatastore(t, ds.REGRESSION)
			defer cleanup()
			store := NewRegressionStoreDS()
			subTest(t, store)
		})
	}
}
