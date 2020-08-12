package isolate_cache_testutils

import (
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/task_scheduler/go/isolate_cache"
)

// SetupSharedBigTable performs setup in BigTable, using the given project and
// BigTable instance. This is useful when a given test has multiple entities
// which are backed by BigTable and should use the same instance. Returns a
// cleanup function which should be deferred.
func SetupSharedBigTable(t sktest.TestingT, project, instance string) func() {
	require.NoError(t, bt.InitBigtable(project, instance, isolate_cache.BT_TABLE, []string{isolate_cache.BT_COLUMN_FAMILY}))
	return func() {
		require.NoError(t, bt.DeleteTables(project, instance, isolate_cache.BT_TABLE))
	}
}
