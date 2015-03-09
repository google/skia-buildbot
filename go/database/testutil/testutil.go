package testutil

import (
	"strings"
	"testing"
)

import (
	"github.com/jmoiron/sqlx"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/database"
)

// Connection string to the local MySQL database for testing.
const (
	// String to open a local database for testing. The string formatting
	// parameters are: username, password, database.
	MYSQL_DB_OPEN = "%s:%s@tcp(localhost:3306)/%s?parseTime=true"

	// Name of the MySQL lock
	SQL_LOCK = "mysql_testlock"

	// Name of the shared test database.
	TEST_DB_HOST = "localhost"
	TEST_DB_PORT = 3306
	TEST_DB_NAME = "sk_testing"

	// Names of test users. These users should have no password and be
	// limited to accessing the sk_testing database.
	USER_ROOT = "test_root"
	USER_RW   = "test_rw"

	// Empty password for testing.
	TEST_PASSWORD = ""
)

// LocalTestDatabaseConfig returns a DatabaseConfig appropriate for local
// testing.
func LocalTestDatabaseConfig(m []database.MigrationStep) *database.DatabaseConfig {
	return database.NewDatabaseConfig(USER_RW, "", TEST_DB_HOST, TEST_DB_PORT, TEST_DB_NAME, m)
}

// LocalTestRootDatabaseConfig returns a DatabaseConfig appropriate for local
// testing, with root access.
func LocalTestRootDatabaseConfig(m []database.MigrationStep) *database.DatabaseConfig {
	return database.NewDatabaseConfig(USER_ROOT, "", TEST_DB_HOST, TEST_DB_PORT, TEST_DB_NAME, m)
}

// Creates an MySQL test database and runs migration tests against it using the
// given migration steps. See Get for required credentials.
// The test assumes that the database is empty and that the readwrite user is
// not allowed to create/drop/alter tables.
func MySQLVersioningTests(t *testing.T, dbName string, migrationSteps []database.MigrationStep) {
	// OpenDB as root user and remove all tables.
	rootConf := LocalTestRootDatabaseConfig(migrationSteps)
	lockDb := GetMySQlLock(t, rootConf)
	defer lockDb.Close(t)

	rootVdb := database.NewVersionedDB(rootConf)
	ClearMySQLTables(t, rootVdb)
	assert.Nil(t, rootVdb.Close())

	// Configuration for the readwrite user without DDL privileges.
	readWriteConf := LocalTestDatabaseConfig(migrationSteps)

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

	// Downgrade database, removing most if not all tables.
	downgradeDB(t, rootVdb)
	ClearMySQLTables(t, rootVdb)
}

type LockDB struct {
	DB *sqlx.DB
}

// Get a lock from MySQL to serialize DB tests.
func GetMySQlLock(t *testing.T, conf *database.DatabaseConfig) *LockDB {
	db, err := sqlx.Open("mysql", conf.MySQLString)
	assert.Nil(t, err)

	for {
		var result int
		assert.Nil(t, db.Get(&result, "SELECT GET_LOCK(?,5)", SQL_LOCK))

		// We obtained the lock. If not try again.
		if result == 1 {
			return &LockDB{db}
		}
	}
}

// Release the MySQL lock.
func (l *LockDB) Close(t *testing.T) {
	var result int
	assert.Nil(t, l.DB.Get(&result, "SELECT RELEASE_LOCK(?)", SQL_LOCK))
	assert.Equal(t, result, 1)
	assert.Nil(t, l.DB.Close())
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

// MySQLTestDatabase is a convenience struct for using a test database which
// starts in a clean state.
type MySQLTestDatabase struct {
	lockDb  *LockDB
	rootVdb *database.VersionedDB
	t       *testing.T
}

// SetupMySQLTestDatabase returns a MySQLTestDatabase in a clean state. It must
// be closed after use.
//
// Example usage:
//
// db := SetupMySQLTestDatabase(t, migrationSteps)
// defer db.Close()
// ... Tests here ...
func SetupMySQLTestDatabase(t *testing.T, migrationSteps []database.MigrationStep) *MySQLTestDatabase {
	conf := LocalTestRootDatabaseConfig(migrationSteps)
	lock := GetMySQlLock(t, conf)
	rootVdb := database.NewVersionedDB(conf)
	ClearMySQLTables(t, rootVdb)
	if err := rootVdb.Close(); err != nil {
		t.Fatal(err)
	}
	rootVdb = database.NewVersionedDB(conf)
	if err := rootVdb.Migrate(rootVdb.MaxDBVersion()); err != nil {
		t.Fatal(err)
	}
	return &MySQLTestDatabase{lock, rootVdb, t}
}

func (d *MySQLTestDatabase) Close(t *testing.T) {
	assert.Nil(t, d.rootVdb.Migrate(0))
	assert.Nil(t, d.rootVdb.Close())
	d.lockDb.Close(t)
}

// Test wether the migration steps execute correctly.
func testDBVersioning(t *testing.T, vdb *database.VersionedDB) {
	// get the DB version
	dbVersion, err := vdb.DBVersion()
	assert.Nil(t, err)
	maxVersion := vdb.MaxDBVersion()

	downgradeDB(t, vdb)

	// upgrade the the latest version
	err = vdb.Migrate(maxVersion)
	assert.Nil(t, err)
	dbVersion, err = vdb.DBVersion()
	assert.Nil(t, err)
	assert.Equal(t, maxVersion, dbVersion)
}

func downgradeDB(t *testing.T, vdb *database.VersionedDB) {
	// downgrade to 0
	err := vdb.Migrate(0)
	assert.Nil(t, err)
	dbVersion, err := vdb.DBVersion()
	assert.Nil(t, err)
	assert.Equal(t, 0, dbVersion)
}
