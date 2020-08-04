package sqlalertstore

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/alerts/alertstest"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func TestSQLAlertStore_CockroachDB(t *testing.T) {
	unittest.LargeTest(t)

	for name, subTest := range alertstest.SubTests {
		t.Run(name, func(t *testing.T) {
			db, cleanup := sqltest.NewCockroachDBForTests(t, "alertstore", sqltest.ApplyMigrations)
			// If this test timeouts then comment out the cleanup(), as it may hide the
			// actual errors.
			defer cleanup()

			store, err := New(db)
			require.NoError(t, err)
			subTest(t, store)
		})
	}
}
