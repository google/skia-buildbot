package sqlalertstore

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/alerts/alertstest"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func TestSQLAlertStore_SQLite(t *testing.T) {
	unittest.LargeTest(t)

	db, cleanup := sqltest.NewSQLite3DBForTests(t)
	defer cleanup()

	store, err := New(db, perfsql.SQLiteDialect)
	require.NoError(t, err)

	alertstest.Store_SaveListDelete(t, store)
}

func TestSQLAlertStore_CockroachDB(t *testing.T) {
	unittest.LargeTest(t)

	db, cleanup := sqltest.NewCockroachDBForTests(t, "alertstore")
	// If this test timeouts then comment out the cleanup(), as it may hide the
	// actual errors.
	defer cleanup()

	store, err := New(db, perfsql.CockroachDBDialect)
	require.NoError(t, err)

	alertstest.Store_SaveListDelete(t, store)
}
