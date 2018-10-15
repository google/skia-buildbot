package testutil

import (
	"strings"

	"github.com/jmoiron/sqlx"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
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
	return &database.DatabaseConfig{
		User:           USER_RW,
		Host:           TEST_DB_HOST,
		Port:           TEST_DB_PORT,
		Name:           TEST_DB_NAME,
		MigrationSteps: m,
	}
}

// LocalTestRootDatabaseConfig returns a DatabaseConfig appropriate for local
// testing, with root access.
func LocalTestRootDatabaseConfig(m []database.MigrationStep) *database.DatabaseConfig {
	return &database.DatabaseConfig{
		User:           USER_ROOT,
		Host:           TEST_DB_HOST,
		Port:           TEST_DB_PORT,
		Name:           TEST_DB_NAME,
		MigrationSteps: m,
	}
}

// Creates an MySQL test database and runs migration tests against it using the
// given migration steps. See Get for required credentials.
// The test assumes that the database is empty and that the readwrite user is
// not allowed to create/drop/alter tables.
func MySQLVersioningTests(t testutils.TestingT, dbName string, migrationSteps []database.MigrationStep) {
	// OpenDB as root user and remove all tables.
	rootConf := LocalTestRootDatabaseConfig(migrationSteps)
	lockDB := GetMySQlLock(t, rootConf)
	defer lockDB.Close(t)

	rootVdb, err := rootConf.NewVersionedDB()
	assert.NoError(t, err)
	ClearMySQLTables(t, rootVdb)
	assert.NoError(t, rootVdb.Close())

	// Configuration for the readwrite user without DDL privileges.
	readWriteConf := LocalTestDatabaseConfig(migrationSteps)

	// Open DB as readwrite user and make sure it fails because of a missing
	// version table.
	// Note: This requires the database to be empty.
	_, err = readWriteConf.NewVersionedDB()
	assert.NotNil(t, err)

	rootVdb, err = rootConf.NewVersionedDB()
	assert.NoError(t, err)
	testDBVersioning(t, rootVdb)

	// Make sure it doesn't fail for readwrite user after the migration
	_, err = readWriteConf.NewVersionedDB()
	assert.NoError(t, err)

	// Downgrade database, removing most if not all tables.
	downgradeDB(t, rootVdb)
	ClearMySQLTables(t, rootVdb)
}

type LockDB struct {
	DB *sqlx.DB
}

// Get a lock from MySQL to serialize DB tests.
func GetMySQlLock(t testutils.TestingT, conf *database.DatabaseConfig) *LockDB {
	db, err := sqlx.Open("mysql", conf.MySQLString())
	assert.NoError(t, err)

	for {
		var result int
		assert.NoError(t, db.Get(&result, "SELECT GET_LOCK(?,5)", SQL_LOCK))

		// We obtained the lock. If not try again.
		if result == 1 {
			return &LockDB{db}
		}
	}
}

// Release the MySQL lock.
func (l *LockDB) Close(t testutils.TestingT) {
	var result int
	assert.NoError(t, l.DB.Get(&result, "SELECT RELEASE_LOCK(?)", SQL_LOCK))
	assert.Equal(t, result, 1)
	assert.NoError(t, l.DB.Close())
}

// Remove all tables from the database.
func ClearMySQLTables(t testutils.TestingT, vdb *database.VersionedDB) {
	stmt := `SHOW TABLES`
	rows, err := vdb.DB.Query(stmt)
	assert.NoError(t, err)
	defer util.Close(rows)

	names := make([]string, 0)
	var tableName string
	for rows.Next() {
		assert.NoError(t, rows.Scan(&tableName))
		names = append(names, tableName)
	}

	if len(names) > 0 {
		stmt = "DROP TABLE " + strings.Join(names, ",")
		_, err = vdb.DB.Exec(stmt)
		assert.NoError(t, err)
	}
}

// MySQLTestDatabase is a convenience struct for using a test database which
// starts in a clean state.
type MySQLTestDatabase struct {
	lockDB  *LockDB
	rootVdb *database.VersionedDB
	t       testutils.TestingT
}

// SetupMySQLTestDatabase returns a MySQLTestDatabase in a clean state. It must
// be closed after use.
//
// Example usage:
//
// db := SetupMySQLTestDatabase(t, migrationSteps)
// defer util.Close(db)
// ... Tests here ...
func SetupMySQLTestDatabase(t testutils.TestingT, migrationSteps []database.MigrationStep) *MySQLTestDatabase {

	conf := LocalTestRootDatabaseConfig(migrationSteps)
	lock := GetMySQlLock(t, conf)
	rootVdb, err := conf.NewVersionedDB()
	assert.NoError(t, err)
	ClearMySQLTables(t, rootVdb)
	if err := rootVdb.Close(); err != nil {
		t.Fatal(err)
	}
	rootVdb, err = conf.NewVersionedDB()
	assert.NoError(t, err)
	if err := rootVdb.Migrate(rootVdb.MaxDBVersion()); err != nil {
		t.Fatal(err)
	}
	return &MySQLTestDatabase{lock, rootVdb, t}
}

func (d *MySQLTestDatabase) Close(t testutils.TestingT) {
	assert.NoError(t, d.rootVdb.Migrate(0))
	assert.NoError(t, d.rootVdb.Close())
	d.lockDB.Close(t)
}

// Test wether the migration steps execute correctly.
func testDBVersioning(t testutils.TestingT, vdb *database.VersionedDB) {
	// get the DB version
	dbVersion, err := vdb.DBVersion()
	assert.NoError(t, err)
	maxVersion := vdb.MaxDBVersion()

	downgradeDB(t, vdb)

	// upgrade the the latest version
	err = vdb.Migrate(maxVersion)
	assert.NoError(t, err)
	dbVersion, err = vdb.DBVersion()
	assert.NoError(t, err)
	assert.Equal(t, maxVersion, dbVersion)
}

func downgradeDB(t testutils.TestingT, vdb *database.VersionedDB) {
	// downgrade to 0
	err := vdb.Migrate(0)
	assert.NoError(t, err)
	dbVersion, err := vdb.DBVersion()
	assert.NoError(t, err)
	assert.Equal(t, 0, dbVersion)
}
