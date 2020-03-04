// Package sqlts implements a tracestore.TraceStore on top of SQL.
package sqlts

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru"
	_ "github.com/lib/pq"           // Get postgres database/sql drivers.
	_ "github.com/mattn/go-sqlite3" // Get sqlite database/sql drivers.
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/vec32"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracestore/sqltracestore/engine"
	"go.skia.org/infra/perf/go/types"
)

const cacheSize = 10 * 1000 * 1000

// statement is an SQL statement or fragment of an SQL statement.
type statement int

// All the different statements we need.
const (
	insertIntoSourceFiles statement = iota
	getSourceFileID
	insertIntoTraceIDs
	getTraceID
	insertIntoPostings
	replaceTraceValues
	countIndices
	getLatestTile
	paramSetForTile
	getSource
	traceCount
)

type statements map[statement]string

var statementsByDialect = map[perfsql.Dialect]statements{
	perfsql.SQLiteDialect: {
		insertIntoSourceFiles: `
		INSERT OR IGNORE INTO
			SourceFiles (source_file)
		VALUES
			(?)`,
		getSourceFileID: `
		SELECT
			source_file_id
		FROM
			SourceFiles
		WHERE
			source_file=?`,
		insertIntoTraceIDs: `
		INSERT OR IGNORE INTO
			TraceIDs (trace_name)
		VALUES
			(?)`,
		getTraceID: `
		SELECT
			trace_id
		FROM
			TraceIDs
		WHERE
			trace_name=?`,
		insertIntoPostings: `
		INSERT OR IGNORE INTO
			Postings (tile_number, key_value, trace_id)
		VALUES
			(?, ?, ?)`,
		replaceTraceValues: `
		INSERT OR REPLACE INTO
			TraceValues (trace_id, commit_number, val, source_file_id)
		VALUES
			(?, ?, ?, ?)`,
		countIndices: `
		SELECT
			COUNT(*)
		FROM
			Postings
		WHERE
			tile_number=?`,
		getLatestTile: `
		SELECT
			tile_number
		FROM
			Postings
		ORDER BY
			tile_number DESC
		LIMIT 1`,
		paramSetForTile: `
		SELECT DISTINCT
			key_value
		FROM
			Postings
		WHERE
			tile_number=?`,
		getSource: `
		SELECT
			SourceFiles.source_file
		FROM
			TraceIDs
		INNER JOIN
			TraceValues ON TraceValues.trace_id = TraceIDs.trace_id
		INNER JOIN
			SourceFiles ON SourceFiles.source_file_id = TraceValues.source_file_id
		WHERE
			TraceIDs.trace_name=? AND TraceValues.commit_number=?`,
		traceCount: `
		SELECT
			COUNT(DISTINCT trace_id)
		FROM
			TraceValues
		WHERE
		  commit_number >= ? AND commit_number <= ?`,
	},
	perfsql.CockroachDBDialect: {
		insertIntoSourceFiles: `
		INSERT INTO
			SourceFiles (source_file)
		VALUES
			($1)
		ON CONFLICT
		DO NOTHING`,
		getSourceFileID: `
		SELECT
			source_file_id
		FROM
			SourceFiles
		WHERE
			source_file=$1`,
		insertIntoTraceIDs: `
		INSERT INTO
			TraceIDs (trace_name)
		VALUES
			($1)
		ON CONFLICT
		DO NOTHING`,
		getTraceID: `
		SELECT
			trace_id
		FROM
			TraceIDs
		WHERE
			trace_name=$1`,
		insertIntoPostings: `
		INSERT INTO
			Postings (tile_number, key_value, trace_id)
		VALUES
			($1, $2, $3)
		ON CONFLICT
		DO NOTHING`,
		replaceTraceValues: `
		UPSERT INTO
			TraceValues (trace_id, commit_number, val, source_file_id)
		VALUES
			($1, $2, $3, $4)`,
		countIndices: `
		SELECT
			COUNT(*)
		FROM
			Postings
		WHERE
			tile_number=$1`,
		getLatestTile: `
		SELECT
			tile_number
		FROM
			Postings
		ORDER BY
			tile_number DESC
		LIMIT 1`,
		paramSetForTile: `
		SELECT DISTINCT
			key_value
		FROM
			Postings
		WHERE
			tile_number=$1`,
		getSource: `
		SELECT
			SourceFiles.source_file
		FROM
			TraceIDs
		INNER JOIN
			TraceValues ON TraceValues.trace_id = TraceIDs.trace_id
		INNER JOIN
			SourceFiles ON SourceFiles.source_file_id = TraceValues.source_file_id
		WHERE
			TraceIDs.trace_name=$1 AND TraceValues.commit_number=$2`,
		traceCount: `
		SELECT
			COUNT(DISTINCT trace_id)
		FROM
			TraceValues
		WHERE
		  commit_number >= $1 AND commit_number <= $2`,
	},
}

// SQLTraceStore implements tracestore.TraceStore backed onto an SQL database.
type SQLTraceStore struct {
	db                 *sql.DB
	preparedStatements map[statement]*sql.Stmt
	cache              *lru.Cache
	st                 statements
	tileSize           int32
}

// New returns a new *SQLTraceStore that implements tracestore.TraceStore on top of
// SQLite.
//
// The filename is the name of the sqlite3 database, which will be created if
// not present.
//
// Presumes that perf/sql/migrations.Up() has already been called on the
// database so that the schema is up to date.
func New(db *sql.DB, dialect perfsql.Dialect, tileSize int32) (*SQLTraceStore, error) {
	preparedStatements := map[statement]*sql.Stmt{}
	for key, statement := range statementsByDialect[dialect] {
		prepared, err := db.Prepare(statement)
		if err != nil {
			return nil, fmt.Errorf("Failed to prepare statment %v %q: %s", key, statement, err)
		}
		preparedStatements[key] = prepared
	}

	cache, err := lru.New(cacheSize)
	if err != nil {
		return nil, err
	}

	return &SQLTraceStore{
		db:                 db,
		preparedStatements: preparedStatements,
		cache:              cache,
		tileSize:           tileSize,
	}, nil
}

// CommitNumberOfTileStart implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) CommitNumberOfTileStart(commitNumber types.CommitNumber) types.CommitNumber {
	tileNumber := types.TileNumberFromCommitNumber(commitNumber, s.tileSize)
	beginCommit, _ := types.TileCommitRangeForTileNumber(tileNumber, s.tileSize)
	return beginCommit
}

// CountIndices implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) CountIndices(ctx context.Context, tileNumber types.TileNumber) (int64, error) {
	var ret int64
	if err := s.preparedStatements[countIndices].QueryRow(tileNumber).Scan(&ret); err != nil {
		return 0, err
	}
	return ret, nil
}

// GetLatestTile implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) GetLatestTile() (types.TileNumber, error) {
	var tileNumber int64
	if err := s.preparedStatements[getLatestTile].QueryRow().Scan(&tileNumber); err != nil {
		return types.BadTileNumber, err
	}
	return types.TileNumber(tileNumber), nil
}

func (s *SQLTraceStore) paramSetForTile(tileNumber types.TileNumber) (paramtools.ParamSet, error) {
	rows, err := s.preparedStatements[paramSetForTile].Query(tileNumber)
	if err != nil {
		return nil, err
	}
	ret := paramtools.NewParamSet()
	for rows.Next() {
		var keyValue string
		if err := rows.Scan(&keyValue); err != nil {
			return nil, err
		}
		parts := strings.Split(keyValue, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("Invalid key=value form: %q", keyValue)
		}
		ret.AddParams(paramtools.Params{parts[0]: parts[1]})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ret.Normalize()
	return ret, nil
}

// GetOrderedParamSet implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) GetOrderedParamSet(ctx context.Context, tileNumber types.TileNumber) (*paramtools.OrderedParamSet, error) {
	ps, err := s.paramSetForTile(tileNumber)
	if err != nil {
		return nil, err
	}
	ret := paramtools.NewOrderedParamSet()
	ret.Update(ps)
	sort.Strings(ret.KeyOrder)
	return ret, nil
}

// GetSource implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) GetSource(ctx context.Context, commitNumber types.CommitNumber, traceName string) (string, error) {
	var filename string
	if err := s.preparedStatements[getSource].QueryRow(traceName, commitNumber).Scan(&filename); err != nil {
		return "", err
	}
	return filename, nil
}

// OffsetFromIndex implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) OffsetFromIndex(commitNumber types.CommitNumber) int32 {
	return int32(commitNumber) % s.tileSize
}

// QueryTracesByIndex implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) QueryTracesByIndex(ctx context.Context, tileNumber types.TileNumber, q *query.Query) (types.TraceSet, error) {
	ops, err := s.GetOrderedParamSet(ctx, tileNumber)
	if err != nil {
		return nil, fmt.Errorf("Failed to get OPS: %s", err)
	}
	plan, err := q.QueryPlan(ops)
	if err != nil {
		// Not an error, we just won't match anything in this tile.
		//
		// The plan may be invalid because it is querying with keys or values
		// that don't appear in a tile, which means they query won't work on
		// this tile, but it may still work on other tiles, so we just don't
		// return any results for this tile.
		return nil, nil
	}
	if len(plan) == 0 {
		// We won't match anything in this tile.
		return nil, nil
	}

	beginCommit, endCommit := types.TileCommitRangeForTileNumber(tileNumber, s.tileSize)

	stmt := fmt.Sprintf(`
SELECT
	TraceIDs.trace_name, TraceValues.commit_number, TraceValues.val
FROM
	TraceIDs
INNER JOIN
	TraceValues
ON
	TraceValues.trace_id = TraceIDs.trace_id
WHERE
	TraceValues.commit_number>=%d
	AND TraceValues.commit_number<=%d
	AND `, beginCommit, endCommit)

	whereClauses := make([]string, len(plan))
	clauseIndex := 0
	for key, values := range plan {
		inClause := make([]string, len(values))
		for i, value := range values {
			inClause[i] = fmt.Sprintf("'%s=%s'", key, value)
		}
		whereClauses[clauseIndex] = `
	TraceValues.trace_id IN (
		SELECT trace_id FROM Postings WHERE key_value IN (` +
			strings.Join(inClause, ",") +
			`)
		)`
		clauseIndex++
	}
	fullStatement := stmt + strings.Join(whereClauses, " AND ")

	rows, err := s.db.Query(fullStatement)
	if err != nil {
		return nil, err
	}

	ret := types.TraceSet{}

	for rows.Next() {
		var traceName string
		var commitNumber int64
		var val float64
		if err := rows.Scan(&traceName, &commitNumber, &val); err != nil {
			return nil, err
		}
		if _, ok := ret[traceName]; ok {
			ret[traceName][commitNumber%int64(s.tileSize)] = float32(val)
		} else {
			ret[traceName] = vec32.New(int(s.tileSize))
			ret[traceName][commitNumber%int64(s.tileSize)] = float32(val)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ret, nil
}

// QueryTracesIDOnlyByIndex implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) QueryTracesIDOnlyByIndex(ctx context.Context, tileNumber types.TileNumber, q *query.Query) (<-chan paramtools.Params, error) {
	if q.Empty() {
		return nil, fmt.Errorf("Can't run QueryTracesIDOnlyByIndex for the empty query.")
	}

	ops, err := s.GetOrderedParamSet(ctx, tileNumber)
	if err != nil {
		return nil, err
	}
	outParams := make(chan paramtools.Params, engine.QUERY_ENGINE_CHANNEL_SIZE)

	plan, err := q.QueryPlan(ops)
	if err != nil {
		// Not an error, we just won't match anything in this tile.
		//
		// The plan may be invalid because it is querying with keys or values
		// that don't appear in a tile, which means they query won't work on
		// this tile, but it may still work on other tiles, so we just don't
		// return any results for this tile.
		close(outParams)
		return outParams, nil
	}
	if len(plan) == 0 {
		// We won't match anything in this tile.
		close(outParams)
		return outParams, nil
	}

	beginCommit, endCommit := types.TileCommitRangeForTileNumber(tileNumber, s.tileSize)

	stmt := fmt.Sprintf(`
SELECT DISTINCT
	TraceIDs.trace_name
FROM
	TraceIDs
INNER JOIN
	TraceValues
ON
	TraceValues.trace_id = TraceIDs.trace_id
WHERE
	TraceValues.commit_number>=%d
	AND TraceValues.commit_number<=%d
	AND `, beginCommit, endCommit)

	whereClauses := make([]string, len(plan))
	clauseIndex := 0
	for key, values := range plan {
		inClause := make([]string, len(values))
		for i, value := range values {
			inClause[i] = fmt.Sprintf("'%s=%s'", key, value)
		}
		whereClauses[clauseIndex] = `
		TraceValues.trace_id IN (
			SELECT trace_id FROM Postings WHERE key_value IN (` +
			strings.Join(inClause, ",") +
			`)
			)`
		clauseIndex++
	}
	fullStatement := stmt + strings.Join(whereClauses, " AND ")

	rows, err := s.db.Query(fullStatement)
	if err != nil {
		close(outParams)
		return outParams, err
	}

	defer close(outParams)

	for rows.Next() {
		var traceName string
		if err := rows.Scan(&traceName); err != nil {
			return nil, err
		}
		p, err := query.ParseKey(traceName)
		if err != nil {
			return nil, err
		}
		outParams <- p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return outParams, nil
}

// ReadTraces implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) ReadTraces(tileNumber types.TileNumber, keys []string) (map[string][]float32, error) {
	// Eventually should be broken into batches so we don't exceed the sql
	// engines limit on query sizes.
	ret := map[string][]float32{}
	for _, key := range keys {
		ret[key] = vec32.New(int(s.tileSize))
	}

	// Get the traceIDs for the given keys.
	beginCommit, endCommit := types.TileCommitRangeForTileNumber(tileNumber, s.tileSize)
	stmt := fmt.Sprintf(`
SELECT
	TraceIDs.trace_name, TraceValues.commit_number, TraceValues.val
FROM
    TraceIDs
INNER JOIN
	TraceValues ON TraceValues.trace_id = TraceIDs.trace_id
WHERE
	TraceValues.commit_number>=%d
	AND TraceValues.commit_number<=%d
	AND TraceIDs.trace_name IN (`, beginCommit, endCommit)
	inClause := make([]string, len(keys))
	for i, key := range keys {
		doubleQuoted := fmt.Sprintf("%q", key)
		singleQuoted := "'" + doubleQuoted[1:len(doubleQuoted)-1] + "'"
		inClause[i] = singleQuoted
	}
	fullStatement := stmt + strings.Join(inClause, ",") + ")"
	rows, err := s.db.Query(fullStatement)
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

// TileNumber implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) TileNumber(commitNumber types.CommitNumber) types.TileNumber {
	return types.TileNumberFromCommitNumber(commitNumber, s.tileSize)
}

// TileSize implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) TileSize() int32 {
	return s.tileSize
}

// TraceCount implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) TraceCount(ctx context.Context, tileNumber types.TileNumber) (int64, error) {
	var ret int64
	beginCommit, endCommit := types.TileCommitRangeForTileNumber(tileNumber, s.tileSize)
	err := s.preparedStatements[traceCount].QueryRow(beginCommit, endCommit).Scan(&ret)
	return ret, err
}

// WriteIndices implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) WriteIndices(ctx context.Context, tileNumber types.TileNumber) error {
	return nil
}

func (s *SQLTraceStore) updateSourceFile(source string) (int64, error) {
	if iret, ok := s.cache.Get(source); ok {
		return iret.(int64), nil
	}

	ret := int64(-1)
	_, err := s.preparedStatements[insertIntoSourceFiles].Exec(source)
	if err != nil {
		return ret, err
	}
	err = s.preparedStatements[getSourceFileID].QueryRow(source).Scan(&ret)
	if err != nil {
		return ret, err
	}

	s.cache.Add(source, ret)
	return ret, nil
}

func (s *SQLTraceStore) updateIndex(p paramtools.Params, tileNumber types.TileNumber, traceID int64) error {
	for k, v := range p {
		keyValue := fmt.Sprintf("%s=%s", k, v)
		_, err := s.preparedStatements[insertIntoPostings].Exec(tileNumber, keyValue, traceID)
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
		_, err := s.preparedStatements[insertIntoTraceIDs].Exec(traceName)
		if err != nil {
			return ret, err
		}
		err = s.preparedStatements[getTraceID].QueryRow(traceName).Scan(&ret)
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

func (s *SQLTraceStore) updateTraceValues(traceID int64, commitNumber types.CommitNumber, x float32, sourceID int64) error {
	_, err := s.preparedStatements[replaceTraceValues].Exec(traceID, commitNumber, x, sourceID)
	return err
}

// WriteTraces implements the tracestore.TraceStore interface.
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
		if err := s.updateTraceValues(traceIDs[i], commitNumber, x, sourceID); err != nil {
			return err
		}
	}
	return nil
}

// Confirm that *SQLTraceStore fulfills the tracestore.TraceStore interface.
var _ tracestore.TraceStore = (*SQLTraceStore)(nil)
