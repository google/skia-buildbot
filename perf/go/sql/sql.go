package sql

import "os"

// Dialect is a type for the dialect of SQL that can be used. Make sure that the
// names of each dialect match the name of their corresponding sub-directory of
// /infra/perf/migrations.
type Dialect string

const (
	// SQLiteDialect covers both SQLite and DQLite.
	SQLiteDialect Dialect = "sqlite3"

	// CockroachDBDialect covers CockroachDB.
	CockroachDBDialect Dialect = "cockroachdb"
)

// COCKROACHDB_EMULATOR_HOST_ENV_VAR is the name of the environment variable
// that points to a test instance of CockroachDB.
const COCKROACHDB_EMULATOR_HOST_ENV_VAR = "COCKROACHDB_EMULATOR_HOST"

// GetCockroachDBEmulatorHost returns the connection string to connect to a
// local test instance of CockroachDB.
func GetCockroachDBEmulatorHost() string {
	ret := os.Getenv(COCKROACHDB_EMULATOR_HOST_ENV_VAR)
	if ret == "" {
		ret = "localhost:26257"
	}
	return ret
}
