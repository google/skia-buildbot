package sqlcoveragestore

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func setUp(t *testing.T) (*CoverageStore, pool.Pool) {
	db := sqltest.NewCockroachDBForTests(t, "coveragestore")
	store, err := New(db)
	require.NoError(t, err)

	return store, db
}

// Tests a hypothetical pipeline of Store.
func TestStore_SaveListDelete(t *testing.T) {
	// TODO(seawardt: Hook up Test to DB emulator)
}
