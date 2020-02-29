package dsshortcutstore

import (
	"testing"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/shortcut/shortcuttest"
)

func TestShortcutStore(t *testing.T) {
	unittest.LargeTest(t)

	for name, subTest := range shortcuttest.SubTests {
		t.Run(name, func(t *testing.T) {
			cleanup := testutil.InitDatastore(t, ds.SHORTCUT)
			defer cleanup()
			store := New()
			subTest(t, store)
		})
	}

}
