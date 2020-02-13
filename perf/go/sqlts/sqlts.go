// Package sqlts implements a types.TraceStore on top of SQL.
package sqlts

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru"
	_ "github.com/mattn/go-sqlite3" // Get sqlite.	// Get sqlite.
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/types"
)

const cacheSize = 10 * 1000 * 1000

// Dialect is type for the dialect of SQL that can be used.
type Dialect int

const (
	// SQLiteDialect covers both SQLite and DQLite.
	SQLiteDialect = iota
)

// statement is an SQL statement or fragment of an SQL statement.
type statement int

// All the different statements we need.
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

var statementsByDialect = map[Dialect]statements{
	SQLiteDialect: statements{
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

	cache    *lru.Cache
	st       statements
	tileSize int32
}

// NewSQLite returns a new *SQLTraceStore that implements types.TraceStore on
// top of SQLite.
//
// The filename is the name of the sqlite3 database, which will be created if
// not present.
func NewSQLite(db *sql.DB, dialect Dialect, tileSize int32) (*SQLTraceStore, error) {
	_, err := db.Exec(statementsByDialect[dialect][createTables])
	if err != nil {
		return nil, err
	}
	st := statementsByDialect[dialect]
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
	cache, err := lru.New(cacheSize)
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

		st: statementsByDialect[dialect],

		cache:    cache,
		tileSize: tileSize,
	}, nil
}

// ReadTraces implements the types.TraceStore interface.
func (s *SQLTraceStore) ReadTraces(tileNumber types.TileNumber, keys []string) (map[string][]float32, error) {
	// Eventually should be broken into batches so we don't exceed the sql
	// engines limit on query sizes.
	ret := map[string][]float32{}
	for _, key := range keys {
		ret[key] = vec32.New(int(s.tileSize))
	}

	// Get the traceIDs for the given keys.
	stmt := `SELECT TraceIDs.trace_name, TraceValues.commit_number, TraceValues.val FROM TraceIDs
	INNER JOIN TraceValues ON TraceValues.trace_id = TraceIDs.trace_id
	WHERE `
	whereClauses := make([]string, len(keys))
	for i, key := range keys {
		whereClauses[i] = fmt.Sprintf(`TraceIDs.trace_name=%q`, key)
	}
	rows, err := s.db.Query(stmt + strings.Join(whereClauses, " OR "))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var traceName string
		var commitNumber int64
		var val float64
		if err := rows.Scan(&traceName, &commitNumber, &val); err != nil {
			return nil, err
		}
		ret[traceName][commitNumber%int64(s.tileSize)] = float32(val)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ret, nil
}

// TileNumber implements the types.TraceStore interface.
func (s *SQLTraceStore) TileNumber(commitNumber types.CommitNumber) types.TileNumber {
	return types.TileNumberFromCommitNumber(commitNumber, s.tileSize)
}

// TileSize implements the types.TraceStore interface.
func (s *SQLTraceStore) TileSize() int32 {
	return s.tileSize
}

// WriteIndices implements the types.TraceStore interface.
func (s *SQLTraceStore) WriteIndices(ctx context.Context) error {
	return nil
}

func (s *SQLTraceStore) updateSourceFile(source string) (int64, error) {
	if iret, ok := s.cache.Get(source); ok {
		return iret.(int64), nil
	}

	ret := int64(-1)
	res, err := s.insertIntoSourceFiles.Exec(source)
	if err != nil {
		return ret, err
	}
	sourceID, err := res.LastInsertId()
	if err != nil {
		return ret, err
	}
	s.cache.Add(source, sourceID)
	return sourceID, nil
}

func (s *SQLTraceStore) updateIndex(p paramtools.Params, tileNumber types.TileNumber, traceID int64) error {
	for k, v := range p {
		keyValue := fmt.Sprintf("%s=%s", k, v)
		_, err := s.insertIntoPostings.Exec(tileNumber, keyValue, traceID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLTraceStore) updateTraceID(traceNameAsParams paramtools.Params, tileNumber types.TileNumber) (int64, error) {
	ret := int64(-1)

	traceName, err := query.MakeKeyFast(traceNameAsParams)
	if err != nil {
		return ret, err
	}

	// Get an int64 trace id for the traceName.
	if iret, ok := s.cache.Get(traceName); ok {
		ret = iret.(int64)
	} else {
		res, err := s.insertIntoTraceIDs.Exec(traceName)
		if err != nil {
			return ret, err
		}
		ret, err = res.LastInsertId()
		if err != nil {
			return ret, err
		}
		s.cache.Add(traceName, ret)
	}

	// Update postings.
	if err := s.updateIndex(traceNameAsParams, tileNumber, ret); err != nil {
		return ret, err
	}

	return ret, nil
}

func (s *SQLTraceStore) updateTraceValues(tileNumber types.TileNumber, traceID int64, commitNumber types.CommitNumber, x float32, sourceID int64) error {
	_, err := s.replaceTraceValues.Exec(tileNumber, traceID, commitNumber, x, sourceID)
	return err
}

// WriteTraces implements the types.TraceStore interface.
func (s *SQLTraceStore) WriteTraces(commitNumber types.CommitNumber, params []paramtools.Params, values []float32, _ paramtools.ParamSet, source string, _ time.Time) error {
	tileNumber := types.TileNumberFromCommitNumber(commitNumber, s.tileSize)
	// Get the row id for the source file.
	sourceID, err := s.updateSourceFile(source)
	if err != nil {
		return err
	}

	// Get trace ids for each trace and add trace ids to the index/postings.
	traceIDs := make([]int64, len(params))
	for i, p := range params {
		traceID, err := s.updateTraceID(p, tileNumber)
		if err != nil {
			return err
		}
		traceIDs[i] = traceID
	}

	// Now add each trace value.
	for i, x := range values {
		if err := s.updateTraceValues(tileNumber, traceIDs[i], commitNumber, x, sourceID); err != nil {
			return err
		}
	}
	return nil
}
