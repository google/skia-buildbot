package sqltest

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/migrations"
	"go.skia.org/infra/perf/go/sql/migrations/cockroachdb"
)

// Cleanup is a function to call after the test has ended to clean up any
// database resources.
type Cleanup func()

// NewCockroachDBForTests creates a new temporary CockroachDB database with all
// migrations applied for testing. It also returns a function to call to clean
// up the database after the tests have completed.
//
// We pass in a database name so that different tests work in different
// databases, even though they may be in the same CockroachDB instance, so that
// if a test fails it doesn't leave the database in a bad state for a subsequent
// test.
//
// If migrations to are be applied then set applyMigrations to true.
func NewCockroachDBForTests(t *testing.T, databaseName string) (*pgxpool.Pool, Cleanup) {
	unittest.RequiresCockroachDB(t)

	// Note that the migrationsConnection is different from the sql.Open
	// connection string since migrations know about CockroachDB, but we use the
	// Postgres driver for the database/sql connection since there's no native
	// CockroachDB golang driver, and the suggested SQL drive for CockroachDB is
	// the Postgres driver since that's the underlying communication protocol it
	// uses.
	migrationsConnection := fmt.Sprintf("cockroach://root@%s/%s?sslmode=disable", perfsql.GetCockroachDBEmulatorHost(), databaseName)
	connectionString := fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", perfsql.GetCockroachDBEmulatorHost(), databaseName)
	db, err := sql.Open("postgres", connectionString)
	require.NoError(t, err)

	// Create a database in cockroachdb just for this test.
	_, err = db.Exec(fmt.Sprintf(`
 		CREATE DATABASE IF NOT EXISTS %s;
 		SET DATABASE = %s;`, databaseName, databaseName))
	require.NoError(t, err)

	cockroachdbMigrations, err := cockroachdb.New()
	require.NoError(t, err)

	err = migrations.Up(cockroachdbMigrations, migrationsConnection)
	require.NoError(t, err)

	ctx := context.Background()
	conn, err := pgxpool.Connect(ctx, connectionString)
	require.NoError(t, err)

	cleanup := func() {
		// Don't bother applying migrations.Down since we aren't testing
		// migrations here and it just slows down the tests.
		_, err = db.Exec(fmt.Sprintf("DROP DATABASE %s CASCADE;", databaseName))
		assert.NoError(t, err)
		require.NoError(t, db.Close())
		conn.Close()
	}
	return conn, cleanup
}
