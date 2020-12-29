package sqltest

import (
	"context"
	"fmt"
	"math/rand"
	"os/exec"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/sql"
)

// NewCockroachDBForTests creates a randomly named database on the presumed to be running
// cockroachDB instance as configured by the COCKROACHDB_EMULATOR_HOST environment variable.
func NewCockroachDBForTests(ctx context.Context, t *testing.T) *pgxpool.Pool {
	unittest.RequiresCockroachDB(t)
	out, err := exec.Command("cockroach", "version").CombinedOutput()
	require.NoError(t, err, "Do you have 'cockroach' on your path? %s", out)

	dbName := "for_tests" + strconv.Itoa(rand.Int())
	port := sql.GetCockroachDBEmulatorHost()

	out, err = exec.Command("cockroach", "sql", "--insecure", "--host="+port,
		"--execute=CREATE DATABASE IF NOT EXISTS "+dbName).CombinedOutput()
	require.NoError(t, err, `creating test database: %s
If running locally, make sure you set the env var TMPDIR and ran:
./scripts/run_emulators/run_emulators start
and now currently have COCKROACHDB_EMULATOR_HOST set. Even though we call it an "emulator",
this sets up a real version of cockroachdb.
`, out)

	connectionString := fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", port, dbName)
	conn, err := pgxpool.Connect(ctx, connectionString)
	require.NoError(t, err)
	t.Cleanup(func() {
		conn.Close()
	})
	return conn
}
