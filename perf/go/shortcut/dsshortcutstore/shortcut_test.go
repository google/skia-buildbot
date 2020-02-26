package dsshortcutstore

import (
	"testing"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/shortcut/shortcuttest"
)

func TestInsertGet(t *testing.T) {
	unittest.LargeTest(t)
	cleanup := testutil.InitDatastore(t, ds.SHORTCUT)

	defer cleanup()

	store := New()
	shortcuttest.InsertGet(t, store)
}
