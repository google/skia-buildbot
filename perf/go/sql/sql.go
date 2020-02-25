package sql

// Dialect is a type for the dialect of SQL that can be used. Make sure that the
// names of each dialect match the name of their corresponding sub-directory of
// /infra/perf/migrations.
type Dialect string

const (
	// SQLiteDialect covers both SQLite and DQLite.
	SQLiteDialect Dialect = "sqlite"

	// CockroachDBDialect covers CockroachDB.
	CockroachDBDialect Dialect = "cockroachdb"
)
