package dsalertstore

import (
	"testing"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/alerts/alertstest"
)

func TestAlertStoreDS(t *testing.T) {
	// Cloud Datastore Emulator emulates even the eventual consistency of Cloud
	// Datastore, so we only run this as a Manual test, otherwise this test is
	// flaky on the waterfall.
	unittest.ManualTest(t)

	for name, subTest := range alertstest.SubTests {
		t.Run(name, func(t *testing.T) {
			cleanup := testutil.InitDatastore(t, ds.ALERT)
			defer cleanup()
			store := New()
			subTest(t, store)
		})
	}
}
