package sqlshortcutstore

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/shortcut/shortcuttest"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func TestShortcutStore_CockroachDB(t *testing.T) {
	unittest.LargeTest(t)

	for name, subTest := range shortcuttest.SubTests {
		t.Run(name, func(t *testing.T) {
			db, cleanup := sqltest.NewCockroachDBForTests(t, "shortcutstore", sqltest.ApplyMigrations)
			defer cleanup()
			store, err := New(db, perfsql.CockroachDBDialect)
			require.NoError(t, err)
			subTest(t, store)
		})
	}
}
