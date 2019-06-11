package ds_ignorestore

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	ds_testutil "go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/ignore/testutils"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
)

func TestCloudIgnoreStore(t *testing.T) {
	unittest.LargeTest(t)

	// Run against the locally running emulator.
	cleanup := ds_testutil.InitDatastore(t, ds.IGNORE_RULE, ds.HELPER_RECENT_KEYS)
	defer cleanup()

	store, err := New(ds.DS)
	assert.NoError(t, err)
	testutils.IgnoreStoreAll(t, store)
}

func TestFilterIgnored(t *testing.T) {
	unittest.LargeTest(t)

	// Run against the locally running emulator.
	cleanup := ds_testutil.InitDatastore(t, ds.IGNORE_RULE, ds.HELPER_RECENT_KEYS)
	defer cleanup()

	store, err := New(ds.DS)
	assert.NoError(t, err)

	// With no ignore rules, nothing is filtered
	ft, pm, err := ignore.FilterIgnored(data.MakeTestTile(), store)
	assert.NoError(t, err)
	assert.Empty(t, pm)
	assert.Equal(t, data.MakeTestTile(), ft)

	// Add one ignore rule

	future := time.Now().Add(time.Hour)
	store.Create(ignore.NewIgnoreRule("user@example.com", future, "device=crosshatch", "note"))

	ft, pm, err = ignore.FilterIgnored(data.MakeTestTile(), store)
	assert.NoError(t, err)
	assert.Equal(t, paramtools.ParamMatcher{
		{
			"device": {"crosshatch"},
		},
	}, pm)
	assert.Len(t, ft.Traces, 4)
	assert.NotContains(t, ft.Traces, data.CrosshatchTraceID1)
	assert.NotContains(t, ft.Traces, data.CrosshatchTraceID2)
}
