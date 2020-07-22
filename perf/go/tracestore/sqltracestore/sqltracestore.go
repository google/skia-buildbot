/*
Package sqltracestore implements a tracestore.TraceStore on top of SQL.

We'll look that the SQL schema used to explain how SQLTraceStore maps
traces into an SQL database.

Each trace name, which is a structured key (See /infra/go/query) of the form
,key1=value1,key2=value2,..., is stored in the TraceIds table so we can use the
much shorter 64 bit trace_id in other tables.

	TraceIDs  (
		trace_id INTEGER PRIMARY KEY,
		trace_name TEXT UNIQUE NOT NULL
	)

Similarly we store the name of every source file that has been ingested in the
SourceFiles table so we can use the shorter 64 bit source_file_id in other
tables.

	SourceFiles (
		source_file_id INTEGER PRIMARY KEY,
		source_file TEXT UNIQUE NOT NULL
	)

We store the values of each trace in the TraceValues table, and use the trace_id
and the commit_number as the primary key. We also store not only the value but
the id of the source file that the value came from.

	TraceValues (
		trace_id INTEGER,
		commit_number INTEGER,
		val REAL,
		source_file_id INTEGER,
		PRIMARY KEY (trace_id, commit_number)
	)

Just using this table we can construct some useful queries. For example
we can count the number of traces in a single tile, in this case the
0th tile in a system with a tileSize of 256:

	SELECT
		COUNT(DISTINCT trace_id)
	FROM
		TraceValues
	WHERE
  		commit_number >= 0 AND commit_number < 256;

The Postings table is our inverted index for looking up which trace ids
contain which key=value pairs. For a good introduction to postings and search
https://www.tbray.org/ongoing/When/200x/2003/06/18/HowSearchWorks is a good
resource.

Remember that each trace name is a structured key of the form
,arch=x86,config=8888,..., and that over time traces may come and go, i.e. we
may stop running a test, or start running new tests, so if we want to make
searching for traces efficient we need to be aware of how those trace ids change
over time. The answer is to break our store in Tiles, i.e. blocks of commits of
tileSize length, and then for each Tile we keep an inverted index of the trace
ids. This allows us to not only construct fast queries, but to also do things
like build ParamSets, a collection of all the keys and all their values ever
seen for a particular Tile.

In the table below we store a key_value which is the literal "key=value" part of
a trace name, along with the tile_number and the 64 bit trace id. Note that
tile_number is just int(commitNumber/tileSize).

	Postings  (
		tile_number INTEGER,
		key_value text NOT NULL,
		trace_id INTEGER,
		PRIMARY KEY (tile_number, key_value, trace_id)
	)

So for example to build a ParamSet from Postings:

	SELECT DISTINCT
		key_value
	FROM
		Postings
	WHERE
		tile_number=0;

To find the most recent tile:

	SELECT
		tile_number
	FROM
		Postings
	ORDER BY
		tile_number DESC LIMIT 1;

And finally, to retrieve all the trace values that
would match a query, we first start with sub-queries for
each of the common keys in the query, which produce the
trace_ids, which are then ANDed across all the distinct
keys in the query. Finally that list is inner joined to the
TraceValues table to load up all the values.

	SELECT
		TraceIDs.trace_name, TraceValues.commit_number, TraceValues.val
	FROM
		TraceIDs
	INNER JOIN
		TraceValues
	ON
		TraceValues.trace_id = TraceIDs.trace_id
	WHERE
  		TraceValues.trace_id IN (
			SELECT trace_id FROM Postings
			WHERE key_value IN ("arch=x86", "arch=arm")
			AND tile_number=0
  		)
	  AND
	  	TraceValues.trace_id IN (
			SELECT trace_id FROM Postings
			WHERE key_value IN ("config=8888")
			AND tile_number=0
		  );

Look in migrations/test.sql for more example of raw queries using
a simple example dataset.
*/
package sqltracestore

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracestore/sqltracestore/engine"
	"go.skia.org/infra/perf/go/types"
)

// cacheSize is the size of the LRU cache.
//
// TODO(jcgregorio) Move to config.InstanceConfig since this should be tweaked
// per instance.
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
	// db is the SQL database instance.
	db *sql.DB

	// preparedStatements are all the prepared SQL statements.
	preparedStatements map[statement]*sql.Stmt

	// cache is an LRU cache used to store the ids (int64) of trace names (string).
	cache *lru.Cache

	// tileSize is the number of commits per Tile.
	tileSize int32
}

// New returns a new *SQLTraceStore.
//
// We presume all migrations have been run against db before this function is
// called.
func New(db *sql.DB, dialect perfsql.Dialect, tileSize int32) (*SQLTraceStore, error) {
	preparedStatements := map[statement]*sql.Stmt{}
	for key, statement := range statementsByDialect[dialect] {
		prepared, err := db.Prepare(statement)
		if err != nil {
			return nil, skerr.Wrapf(err, "preparing statement %v, %q", key, statement)
		}
		preparedStatements[key] = prepared
	}

	cache, err := lru.New(cacheSize)
	if err != nil {
		return nil, skerr.Wrap(err)
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
	if err := s.preparedStatements[countIndices].QueryRowContext(ctx, tileNumber).Scan(&ret); err != nil {
		return 0, skerr.Wrap(err)
	}
	return ret, nil
}

// GetLatestTile implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) GetLatestTile() (types.TileNumber, error) {
	var tileNumber int64
	if err := s.preparedStatements[getLatestTile].QueryRowContext(context.TODO()).Scan(&tileNumber); err != nil {
		return types.BadTileNumber, skerr.Wrap(err)
	}
	return types.TileNumber(tileNumber), nil
}

func (s *SQLTraceStore) paramSetForTile(tileNumber types.TileNumber) (paramtools.ParamSet, error) {
	rows, err := s.preparedStatements[paramSetForTile].QueryContext(context.TODO(), tileNumber)
	if err != nil {
		return nil, skerr.Wrapf(err, "tileNumer=%d", tileNumber)
	}
	ret := paramtools.NewParamSet()
	for rows.Next() {
		var keyValue string
		if err := rows.Scan(&keyValue); err != nil {
			return nil, skerr.Wrapf(err, "tileNumer=%d", tileNumber)
		}
		parts := strings.Split(keyValue, "=")
		if len(parts) != 2 {
			return nil, skerr.Fmt("Invalid key=value form: %q", keyValue)
		}
		ret.AddParams(paramtools.Params{parts[0]: parts[1]})
	}
	if err := rows.Err(); err != nil {
		return nil, skerr.Wrapf(err, "tileNumer=%d", tileNumber)
	}
	ret.Normalize()
	return ret, nil
}

// GetOrderedParamSet implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) GetOrderedParamSet(ctx context.Context, tileNumber types.TileNumber) (*paramtools.OrderedParamSet, error) {
	ps, err := s.paramSetForTile(tileNumber)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	ret := paramtools.NewOrderedParamSet()
	ret.Update(ps)
	sort.Strings(ret.KeyOrder)
	return ret, nil
}

// GetSource implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) GetSource(ctx context.Context, commitNumber types.CommitNumber, traceName string) (string, error) {
	var filename string
	if err := s.preparedStatements[getSource].QueryRowContext(ctx, traceName, commitNumber).Scan(&filename); err != nil {
		return "", skerr.Wrapf(err, "commitNumber=%d traceName=%q", commitNumber, traceName)
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
		return nil, skerr.Wrapf(err, "Failed to get OPS.")
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
	// Sanitize our inputs.
	if err := query.ValidateParamSet(plan); err != nil {
		return nil, skerr.Wrapf(err, "invalid query %#v", *q)
	}
	tileNumberString := strconv.FormatInt(int64(tileNumber), 10)
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
	TraceValues.commit_number >= %d
	AND TraceValues.commit_number <= %d
	AND `, beginCommit, endCommit)

	// TODO(jcgregorio) Break out into own function.
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
			`) AND tile_number=` + tileNumberString + `
		)`
		clauseIndex++
	}
	fullStatement := stmt + strings.Join(whereClauses, " AND ")

	rows, err := s.db.QueryContext(context.TODO(), fullStatement)
	if err != nil {
		sklog.Debugf("QueryTracesByIndex: fullStatement: %q", fullStatement)
		return nil, skerr.Wrap(err)
	}

	ret := types.TraceSet{}

	for rows.Next() {
		var traceName string
		var commitNumber int64
		var val float64
		if err := rows.Scan(&traceName, &commitNumber, &val); err != nil {
			return nil, skerr.Wrap(err)
		}
		if _, ok := ret[traceName]; ok {
			ret[traceName][commitNumber%int64(s.tileSize)] = float32(val)
		} else {
			// TODO(jcgregorio) Replace this vec32.New() with a
			// https://golang.org/pkg/sync/#Pool since this is our most used/reused
			// type of memory.
			ret[traceName] = vec32.New(int(s.tileSize))
			ret[traceName][commitNumber%int64(s.tileSize)] = float32(val)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, skerr.Wrap(err)
	}

	return ret, nil
}

// QueryTracesIDOnlyByIndex implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) QueryTracesIDOnlyByIndex(ctx context.Context, tileNumber types.TileNumber, q *query.Query) (<-chan paramtools.Params, error) {
	outParams := make(chan paramtools.Params, engine.QueryEngineChannelSize)
	if q.Empty() {
		close(outParams)
		return outParams, skerr.Fmt("Can't run QueryTracesIDOnlyByIndex for the empty query.")
	}

	ops, err := s.GetOrderedParamSet(ctx, tileNumber)
	if err != nil {
		close(outParams)
		return outParams, skerr.Wrap(err)
	}

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
	// Sanitize our inputs.
	if err := query.ValidateParamSet(plan); err != nil {
		close(outParams)
		return outParams, skerr.Wrapf(err, "invalid query %#v", *q)
	}

	tileNumberString := strconv.FormatInt(int64(tileNumber), 10)
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
	TraceValues.commit_number >= %d
	AND TraceValues.commit_number <= %d
	AND `, beginCommit, endCommit)

	// TODO(jcgregorio) Break out into own function.
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
			`) AND tile_number=` + tileNumberString + `
			)`
		clauseIndex++
	}
	fullStatement := stmt + strings.Join(whereClauses, " AND ")

	rows, err := s.db.QueryContext(context.TODO(), fullStatement)
	if err != nil {
		sklog.Debugf("QueryTracesIDOnlyByIndex: fullStatement: %q", fullStatement)
		close(outParams)
		return outParams, skerr.Wrap(err)
	}

	go func() {
		defer close(outParams)

		for rows.Next() {
			var traceName string
			if err := rows.Scan(&traceName); err != nil {
				sklog.Errorf("Failed to scan traceName: %s", skerr.Wrap(err))
				return
			}
			p, err := query.ParseKey(traceName)
			if err != nil {
				sklog.Errorf("Failed to parse traceName: %s", skerr.Wrap(err))
				return
			}
			outParams <- p
		}
		if err := rows.Err(); err != nil {
			sklog.Errorf("Failed while reading traceNames: %s", skerr.Wrap(err))
			return
		}
	}()

	return outParams, nil
}

// ReadTraces implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) ReadTraces(tileNumber types.TileNumber, keys []string) (types.TraceSet, error) {
	// TODO(jcgregorio) Should be broken into batches so we don't exceed the SQL
	// engine limit on query sizes.
	ret := types.TraceSet{}
	for _, key := range keys {
		if !query.ValidateKey(key) {
			return nil, skerr.Fmt("Invalid key stored in shortcut: %q", key)
		}

		// TODO(jcgregorio) Replace this vec32.New() with a
		// https://golang.org/pkg/sync/#Pool since this is our most used/reused
		// type of memory.
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
	TraceValues.commit_number >= %d
	AND TraceValues.commit_number <= %d
	AND TraceIDs.trace_name IN (`, beginCommit, endCommit)
	inClause := make([]string, len(keys))
	for i, key := range keys {
		singleQuoted := "'" + key + "'"
		inClause[i] = singleQuoted
	}
	fullStatement := stmt + strings.Join(inClause, ",") + ")"
	rows, err := s.db.QueryContext(context.TODO(), fullStatement)
	if err != nil {
		sklog.Debugf("ReadTraces: fullStatement: %q", fullStatement)
		return nil, skerr.Wrap(err)
	}
	for rows.Next() {
		var traceName string
		var commitNumber int64
		var val float64
		if err := rows.Scan(&traceName, &commitNumber, &val); err != nil {
			return nil, skerr.Wrap(err)
		}
		ret[traceName][commitNumber%int64(s.tileSize)] = float32(val)
	}
	if err := rows.Err(); err != nil {
		return nil, skerr.Wrapf(err, "tileNumber=%d", tileNumber)
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
	err := s.preparedStatements[traceCount].QueryRowContext(context.TODO(), beginCommit, endCommit).Scan(&ret)
	return ret, skerr.Wrap(err)
}

// WriteIndices implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) WriteIndices(ctx context.Context, tileNumber types.TileNumber) error {
	// TODO(jcgregorio) This func should be removed from the interface since it only applied to BigTableTraceStore.
	return nil
}

// updateSourceFile writes the filename into the SourceFiles table and returns
// the int64 id of that filename.
func (s *SQLTraceStore) updateSourceFile(filename string) (int64, error) {
	ret := int64(-1)
	_, err := s.preparedStatements[insertIntoSourceFiles].ExecContext(context.TODO(), filename)
	if err != nil {
		return ret, skerr.Wrap(err)
	}
	err = s.preparedStatements[getSourceFileID].QueryRowContext(context.TODO(), filename).Scan(&ret)
	if err != nil {
		return ret, skerr.Wrap(err)
	}

	return ret, nil
}

// updatePostings writes all the entries into our inverted index in Postings for
// the given traceID and tileNumber. The Params given are from the parse trace
// name.
func (s *SQLTraceStore) updatePostings(p paramtools.Params, tileNumber types.TileNumber, traceID int64) error {
	for k, v := range p {
		keyValue := fmt.Sprintf("%s=%s", k, v)
		_, err := s.preparedStatements[insertIntoPostings].ExecContext(context.TODO(), tileNumber, keyValue, traceID)
		if err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// writeTraceIDAndPostings writes the trace name into the TraceIDs table and returns the
// int64 id of that trace name. This operation will happen repeatedly as data is
// ingested so we cache the results in the LRU cache.
func (s *SQLTraceStore) writeTraceIDAndPostings(traceNameAsParams paramtools.Params, tileNumber types.TileNumber) (int64, error) {
	ret := int64(-1)

	traceName, err := query.MakeKey(traceNameAsParams)
	if err != nil {
		return ret, skerr.Wrap(err)
	}

	// Get an int64 trace id for the traceName.
	if iret, ok := s.cache.Get(traceName); ok {
		ret = iret.(int64)
	} else {
		_, err := s.preparedStatements[insertIntoTraceIDs].ExecContext(context.TODO(), traceName)
		if err != nil {
			return ret, skerr.Wrapf(err, "traceName=%q", traceName)
		}
		err = s.preparedStatements[getTraceID].QueryRowContext(context.TODO(), traceName).Scan(&ret)
		if err != nil {
			return ret, skerr.Wrapf(err, "traceName=%q", traceName)
		}
		s.cache.Add(traceName, ret)
	}

	// Update postings.
	if err := s.updatePostings(traceNameAsParams, tileNumber, ret); err != nil {
		return ret, skerr.Wrapf(err, "traceName=%q", traceName)
	}

	return ret, nil
}

// updateTraceValues writes a single entry in to the TraceValues table.
func (s *SQLTraceStore) updateTraceValues(traceID int64, commitNumber types.CommitNumber, x float32, sourceID int64) error {
	_, err := s.preparedStatements[replaceTraceValues].ExecContext(context.TODO(), traceID, commitNumber, x, sourceID)
	return skerr.Wrap(err)
}

// WriteTraces implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) WriteTraces(commitNumber types.CommitNumber, params []paramtools.Params, values []float32, _ paramtools.ParamSet, source string, _ time.Time) error {
	tileNumber := types.TileNumberFromCommitNumber(commitNumber, s.tileSize)
	// Get the row id for the source file.
	sourceID, err := s.updateSourceFile(source)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Get trace ids for each trace and add trace ids to the index/postings.
	// We populate the traceIDs slice whose values are 1:1 with the values and
	// params slices.
	traceIDs := make([]int64, len(params))
	for i, p := range params {
		traceID, err := s.writeTraceIDAndPostings(p, tileNumber)
		if err != nil {
			return skerr.Wrap(err)
		}
		traceIDs[i] = traceID
	}

	// Now add each trace value.
	for i, x := range values {
		if err := s.updateTraceValues(traceIDs[i], commitNumber, x, sourceID); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// Confirm that *SQLTraceStore fulfills the tracestore.TraceStore interface.
var _ tracestore.TraceStore = (*SQLTraceStore)(nil)
