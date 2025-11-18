package sqltest

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/emulators"

	"go.skia.org/infra/go/emulators/gcp_emulator"
	"go.skia.org/infra/go/emulators/pgadapter"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/pool/wrapper/timeout"
	"go.skia.org/infra/perf/go/sql/spanner"
)

// NewSpannerDBForTests returns a connection to a local spanner emulator database to
// be used for executing unit tests.
func NewSpannerDBForTests(t *testing.T, databaseNamePrefix string) pool.Pool {
	// Ensure that both spanner emulator and pgadapter are running first.
	gcp_emulator.RequireSpanner(t)
	pgadapter.Require(t)

	databaseName := fmt.Sprintf("%s_%d", databaseNamePrefix, rand.Int())

	if len(databaseName) > 30 {
		databaseName = databaseName[:30]
	}
	host := emulators.GetEmulatorHostEnvVar(emulators.PGAdapter)
	connectionString := fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", host, databaseName)

	ctx := context.Background()
	rawConn, err := pgxpool.Connect(ctx, connectionString)
	require.NoError(t, err)

	// Apply the db schema so that the tables are ready for the tests.
	require.Eventually(t, func() bool {
		_, err := rawConn.Exec(ctx, spanner.Schema)
		if err != nil {
			fmt.Printf("Error while applying database migration: %v", err)
		}
		return err == nil
	}, 10*time.Second, 1*time.Second)
	// Wrap the db pool in a ContextTimeout which checks that every context has
	// a timeout.
	conn := timeout.New(rawConn)

	return conn
}
