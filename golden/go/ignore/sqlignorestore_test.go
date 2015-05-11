package ignore

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/database/testutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/db"
)

func TestSQLIgnoreStore(t *testing.T) {
	// Set up the database. This also locks the db until this test is finished
	// causing similar tests to wait.
	migrationSteps := db.MigrationSteps()
	mysqlDB := testutil.SetupMySQLTestDatabase(t, migrationSteps)
	defer mysqlDB.Close(t)

	vdb, err := testutil.LocalTestDatabaseConfig(migrationSteps).NewVersionedDB()
	assert.Nil(t, err)
	defer testutils.AssertCloses(t, vdb)

	store := NewSQLIgnoreStore(vdb)
	testIgnoreStore(t, store)
}
