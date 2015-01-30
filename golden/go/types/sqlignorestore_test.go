package types

import (
	"testing"

	"skia.googlesource.com/buildbot.git/go/database"
	"skia.googlesource.com/buildbot.git/go/database/testutil"
	"skia.googlesource.com/buildbot.git/golden/go/db"
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
