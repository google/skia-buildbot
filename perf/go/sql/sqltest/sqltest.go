package sqltest

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/emulators/cockroachdb_instance"
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
// We pass in a database name prefix so that different tests work in different
// databases, even though they may be in the same CockroachDB instance, so that
// if a test fails it doesn't leave the database in a bad state for a subsequent
// test. A random number will be appended to the database name prefix.
//
// If migrations to are be applied then set applyMigrations to true.
func NewCockroachDBForTests(t *testing.T, databaseNamePrefix string) (*pgxpool.Pool, Cleanup) {
	cockroachdb_instance.Require(t)

	rand.Seed(time.Now().UnixNano())
	databaseName := fmt.Sprintf("%s_%d", databaseNamePrefix, rand.Uint64())

	// Note that the migrationsConnection is different from the sql.Open
	// connection string since migrations know about CockroachDB, but we use the
	// Postgres driver for the database/sql connection since there's no native
	// CockroachDB golang driver, and the suggested SQL drive for CockroachDB is
	// the Postgres driver since that's the underlying communication protocol it
	// uses.
	host := emulators.GetEmulatorHostEnvVar(emulators.CockroachDB)
	migrationsConnection := fmt.Sprintf("cockroach://root@%s/%s?sslmode=disable", host, databaseName)
	connectionString := fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", host, databaseName)
	db, err := sql.Open("postgres", connectionString)
	require.NoError(t, err)

	// Create a database in cockroachdb just for this test.
	_, err = db.Exec(fmt.Sprintf(`
		CREATE DATABASE %s;
		SET DATABASE = %s;`, databaseName, databaseName))
	require.NoError(t, err)

	cockroachdbMigrations, err := cockroachdb.New()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		err = migrations.Up(cockroachdbMigrations, migrationsConnection)
		if err != nil {
			fmt.Printf("Error while applying database migration: %v", err)
		}
		return err == nil
	}, 10*time.Second, 1*time.Second)

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
