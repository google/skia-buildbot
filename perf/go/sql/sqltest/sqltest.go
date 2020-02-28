package sqltest

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/migrations"
)

// Cleanup is a function to call after the test has ended to clean up any
// database resources.
type Cleanup func()

// NewSQLite3DBForTests creates a new temporary sqlite3 database with all
// migrations applied for testing. It also returns a function to call to clean
// up the database after the tests have completed.
func NewSQLite3DBForTests(t *testing.T) (*sql.DB, Cleanup) {
	// Get a temp file to use as an sqlite3 database.
	tmpfile, err := ioutil.TempFile("", "sqlts")
	require.NoError(t, err)
	require.NoError(t, tmpfile.Close())

	cleanup := func() {
		err := os.Remove(tmpfile.Name())
		assert.NoError(t, err)
	}

	db, err := sql.Open("sqlite3", tmpfile.Name())
	require.NoError(t, err)

	migrationsDir := "../../../migrations/sqlite"
	migrationsConnection := fmt.Sprintf("sqlite3://%s", tmpfile.Name())

	err = migrations.Up(migrationsDir, migrationsConnection)
	require.NoError(t, err)

	return db, cleanup
}

// NewCockroachDBForTests creates a new temporary CockroachDB database with all
// migrations applied for testing. It also returns a function to call to clean
// up the database after the tests have completed.
//
// We pass in a database name so that different tests work in different
// databases, even though they may be in the same CockroachDB instance, so that
// if a test fails it doesn't leave the database in a bad state for a subsequent
// test.
func NewCockroachDBForTests(t *testing.T, databaseName string) (*sql.DB, Cleanup) {
	migrationsDir := "../../../migrations/cockroachdb"
	migrationsConnection := fmt.Sprintf("cockroach://root@%s/%s?sslmode=disable", perfsql.GetCockroachDBEmulatorHost(), databaseName)
	// Note that the migrationsConnection is different from the sql.Open
	// connection string since migrations know about CockroachDB, but we use the
	// Postgres driver for the database/sql connection since there's no native
	// CockroachDB golang driver, and the suggested SQL drive for CockroachDB is
	// the Postgres driver since that's the underlying communication protocol it
	// uses.
	connectionString := fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", perfsql.GetCockroachDBEmulatorHost(), databaseName)
	db, err := sql.Open("postgres", connectionString)
	require.NoError(t, err)

	// Create a database in the cockroachdb just for this test.
	_, err = db.Exec(fmt.Sprintf(`
 		CREATE DATABASE IF NOT EXISTS %s;
 		SET DATABASE = %s;`, databaseName, databaseName))
	require.NoError(t, err)

	err = migrations.Up(migrationsDir, migrationsConnection)
	require.NoError(t, err)

	cleanup := func() {
		err := migrations.Down(migrationsDir, migrationsConnection)
		assert.NoError(t, err)
		_, err = db.Exec(fmt.Sprintf("DROP DATABASE %s CASCADE;", databaseName))
		assert.NoError(t, err)
	}

	return db, cleanup
}
