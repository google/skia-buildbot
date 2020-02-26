package sqlshortcutstore

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/shortcut/shortcuttest"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func TestInsertGet_SQLite(t *testing.T) {
	unittest.LargeTest(t)

	db, cleanup := sqltest.NewSQLite3DBForTests(t)
	defer cleanup()

	store, err := New(db, perfsql.SQLiteDialect)
	require.NoError(t, err)

	shortcuttest.InsertGet(t, store)
}

func TestInsertGet_CockroachDB(t *testing.T) {
	unittest.LargeTest(t)

	db, cleanup := sqltest.NewCockroachDBForTests(t, "shortcutstore")
	defer cleanup()

	store, err := New(db, perfsql.CockroachDBDialect)
	require.NoError(t, err)

	shortcuttest.InsertGet(t, store)
}
