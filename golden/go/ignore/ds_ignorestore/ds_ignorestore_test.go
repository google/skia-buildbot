package ds_ignorestore

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	ds_testutil "go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/ignore/testutils"
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
