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
