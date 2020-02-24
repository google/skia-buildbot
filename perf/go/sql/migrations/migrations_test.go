package migrations

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func getEmulatorHost() string {
	ret := os.Getenv("COCKROACHDB_EMULATOR_HOST")
	if ret == "" {
		ret = "localhost:26257"
	}
	return ret
}

func TestUpDownCockroachDB(t *testing.T) {
	unittest.LargeTest(t)

	const cockroachMigrations = "../../../migrations/cockroachdb"

	cockroachDBTest := fmt.Sprintf("cockroach://root@%s?sslmode=disable", getEmulatorHost())

	err := Up(cockroachMigrations, cockroachDBTest)
	assert.NoError(t, err)
	err = Down(cockroachMigrations, cockroachDBTest)
	assert.NoError(t, err)
}

func TestUpDownSqlite3(t *testing.T) {
	unittest.LargeTest(t)
	const sqlite3Migrations = "../../../migrations/sqlite"
	const sqlite3DBTest = "sqlite3:///tmp/test.db"

	err := Up(sqlite3Migrations, sqlite3DBTest)
	assert.NoError(t, err)
	err = Down(sqlite3Migrations, sqlite3DBTest)
	assert.NoError(t, err)
}
