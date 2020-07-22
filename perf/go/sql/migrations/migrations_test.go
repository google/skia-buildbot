package migrations

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/migrations/cockroachdb"
)

func getEmulatorHost() string {
	ret := os.Getenv("COCKROACHDB_EMULATOR_HOST")
	if ret == "" {
		ret = "localhost:26257"
	}
	return ret
}

func TestUpDown_CockroachDB(t *testing.T) {
	unittest.LargeTest(t)

	cockroachMigrations, err := cockroachdb.New()
	require.NoError(t, err)

	cockroachDBTest := fmt.Sprintf("cockroach://root@%s?sslmode=disable", perfsql.GetCockroachDBEmulatorHost())

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
