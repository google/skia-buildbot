package cdbtest

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/emulators/cockroachdb_instance"
	"go.skia.org/infra/machine/go/machine/store/cdb"
)

// NewCockroachDBForTests creates a new temporary CockroachDB database with all
// tables created for testing.
//
// We pass in a database name prefix so that different tests work in different
// databases, even though they may be in the same CockroachDB instance, so that
// if a test fails it doesn't leave the database in a bad state for a subsequent
// test. A random number will be appended to the database name prefix.
func NewCockroachDBForTests(t *testing.T, databaseNamePrefix string) *pgxpool.Pool {
	cockroachdb_instance.Require(t)

	rand.Seed(time.Now().UnixNano())
	databaseName := fmt.Sprintf("%s_%d", databaseNamePrefix, rand.Uint64())
	host := emulators.GetEmulatorHostEnvVar(emulators.CockroachDB)
	connectionString := fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", host, databaseName)

	ctx := context.Background()
	db, err := pgxpool.Connect(ctx, connectionString)
	require.NoError(t, err)

	// Create a database in cockroachdb just for this test.
	_, err = db.Exec(ctx, fmt.Sprintf(`
		CREATE DATABASE %s;
		SET DATABASE = %s;`, databaseName, databaseName))
	require.NoError(t, err)

	_, err = db.Exec(ctx, cdb.Schema)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
	})
	return db
}
