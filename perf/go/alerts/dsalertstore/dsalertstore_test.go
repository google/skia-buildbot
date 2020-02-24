package dsalertstore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/alerts"
)

func TestAlertStoreDS(t *testing.T) {
	unittest.ManualTest(t)

	cleanup := testutil.InitDatastore(t, ds.ALERT)
	defer cleanup()

	ctx := context.Background()
	// Test saving one alert.
	a := New()
	cfg := alerts.NewConfig()
	cfg.Query = "source_type=svg"
	cfg.DisplayName = "bar"
	err := a.Save(ctx, cfg)
	assert.NoError(t, err)

	// Confirm it appears in the list.
	cfgs, err := a.List(ctx, false)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 1)

	// Delete it.
	err = a.Delete(ctx, int(cfgs[0].ID))
	assert.NoError(t, err)

	// Confirm it is still there if we list deleted configs.
	cfgs, err = a.List(ctx, true)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 1)

	// Confirm it is not there if we don't list deleted configs.
	cfgs, err = a.List(ctx, false)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 0)

	// Store a second config.
	cfg = alerts.NewConfig()
	cfg.Query = "source_type=skp"
	cfg.DisplayName = "foo"
	err = a.Save(ctx, cfg)
	assert.NoError(t, err)

	time.Sleep(1)
	// Confirm they are both listed when including deleted configs, and they are
	// ordered by DisplayName.
	cfgs, err = a.List(ctx, true)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 2)
	assert.Equal(t, "bar", cfgs[0].DisplayName)
	assert.Equal(t, "foo", cfgs[1].DisplayName)
}
