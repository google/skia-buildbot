package migrations

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/emulators/cockroachdb_instance"
	"go.skia.org/infra/perf/go/sql/migrations/cockroachdb"
)

func TestUpDown_CockroachDB(t *testing.T) {
	cockroachdb_instance.Require(t)

	cockroachMigrations, err := cockroachdb.New()
	require.NoError(t, err)

	cockroachDBTest := fmt.Sprintf("cockroach://root@%s?sslmode=disable", emulators.GetEmulatorHostEnvVar(emulators.CockroachDB))

	err = Up(cockroachMigrations, cockroachDBTest)
	assert.NoError(t, err)
	err = Down(cockroachMigrations, cockroachDBTest)
	assert.NoError(t, err)

	// Do it a second time to ensure we are idempotent.
	err = Up(cockroachMigrations, cockroachDBTest)
	assert.NoError(t, err)
	err = Down(cockroachMigrations, cockroachDBTest)
	assert.NoError(t, err)

}
