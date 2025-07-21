package sqlshortcutstore

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/shortcut/shortcuttest"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func TestShortcutStore(t *testing.T) {

	for name, subTest := range shortcuttest.SubTests {
		t.Run(name, func(t *testing.T) {
			db := sqltest.NewSpannerDBForTests(t, "shortcutstore")
			store, err := New(db)
			require.NoError(t, err)
			subTest(t, store)
		})
	}
}
