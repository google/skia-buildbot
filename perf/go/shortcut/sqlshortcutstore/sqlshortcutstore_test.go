package sqlshortcutstore

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/shortcut/shortcuttest"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/migrations"
)

func TestInsertGet_SQLite(t *testing.T) {
	unittest.LargeTest(t)

	// Get a temp file to use as an sqlite3 database.
	tmpfile, err := ioutil.TempFile("", "sqlts")
	assert.NoError(t, err)
	err = tmpfile.Close()
	assert.NoError(t, err)

	defer func() {
		err := os.Remove(tmpfile.Name())
		assert.NoError(t, err)
	}()

	db, err := sql.Open("sqlite3", tmpfile.Name())
	assert.NoError(t, err)

	migrationsDir := "../../../migrations/sqlite"
	migrationsConnection := fmt.Sprintf("sqlite3://%s", tmpfile.Name())

	err = migrations.Up(migrationsDir, migrationsConnection)
	assert.NoError(t, err)

	store, err := New(db, perfsql.SQLiteDialect)
	require.NoError(t, err)

	shortcuttest.TestInsertGet(store, t)
}

func TestInsertGet_CockroachDB(t *testing.T) {
	unittest.LargeTest(t)

	migrationsDir := "../../../migrations/cockroachdb"
	migrationsConnection := fmt.Sprintf("cockroach://root@%s/shortcutstore?sslmode=disable", perfsql.GetCockroachDBEmulatorHost())
	// Note that the migrationsConnection is different from the sql.Open
	// connection string since migrations know about CockroachDB, but we use the
	// Postgres driver for the database/sql connection.
	connectionString := fmt.Sprintf("postgresql://root@%s/shortcutstore?sslmode=disable", perfsql.GetCockroachDBEmulatorHost())
	db, err := sql.Open("postgres", connectionString)
	assert.NoError(t, err)

	// Create a database in the cockroachdb just for this test.
	_, err = db.Exec(`
 		CREATE DATABASE IF NOT EXISTS shortcutstore;
 		SET DATABASE = shortcutstore;`)
	assert.NoError(t, err)

	err = migrations.Up(migrationsDir, migrationsConnection)
	assert.NoError(t, err)

	store, err := New(db, perfsql.CockroachDBDialect)
	require.NoError(t, err)

	defer func() {
		err := migrations.Down(migrationsDir, migrationsConnection)
		assert.NoError(t, err)
		_, err = db.Exec("DROP DATABASE shortcutstore CASCADE;")
		assert.NoError(t, err)
	}()

	shortcuttest.TestInsertGet(store, t)
}
