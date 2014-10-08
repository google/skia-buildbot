package testutil

import (
	"fmt"
	"os"
	"testing"
)

import (
	// Using 'require' which is like using 'assert' but causes tests to fail.
	assert "github.com/stretchr/testify/require"

	"skia.googlesource.com/buildbot.git/go/database"
)

// Connection string to the local MySQL database for testing.
const (
	// String to open a local database for testing. The string formatting
	// parameters are: username, password, database.
	MYSQL_DB_OPEN = "%s:%s@tcp(localhost:3306)/%s?parseTime=true"

	// File path to the local SQLite testing databse.
	SQLITE_DB_PATH = "./testing.db"
)

// Creates an SQLite test database and runs migration tests against it using the
// given migration steps.
func SQLiteVersioningTests(t *testing.T, migrationSteps []database.MigrationStep) {
	// Initialize without argument to test against SQLite3
	conf := &database.DatabaseConfig{
		SQLiteFilePath: SQLITE_DB_PATH,
		MigrationSteps: migrationSteps,
	}

	vdb := database.NewVersionedDB(conf)
	assert.False(t, vdb.IsMySQL)
	testDBVersioning(t, vdb)
}

// Creates an MySQL test database and runs migration tests against it using the
// given migration steps. The test will be skipped if these environement
// variables are not set:
//      MYSQL_TESTING_RWPW   (password of readwrite user)
//      MYSQL_TESTING_ROOTPW (password of the db root user)
// The test assumes that the database is empty and that the readwrite user is
// not allowed to create/drop/alter tables.
func MySQLVersioningTests(t *testing.T, dbName string, migrationSteps []database.MigrationStep) {
	rwUserPw, rootPw := os.Getenv("MYSQL_TESTING_RWPW"), os.Getenv("MYSQL_TESTING_ROOTPW")
	// Skip this test unless there are environment variables with the rwuser and
	// root password for the local MySQL instance.
	if (rwUserPw == "") || (rootPw == "") {
		t.Skip("Skipping versioning tests against MySQL. Set 'MYSQL_TESTING_ROOTPW' and 'MYSQL_TESTING_RWPW' to enable tests.")
	}

	readWriteConf := &database.DatabaseConfig{
		MySQLString:    fmt.Sprintf(MYSQL_DB_OPEN, "readwrite", rwUserPw, dbName),
		MigrationSteps: migrationSteps,
	}

	// Open DB as readwrite user and make sure it fails because of a missing
	// version table.
	// Note: This requires the database to be empty.
	assert.Panics(t, func() {
		database.NewVersionedDB(readWriteConf)
	})

	// OpenDB as root user and make sure migration works.
	// Initialize to test against local MySQL db
	rootConf := &database.DatabaseConfig{
		MySQLString:    fmt.Sprintf(MYSQL_DB_OPEN, "root", rootPw, dbName),
		MigrationSteps: migrationSteps,
	}
	vdb := database.NewVersionedDB(rootConf)
	assert.True(t, vdb.IsMySQL)
	testDBVersioning(t, vdb)

	// Make sure it doesn't panic for readwrite user after the migration
	assert.NotPanics(t, func() {
		database.NewVersionedDB(readWriteConf)
	})
}

// Test wether the migration steps execute correctly.
func testDBVersioning(t *testing.T, vdb *database.VersionedDB) {
	// get the DB version
	dbVersion, err := vdb.DBVersion()
	assert.Nil(t, err)
	maxVersion := vdb.MaxDBVersion()

	// downgrade to 0
	err = vdb.Migrate(0)
	assert.Nil(t, err)
	dbVersion, err = vdb.DBVersion()
	assert.Nil(t, err)
	assert.Equal(t, 0, dbVersion)

	// upgrade the the latest version
	err = vdb.Migrate(maxVersion)
	assert.Nil(t, err)
	dbVersion, err = vdb.DBVersion()
	assert.Nil(t, err)
	assert.Equal(t, maxVersion, dbVersion)
}
