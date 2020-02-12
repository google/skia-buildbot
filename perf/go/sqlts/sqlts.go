// Package sqlts implements a types.TraceStore on top of SQL.
package sqlts

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3" // Get sqlite.
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/perf/go/types"
)

// flavor is type for the flavor of SQL that can be used. We expect to support
// this on SQLite, DQLite, and CockroachDB, so we will need to support flavors
// of SQL.
type flavor int

const (
	sqliteFlavor = iota // Covers both SQLite and DQLite.
	cockroachFlavor
)

// statement is an SQL statement or fragment of an SQL statement.
type statement int

const (
	createTables = iota
	insertIntoSourceFiles
	insertIntoTraceKeys
)

type statements map[statement]string

var statementsByFlavor = map[flavor]statements{
	sqliteFlavor: statements{
		createTables: `
		CREATE TABLE IF NOT EXISTS TraceKeys  (
			trace_id INTEGER PRIMARY KEY,
			trace_key TEXT UNIQUE NOT NULL
		);

		CREATE TABLE IF NOT EXISTS Postings  (
			tile_key INTEGER,
			key_value text NOT NULL,
			trace_id INTEGER,
			PRIMARY KEY (tile_key, key_value, trace_id)
		);

		CREATE TABLE IF NOT EXISTS SourceFiles (
			source_file_id INTEGER PRIMARY KEY,
			source_file TEXT UNIQUE NOT NULL
		);

		CREATE TABLE IF NOT EXISTS TraceValues (
			tile_key INTEGER,
			trace_id INTEGER,
			commit_number INTEGER,
			val REAL,
			source_file_id INTEGER,
			PRIMARY KEY (tile_key, trace_id, commit_number)
		);
		`,
		insertIntoSourceFiles: `
		INSERT INTO SourceFiles (source_file)
		VALUES
		`,
		insertIntoTraceKeys: `
		INSERT INTO TraceKeys (trace_key)
		VALUES
		`,
	},
}

// SQLTraceStore implements types.TraceStore backed onto an SQL database.
type SQLTraceStore struct {
	db *sql.DB
}

// NewSQLite returns a new *SQLTraceStore that implements types.TraceStore on
// top of SQLite.
//
// The filename is the name of the sqlite3 database, which will be created if
// not present.
func NewSQLite(filename string) (*SQLTraceStore, error) {
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(statementsByFlavor[sqliteFlavor][createTables])
	if err != nil {
		return nil, err
	}
	return &SQLTraceStore{
		db: db,
	}, nil
}

// WriteTraces implements the types.TraceStore interface.
func (sqlts SQLTraceStore) WriteTraces(commitNumber types.CommitNumber, params []paramtools.Params, values []float32, paramset paramtools.ParamSet, source string, timestamp time.Time) error {
	return nil
}
