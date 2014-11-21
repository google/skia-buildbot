package testutil

import (
	"fmt"
	"os"
	"strings"
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

	// Name of the MySQL lock
	SQL_LOCK = "mysql_testlock"
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
// given migration steps. See Get for required credentials.
// The test assumes that the database is empty and that the readwrite user is
// not allowed to create/drop/alter tables.
func MySQLVersioningTests(t *testing.T, dbName string, migrationSteps []database.MigrationStep) {
	// OpenDB as root user and remove all tables.
	rootConf := &database.DatabaseConfig{
		MySQLString:    GetTestMySQLConnStr(t, "root", dbName),
		MigrationSteps: migrationSteps,
	}
	lockVdb := GetMySQlLock(t, rootConf)
	defer func() {
		ReleaseMySQLLock(t, lockVdb)
		lockVdb.Close()
	}()

	rootVdb := database.NewVersionedDB(rootConf)
	assert.True(t, rootVdb.IsMySQL)
	ClearMySQLTables(t, rootVdb)
	assert.Nil(t, rootVdb.Close())

	// Configuration for the readwrite user without DDL privileges.
	readWriteConf := &database.DatabaseConfig{
		MySQLString:    GetTestMySQLConnStr(t, "readwrite", dbName),
		MigrationSteps: migrationSteps,
	}

	// Open DB as readwrite user and make sure it fails because of a missing
	// version table.
	// Note: This requires the database to be empty.
	assert.Panics(t, func() {
		database.NewVersionedDB(readWriteConf)
	})

	rootVdb = database.NewVersionedDB(rootConf)
	testDBVersioning(t, rootVdb)

	// Make sure it doesn't panic for readwrite user after the migration
	assert.NotPanics(t, func() {
		database.NewVersionedDB(readWriteConf)
	})
}

// Returns a connection string to the local MySQL server and the given database.
// The test will be skipped if these environement variables are not set:
//      MYSQL_TESTING_RWPW   (password of readwrite user)
//      MYSQL_TESTING_ROOTPW (password of the db root user)
func GetTestMySQLConnStr(t *testing.T, user string, dbName string) string {
	rwUserPw, rootPw := os.Getenv("MYSQL_TESTING_RWPW"), os.Getenv("MYSQL_TESTING_ROOTPW")
	if testing.Short() {
		t.Skip("Skipping test against MySQL in short mode.")
	}

	// Skip this test unless there are environment variables with the rwuser and
	// root password for the local MySQL instance.
	if (rwUserPw == "") || (rootPw == "") {
		t.Skip("Skipping test against MySQL. Set 'MYSQL_TESTING_ROOTPW' and 'MYSQL_TESTING_RWPW' to enable tests.")
	}
	pw := rwUserPw
	if user == "root" {
		pw = rootPw
	}
	return fmt.Sprintf(MYSQL_DB_OPEN, user, pw, dbName)
}

// Get a lock from MySQL to serialize DB tests.
func GetMySQlLock(t *testing.T, conf *database.DatabaseConfig) *database.VersionedDB {
	vdb := database.NewVersionedDB(conf)
	_, err := vdb.DB.Exec("SELECT GET_LOCK(?,30)", SQL_LOCK)
	assert.Nil(t, err)
	return vdb
}

// Release the MySQL lock.
func ReleaseMySQLLock(t *testing.T, vdb *database.VersionedDB) {
	_, err := vdb.DB.Exec("SELECT RELEASE_LOCK(?)", SQL_LOCK)
	assert.Nil(t, err)
}

// Remove all tables from the database.
func ClearMySQLTables(t *testing.T, vdb *database.VersionedDB) {
	stmt := `SHOW TABLES`
	rows, err := vdb.DB.Query(stmt)
	assert.Nil(t, err)
	defer rows.Close()

	names := make([]string, 0)
	var tableName string
	for rows.Next() {
		rows.Scan(&tableName)
		names = append(names, tableName)
	}

	if len(names) > 0 {
		stmt = "DROP TABLE " + strings.Join(names, ",")
		_, err = vdb.DB.Exec(stmt)
		assert.Nil(t, err)
	}
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
