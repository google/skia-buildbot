package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestUpDownCockroachDB(t *testing.T) {
	unittest.MediumTest(t)

	const cockroachMigrations = "../../../migrations/cockroachdb"
	const cockroachDBTest = "cockroach://root@localhost:26257?sslmode=disable"

	err := Up(cockroachMigrations, cockroachDBTest)
	assert.NoError(t, err)
	err = Down(cockroachMigrations, cockroachDBTest)
	assert.NoError(t, err)
}

func TestUpDownSqlite3(t *testing.T) {
	unittest.MediumTest(t)
	const sqlite3Migrations = "../../../migrations/sqlite"
	const sqlite3DBTest = "sqlite3:///tmp/test.db"

	err := Up(sqlite3Migrations, sqlite3DBTest)
	assert.NoError(t, err)
	err = Down(sqlite3Migrations, sqlite3DBTest)
	assert.NoError(t, err)
}
