package sql

// Dialect is a type for the dialect of SQL that can be used.
type Dialect string

const (
	// SQLiteDialect covers both SQLite and DQLite.
	SQLiteDialect Dialect = "sqlite"

	// CockroachDBDialect covers CockroachDB.
	CockroachDBDialect Dialect = "cockroachdb"
)
