package dsalertstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/alerts"
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

func TestUpgradeAlert_LegacyPropertiesAreUpgradedOnSaveAndList_Success(t *testing.T) {
	unittest.ManualTest(t)
	cleanup := testutil.InitDatastore(t, ds.ALERT)
	defer cleanup()
	store := New()

	ctx := context.Background()

	// Create an Alert in the old-style, i.e. with int values for Direction and
	// State. And empty strings for their *AsString counter-parts.
	cfg := alerts.NewConfig()
	cfg.StateAsString = ""
	cfg.DirectionAsString = ""
	cfg.State = 0
	cfg.Direction = 1

	// Store it.
	err := store.Save(ctx, cfg)
	require.NoError(t, err)

	// Confirm that the int values are converted to their string values when
	// read.
	cfgs, err := store.List(ctx, false)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 1)
	assert.Equal(t, alerts.ACTIVE, cfgs[0].StateAsString)
	assert.Equal(t, alerts.UP, cfgs[0].DirectionAsString)
}
