package graphsshortcutstore

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/graphsshortcut/graphsshortcuttest"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func TestShortcutStore(t *testing.T) {

	for name, subTest := range graphsshortcuttest.SubTests {
		t.Run(name, func(t *testing.T) {
			db := sqltest.NewSpannerDBForTests(t, "graphsshortcutstore")
			store, err := New(db)
			require.NoError(t, err)
			subTest(t, store)
		})
	}
}
