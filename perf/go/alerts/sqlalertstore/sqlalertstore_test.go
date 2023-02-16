package sqlalertstore

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/alerts/alertstest"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func TestSQLAlertStore_CockroachDB(t *testing.T) {

	for name, subTest := range alertstest.SubTests {
		t.Run(name, func(t *testing.T) {
			db := sqltest.NewCockroachDBForTests(t, "alertstore")
			store, err := New(db)
			require.NoError(t, err)
			subTest(t, store)
		})
	}
}
