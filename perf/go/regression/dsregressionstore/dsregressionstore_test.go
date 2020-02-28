package dsregressionstore

import (
	"testing"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/regression/regressiontest"
)

// TestDS test storing regressions in the datastore.
func TestDS(t *testing.T) {
	unittest.ManualTest(t)

	cleanup := testutil.InitDatastore(t, ds.REGRESSION)
	defer cleanup()

	store := NewRegressionStoreDS()
	regressiontest.Store_SetAndTriage(t, store)
}
