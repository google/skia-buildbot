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
	"go.skia.org/infra/perf/go/sql/migrations/sqlite3"
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

func TestUpDown_Sqlite3(t *testing.T) {
	unittest.LargeTest(t)

	sqlite3Migrations, err := sqlite3.New()
	require.NoError(t, err)
	const sqlite3DBTest = "sqlite3:///tmp/test-sqlite3-migrations.db"

	err = Up(sqlite3Migrations, sqlite3DBTest)
	assert.NoError(t, err)
	err = Down(sqlite3Migrations, sqlite3DBTest)
	assert.NoError(t, err)

	// Do it a second time to ensure we are idempotent.
	err = Up(sqlite3Migrations, sqlite3DBTest)
	assert.NoError(t, err)
	err = Down(sqlite3Migrations, sqlite3DBTest)
	assert.NoError(t, err)
}
