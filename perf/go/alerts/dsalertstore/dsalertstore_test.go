package dsalertstore

import (
	"testing"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/alerts/alertstest"
)

func TestAlertStoreDS(t *testing.T) {
	unittest.LargeTest(t)

	cleanup := testutil.InitDatastore(t, ds.ALERT)
	defer cleanup()

	store := New()
	alertstest.Store_SaveListDelete(t, store)
}
