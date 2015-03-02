package types

import (
	"testing"

	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/database/testutil"
	"go.skia.org/infra/golden/go/db"
)

func TestSQLIgnoreStore(t *testing.T) {
	// Set up the database. This also locks the db until this test is finished
	// causing similar tests to wait.
	migrationSteps := db.MigrationSteps()
	mysqlDB := testutil.SetupMySQLTestDatabase(t, migrationSteps)
	defer mysqlDB.Close()

	vdb := database.NewVersionedDB(testutil.LocalTestDatabaseConfig(migrationSteps))
	defer vdb.Close()

	store := NewSQLIgnoreStore(vdb)
	testIgnoreStore(t, store)
}
