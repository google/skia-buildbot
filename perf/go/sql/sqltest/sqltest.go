package sqltest

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/emulators/cockroachdb_instance"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/pool/wrapper/timeout"
	"go.skia.org/infra/perf/go/sql"
)

// NewCockroachDBForTests creates a new temporary CockroachDB database with all
// the Schema applied for testing. It also returns a function to call to clean
// up the database after the tests have completed.
//
// We pass in a database name prefix so that different tests work in different
// databases, even though they may be in the same CockroachDB instance, so that
// if a test fails it doesn't leave the database in a bad state for a subsequent
// test. A random number will be appended to the database name prefix.
func NewCockroachDBForTests(t *testing.T, databaseNamePrefix string) pool.Pool {
	cockroachdb_instance.Require(t)

	databaseName := fmt.Sprintf("%s_%d", databaseNamePrefix, rand.Uint64())

	host := emulators.GetEmulatorHostEnvVar(emulators.CockroachDB)
	connectionString := fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", host, databaseName)

	ctx := context.Background()
	rawConn, err := pgxpool.Connect(ctx, connectionString)
	require.NoError(t, err)

	// Wrap the db pool in a ContextTimeout which checks that every context has
	// a timeout.
	conn := timeout.New(rawConn)

	// Create a database in cockroachdb just for this test.
	_, err = conn.Exec(ctx, fmt.Sprintf(`
		CREATE DATABASE %s;
		SET DATABASE = %s;`, databaseName, databaseName))
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, err := conn.Exec(ctx, sql.Schema)
		if err != nil {
			fmt.Printf("Error while applying database migration: %v", err)
		}
		return err == nil
	}, 10*time.Second, 1*time.Second)

	t.Cleanup(func() {
		_, err = conn.Exec(ctx, fmt.Sprintf("DROP DATABASE %s CASCADE;", databaseName))
		assert.NoError(t, err)
		conn.Close()
	})
	return conn
}
