// Package sqlts implements a types.TraceStore on top of SQL.
package sqlts

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3" // Get sqlite.	// Get sqlite.
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/types"
)

// dialect is type for the dialect of SQL that can be used. We expect to support
// this on SQLite, DQLite, and CockroachDB, so we will need to support flavors
// of SQL.
type dialect int

const (
	sqliteDialect = iota // Covers both SQLite and DQLite.
	cockroachDialect
)

// statement is an SQL statement or fragment of an SQL statement.
type statement int

const (
	createTables = iota

	insertIntoSourceFiles
	getSourceFileID

	insertIntoTraceIDs
	getTraceID

	insertIntoPostings

	replaceTraceValues
)

type statements map[statement]string

var statementsByDialect = map[dialect]statements{
	sqliteDialect: statements{
		createTables: `
		CREATE TABLE IF NOT EXISTS TraceIDs  (
			trace_id INTEGER PRIMARY KEY,
			trace_name TEXT UNIQUE NOT NULL
		);

		CREATE TABLE IF NOT EXISTS Postings  (
			tile_number INTEGER,
			key_value text NOT NULL,
			trace_id INTEGER,
			PRIMARY KEY (tile_number, key_value, trace_id)
		);

		CREATE TABLE IF NOT EXISTS SourceFiles (
			source_file_id INTEGER PRIMARY KEY,
			source_file TEXT UNIQUE NOT NULL
		);

		CREATE TABLE IF NOT EXISTS TraceValues (
			tile_number INTEGER,
			trace_id INTEGER,
			commit_number INTEGER,
			val REAL,
			source_file_id INTEGER,
			PRIMARY KEY (tile_number, trace_id, commit_number)
		);
		`,

		insertIntoSourceFiles: `INSERT OR IGNORE INTO SourceFiles (source_file) VALUES (?);`,
		getSourceFileID:       `SELECT source_file_id FROM SourceFiles WHERE source_file=?`,

		insertIntoTraceIDs: `INSERT OR IGNORE INTO TraceIDs (trace_name) VALUES (?)`,
		getTraceID:         `SELECT trace_id FROM TraceIDs WHERE trace_name=?`,

		insertIntoPostings: `INSERT OR IGNORE INTO Postings (tile_number, key_value, trace_id) VALUES (?, ?, ?)`,

		replaceTraceValues: `INSERT OR REPLACE INTO TraceValues (tile_number, trace_id, commit_number, val, source_file_id) VALUES( ?, ?, ?, ?, ?)`,
	},
}

// SQLTraceStore implements types.TraceStore backed onto an SQL database.
type SQLTraceStore struct {
	db                    *sql.DB
	insertIntoSourceFiles *sql.Stmt
	getSourceFileID       *sql.Stmt

	insertIntoTraceIDs *sql.Stmt
	getTraceID         *sql.Stmt

	insertIntoPostings *sql.Stmt

	replaceTraceValues *sql.Stmt

	st       statements
	tileSize int32
}

// NewSQLite returns a new *SQLTraceStore that implements types.TraceStore on
// top of SQLite.
//
// The filename is the name of the sqlite3 database, which will be created if
// not present.
func NewSQLite(filename string, tileSize int32) (*SQLTraceStore, error) {
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(statementsByDialect[sqliteDialect][createTables])
	if err != nil {
		return nil, err
	}
	st := statementsByDialect[sqliteDialect]
	insertIntoSourceFilesStmt, err := db.Prepare(st[insertIntoSourceFiles])
	if err != nil {
		return nil, err
	}
	getSourceFileIDStmt, err := db.Prepare(st[getSourceFileID])
	if err != nil {
		return nil, err
	}
	insertIntoTraceIDsStmt, err := db.Prepare(st[insertIntoTraceIDs])
	if err != nil {
		return nil, err
	}
	getTraceIDStmt, err := db.Prepare(st[getTraceID])
	if err != nil {
		return nil, err
	}
	insertIntoPostingsStmt, err := db.Prepare(st[insertIntoPostings])
	if err != nil {
		return nil, err
	}
	replaceTraceValuesStmt, err := db.Prepare(st[replaceTraceValues])
	if err != nil {
		return nil, err
	}

	return &SQLTraceStore{
		db: db,

		insertIntoSourceFiles: insertIntoSourceFilesStmt,
		getSourceFileID:       getSourceFileIDStmt,

		insertIntoTraceIDs: insertIntoTraceIDsStmt,
		getTraceID:         getTraceIDStmt,

		insertIntoPostings: insertIntoPostingsStmt,

		replaceTraceValues: replaceTraceValuesStmt,

		st: statementsByDialect[sqliteDialect],

		tileSize: tileSize,
	}, nil
}

func (s *SQLTraceStore) updateSourceFile(source string) (int64, error) {
	// TODO(jcgregorio) Add a cache here.
	ret := int64(-1)
	res, err := s.insertIntoSourceFiles.Exec(source)
	if err != nil {
		return ret, err
	}
	return res.LastInsertId()
}

func (s *SQLTraceStore) updateIndex(p paramtools.Params, tileNumber types.TileNumber, traceID int64) error {
	// TODO(jcgregorio) Add a cache here.
	for k, v := range p {
		keyValue := fmt.Sprintf("%s=%s", k, v)
		_, err := s.insertIntoPostings.Exec(tileNumber, keyValue, traceID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLTraceStore) updateTraceID(p paramtools.Params) (int64, error) {
	// TODO(jcgregorio) Add a cache here.
	ret := int64(-1)
	traceID, err := query.MakeKeyFast(p)
	if err != nil {
		return ret, err
	}
	res, err := s.insertIntoTraceIDs.Exec(traceID)
	if err != nil {
		return ret, err
	}
	return res.LastInsertId()
}

func (s *SQLTraceStore) updateTraceValues(tileNumber types.TileNumber, traceID int64, commitNumber types.CommitNumber, x float32, sourceID int64) error {
	_, err := s.replaceTraceValues.Exec(tileNumber, traceID, commitNumber, x, sourceID)
	return err
}

// WriteTraces implements the types.TraceStore interface.
func (s *SQLTraceStore) WriteTraces(commitNumber types.CommitNumber, params []paramtools.Params, values []float32, paramset paramtools.ParamSet, source string, timestamp time.Time) error {
	tileNumber := types.TileNumberFromCommitNumber(commitNumber, s.tileSize)
	// Get the row id for the source file.
	sourceID, err := s.updateSourceFile(source)
	if err != nil {
		return err
	}

	// Get trace ids for each trace and add trace ids to the index/postings.
	traceIDs := make([]int64, len(params))
	for i, p := range params {
		traceID, err := s.updateTraceID(p)
		if err != nil {
			return err
		}
		traceIDs[i] = traceID
		if err := s.updateIndex(p, tileNumber, traceID); err != nil {
			return err
		}
	}

	// Now add each trace value.
	for i, x := range values {
		if err := s.updateTraceValues(tileNumber, traceIDs[i], commitNumber, x, sourceID); err != nil {
			return err
		}
	}
	return nil
}
