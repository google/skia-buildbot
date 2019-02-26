package isolate_cache

import (
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/bt"
	bt_testutil "go.skia.org/infra/go/bt/testutil"
	"go.skia.org/infra/go/testutils"
)

var tableConfig = bt.TableConfig{
	BT_TABLE: {
		BT_COLUMN_FAMILY,
	},
}

// SetupBigTable performs setup in BigTable. Returns the BigTable instance name
// which should be used to instantiate Cache and a cleanup function which should
// be deferred.
func SetupBigTable(t testutils.TestingT) (string, string, func()) {
	return bt_testutil.SetupBigTable(t, tableConfig)
}

// SetupBigTable performs setup in BigTable, using the given project and
// BigTable instance. This is useful when a given test has multiple entities
// which are backed by BigTable and should use the same instance.
func SetupExistingBigTable(t testutils.TestingT, project, instance string) func() {
	assert.NoError(t, bt.InitBigtable(project, instance, tableConfig))
	return func() {
		assert.NoError(t, bt.DeleteTables(project, instance, tableConfig))
	}
}
