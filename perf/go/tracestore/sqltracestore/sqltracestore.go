/*
Package sqltracestore implements a tracestore.TraceStore on top of SQL.

We'll look that the SQL schema used to explain how SQLTraceStore maps
traces into an SQL database.

Each trace name, which is a structured key (See /infra/go/query) of the form
,key1=value1,key2=value2,..., is stored in the TraceNames table so we can use the
much shorter 128 bit md5 hash in trace_id in other tables. The value of the
trace name is parsed into a paramtools.Params and stored in the 'params' column
with an inverted index, which enables all the queries that Perf supports.

    CREATE TABLE IF NOT EXISTS TraceNames (
        -- md5(trace_name)
        trace_id BYTES PRIMARY KEY,
        -- The params that make up the trace_id, {"arch=x86", "config=8888"}.
        params JSONB NOT NULL,
        INVERTED INDEX (params)
    );

Similarly we store the name of every source file that has been ingested in the
SourceFiles table so we can use the shorter 64 bit source_file_id in other
tables.

    SourceFiles (
        source_file_id INTEGER PRIMARY KEY,
        source_file TEXT UNIQUE NOT NULL
    )
    CREATE TABLE IF NOT EXISTS SourceFiles (
        source_file_id INT PRIMARY KEY DEFAULT unique_rowid(),
        source_file STRING UNIQUE NOT NULL
    );

We store the values of each trace in the TraceValues2 table, and use the trace_id
and the commit_number as the primary key. We also store not only the value but
the id of the source file that the value came from.

    CREATE TABLE IF NOT EXISTS TraceValues2 (
        -- md5(trace_name) from TraceNames.
        trace_id BYTES,
        -- A types.CommitNumber.
        commit_number INT,
        -- The floating point measurement.
        val REAL,
        -- Id of the source filename, from SourceFiles.
        source_file_id INT,
        PRIMARY KEY (trace_id, commit_number)
    );

Just using this table we can construct some useful queries. For example
we can count the number of traces in a single tile, in this case the
0th tile in a system with a tileSize of 256:

    SELECT
        COUNT(DISTINCT trace_id)
    FROM
        TraceValues2
    WHERE
          commit_number >= 0 AND commit_number < 256;

The JSONB serialized Params in the TraceNames table allows
building ParamSets for a range of commits:

    SELECT
        DISTINCT TraceNames.params
    FROM
        TraceNames
        INNER JOIN TraceValues2 ON TraceNames.trace_id = TraceValues2.trace_id
    WHERE
        TraceValues2.commit_number >= 0
        AND TraceValues2.commit_number < 512;


And finally, to retrieve all the trace values that
would match a query:

    SELECT
        TraceNames.params,
        TraceValues2.commit_number,
        TraceValues2.val
    FROM
        TraceNames
        INNER JOIN TraceValues2 ON TraceValues2.trace_id = TraceNames.trace_id
    WHERE
        TraceNames.params ->> 'arch' IN ('x86')
        AND TraceNames.params ->> 'config' IN ('565', '8888')
        AND TraceValues2.commit_number >= 0
        AND TraceValues2.commit_number < 255;

Look in migrations/cdb.sql for more example of raw queries using
a simple example dataset.
*/
package sqltracestore

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"sort"
	"text/template"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/cache"
	"go.skia.org/infra/perf/go/cache/local"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracestore/sqltracestore/engine"
	"go.skia.org/infra/perf/go/types"
)

const writeTracesChunkSize = 200

// defaultCacheSize is the size of the in-memory LRU cache if no size was
// specified in the config file.
const defaultCacheSize = 20 * 1000 * 1000

// traceIDForSQL is the type of the IDs that are used in the SQL queries,
// they are hex encoded md5 hashes of a trace name, e.g. "\x00112233...".
// Note the \x prefix which tells CockroachDB that this is hex encoded.
type traceIDForSQL string

var badTraceIDFromSQL traceIDForSQL = ""

// Calculates the traceIDForSQL for the given trace name, e.g. "\x00112233...".
// Note the \x prefix which tells CockroachDB that this is hex encoded.
func traceIDForSQLFromTraceName(traceName string) traceIDForSQL {
	b := md5.Sum([]byte(traceName))
	return traceIDForSQL(fmt.Sprintf("\\x%x", b))
}

// sourceFileIDFromSQL is the type of the IDs that are used in the SQL database
// for source files.
type sourceFileIDFromSQL int64

const badSourceFileIDFromSQL sourceFileIDFromSQL = -1

// statement is an SQL statement or fragment of an SQL statement.
type statement int

// All the different statements we need. Each statement will appear either in
// templatesByDialect or statementsByDialect.
const (
	insertIntoSourceFiles statement = iota
	getSourceFileID
	replaceTraceNames
	getTraceID
	replaceTraceValues
	getLatestCommit
	paramSetForTile
	getSource
	traceCount
	queryTraces
	queryTracesIDOnly
	readTraces
)

var templates = map[statement]string{
	replaceTraceValues: `INSERT INTO
            TraceValues2 (trace_id, commit_number, val, source_file_id)
        VALUES
        {{ range $index, $element :=  . -}}
            {{ if $index }},{{end}}
            (
                '{{ $element.MD5HexTraceID }}', {{ $element.CommitNumber }}, {{ $element.Val }}, {{ $element.SourceFileID }}
            )
        {{ end }}
		ON CONFLICT
		DO NOTHING
		`,
	replaceTraceNames: `INSERT INTO
            TraceNames (trace_id, params)
        VALUES
        {{ range $index, $element :=  . -}}
            {{ if $index }},{{end}}
            (
                '{{ $element.MD5HexTraceID }}', '{{ $element.JSONParams }}'
            )
        {{ end }}
		ON CONFLICT
		DO NOTHING
		`,
	queryTraces: `
        SELECT
            TraceNames.params,
            TraceValues2.commit_number,
            TraceValues2.val
        FROM
            TraceNames
        INNER JOIN TraceValues2 ON TraceValues2.trace_id = TraceNames.trace_id
        WHERE
            TraceValues2.commit_number >= {{ .BeginCommitNumber }}
            AND TraceValues2.commit_number < {{ .EndCommitNumber }}
            {{ range  $key, $values := .QueryPlan }}
                AND TraceNames.params ->> '{{ $key }}' IN
                (
                    {{ range $index, $value :=  $values -}}
                        {{ if $index }},{{end}} '{{ $value }}'
                    {{ end }}
                )
            {{ end }}
        `,
	queryTracesIDOnly: `
        SELECT
            TraceNames.params
        FROM
            TraceNames
        INNER JOIN TraceValues2 ON TraceValues2.trace_id = TraceNames.trace_id
        WHERE
            TraceValues2.commit_number >= {{ .BeginCommitNumber }}
            AND TraceValues2.commit_number < {{ .EndCommitNumber }}
            {{ range  $key, $values := .QueryPlan }}
                AND TraceNames.params ->> '{{ $key }}' IN
                (
                    {{ range $index, $value :=  $values -}}
                        {{ if $index }},{{end}} '{{ $value }}'
                    {{ end }}
                )
            {{ end }}
        `,
	readTraces: `
        SELECT
            TraceNames.params,
            TraceValues2.commit_number,
            TraceValues2.val
        FROM
            TraceNames
        INNER JOIN TraceValues2 ON TraceValues2.trace_id = TraceNames.trace_id
        WHERE
            TraceValues2.commit_number >= {{ .BeginCommitNumber }}
            AND TraceValues2.commit_number < {{ .EndCommitNumber }}
            AND TraceValues2.trace_id IN
            (
                {{ range $index, $trace_id :=  .TraceIDs -}}
                    {{ if $index }},{{end}} '{{ $trace_id }}'
                {{ end }}
            )
        `,
	getSource: `
        SELECT
            SourceFiles.source_file
        FROM
            TraceNames
        INNER JOIN TraceValues2 ON TraceValues2.trace_id = TraceNames.trace_id
        INNER JOIN SourceFiles ON SourceFiles.source_file_id = TraceValues2.source_file_id
        WHERE
            TraceNames.trace_id = '{{ .MD5HexTraceID }}'
            AND TraceValues2.commit_number = {{ .CommitNumber }}`,
}

// replaceTraceValuesContext is the context for the replaceTraceValues template.
type replaceTraceValuesContext struct {
	// The MD5 sum of the trace name as a hex string, i.e.
	// "\xfe385b159ff55dca481069805e5ff050". Note the leading \x which
	// CockroachDB will use to know the string is in hex.
	MD5HexTraceID traceIDForSQL

	CommitNumber types.CommitNumber
	Val          float32
	SourceFileID sourceFileIDFromSQL
}

// replaceTraceNamesContext is the context for the replaceTraceNames template.
type replaceTraceNamesContext struct {
	// The trace's Params serialize as JSON.
	JSONParams string

	// The MD5 sum of the trace name as a hex string, i.e.
	// "\xfe385b159ff55dca481069805e5ff050". Note the leading \x which
	// CockroachDB will use to know the string is in hex.
	MD5HexTraceID traceIDForSQL
}

// queryTracesContext is the context for the queryTraces template.
type queryTracesContext struct {
	BeginCommitNumber types.CommitNumber
	EndCommitNumber   types.CommitNumber
	QueryPlan         paramtools.ParamSet
}

// readTracesContext is the context for the readTraces template.
type readTracesContext struct {
	BeginCommitNumber types.CommitNumber
	EndCommitNumber   types.CommitNumber
	TraceIDs          []traceIDForSQL
}

// getSourceContext is the context for the getSource template.
type getSourceContext struct {
	CommitNumber types.CommitNumber

	// The MD5 sum of the trace name as a hex string, i.e.
	// "\xfe385b159ff55dca481069805e5ff050". Note the leading \x which
	// CockroachDB will use to know the string is in hex.
	MD5HexTraceID traceIDForSQL
}

var statements = map[statement]string{
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
	getLatestCommit: `
        SELECT
            commit_number
        FROM
            TraceValues2
        ORDER BY
            commit_number DESC
        LIMIT
            1;`,
	paramSetForTile: `
        SELECT
            DISTINCT TraceNames.params
        FROM
            TraceNames
        INNER JOIN TraceValues2 ON TraceNames.trace_id = TraceValues2.trace_id
        WHERE
            TraceValues2.commit_number >= $1
            AND TraceValues2.commit_number < $2;`,
	traceCount: `
        SELECT
            COUNT(DISTINCT trace_id)
        FROM
            TraceValues2
        WHERE
          commit_number >= $1 AND commit_number <= $2`,
}

// SQLTraceStore implements tracestore.TraceStore backed onto an SQL database.
type SQLTraceStore struct {
	// db is the SQL database instance.
	db *pgxpool.Pool

	// unpreparedStatements are parsed templates that can be used to construct SQL statements.
	unpreparedStatements map[statement]*template.Template

	cache cache.Cache

	// tileSize is the number of commits per Tile.
	tileSize int32

	// metrics
	writeTracesMetric         metrics2.Float64SummaryMetric
	writeTracesMetricSQL      metrics2.Float64SummaryMetric
	buildTracesContextsMetric metrics2.Float64SummaryMetric
}

// New returns a new *SQLTraceStore.
//
// We presume all migrations have been run against db before this function is
// called.
func New(db *pgxpool.Pool, datastoreConfig config.DataStoreConfig) (*SQLTraceStore, error) {
	unpreparedStatements := map[statement]*template.Template{}
	for key, tmpl := range templates {
		t, err := template.New("").Parse(tmpl)
		if err != nil {
			return nil, skerr.Wrapf(err, "parsing template %v, %q", key, tmpl)
		}
		unpreparedStatements[key] = t
	}

	cache, err := local.New(defaultCacheSize)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &SQLTraceStore{
		db:                        db,
		unpreparedStatements:      unpreparedStatements,
		tileSize:                  datastoreConfig.TileSize,
		cache:                     cache,
		writeTracesMetric:         metrics2.GetFloat64SummaryMetric("perfserver_sqltracestore_writeTraces"),
		writeTracesMetricSQL:      metrics2.GetFloat64SummaryMetric("perfserver_sqltracestore_writeTracesSQL"),
		buildTracesContextsMetric: metrics2.GetFloat64SummaryMetric("perfserver_sqltracestore_buildTracesContext"),
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

	// This doesn't make any sense for the SQL implementation of TraceStore.
	return 0, nil
}

// GetLatestTile implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) GetLatestTile() (types.TileNumber, error) {
	mostRecentCommit := types.BadCommitNumber
	if err := s.db.QueryRow(context.TODO(), statements[getLatestCommit]).Scan(&mostRecentCommit); err != nil {
		return types.BadTileNumber, skerr.Wrap(err)
	}
	return types.TileNumberFromCommitNumber(mostRecentCommit, s.tileSize), nil
}

func (s *SQLTraceStore) paramSetForTile(tileNumber types.TileNumber) (paramtools.ParamSet, error) {
	// Convert the tile number into a range of commits, since we don't store data by
	// tile anymore.
	beginCommit, endCommit := types.TileCommitRangeForTileNumber(tileNumber, s.tileSize)

	rows, err := s.db.Query(context.TODO(), statements[paramSetForTile], beginCommit, endCommit)
	if err != nil {
		fmt.Printf("%q %d %d", statements[paramSetForTile], beginCommit, endCommit)
		return nil, skerr.Wrapf(err, "Failed querying - tileNumber=%d", tileNumber)
	}
	ret := paramtools.NewParamSet()
	for rows.Next() {
		var jsonParams string
		if err := rows.Scan(&jsonParams); err != nil {
			return nil, skerr.Wrapf(err, "Failed scanning row - tileNumber=%d", tileNumber)
		}
		var ps paramtools.Params
		if err := json.Unmarshal([]byte(jsonParams), &ps); err != nil {
			return nil, skerr.Wrapf(err, "Failed unmarshal - tileNumber=%d", tileNumber)
		}
		ret.AddParams(ps)
	}
	if err == pgx.ErrNoRows {
		return ret, nil
	}
	if err := rows.Err(); err != nil {
		return nil, skerr.Wrapf(err, "Other failure - tileNumber=%d", tileNumber)
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
	traceID := traceIDForSQLFromTraceName(traceName)

	sourceContext := getSourceContext{
		MD5HexTraceID: traceID,
		CommitNumber:  commitNumber,
	}

	var b bytes.Buffer
	if err := s.unpreparedStatements[getSource].Execute(&b, sourceContext); err != nil {
		return "", skerr.Wrapf(err, "failed to expand get source template")
	}
	sql := b.String()

	if err := s.db.QueryRow(ctx, sql).Scan(&filename); err != nil {
		return "", skerr.Wrapf(err, "commitNumber=%d traceName=%q traceID=%q", commitNumber, traceName, traceID)
	}
	return filename, nil
}

// OffsetFromCommitNumber implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) OffsetFromCommitNumber(commitNumber types.CommitNumber) int32 {
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

	// Prepare the template context.
	beginCommit, endCommit := types.TileCommitRangeForTileNumber(tileNumber, s.tileSize)
	context := queryTracesContext{
		BeginCommitNumber: beginCommit,
		EndCommitNumber:   endCommit,
		QueryPlan:         plan,
	}

	// Expand the template for the SQL.
	var b bytes.Buffer
	if err := s.unpreparedStatements[queryTraces].Execute(&b, context); err != nil {
		return nil, skerr.Wrapf(err, "failed to expand trace names template")
	}

	sql := b.String()
	// Execute the query.
	rows, err := s.db.Query(ctx, sql)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	ret := types.TraceSet{}
	for rows.Next() {
		var jsonParams string
		var commitNumber types.CommitNumber
		var val float64
		if err := rows.Scan(&jsonParams, &commitNumber, &val); err != nil {
			return nil, skerr.Wrap(err)
		}

		p := paramtools.Params{}
		if err := json.Unmarshal([]byte(jsonParams), &p); err != nil {
			sklog.Warningf("Invalid JSON params found in query response: %s", err)
			continue
		}
		traceName, err := query.MakeKey(p)
		if err != nil {
			sklog.Warningf("Invalid trace name found in query response: %s", err)
			continue
		}
		offset := s.OffsetFromCommitNumber(commitNumber)
		if _, ok := ret[traceName]; ok {
			ret[traceName][offset] = float32(val)
		} else {
			// TODO(jcgregorio) Replace this vec32.New() with a
			// https://golang.org/pkg/sync/#Pool since this is our most used/reused
			// type of memory.
			ret[traceName] = vec32.New(int(s.tileSize))
			ret[traceName][offset] = float32(val)
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
		return nil, skerr.Wrapf(err, "invalid query %#v", *q)
	}

	// Prepare the template context.
	beginCommit, endCommit := types.TileCommitRangeForTileNumber(tileNumber, s.tileSize)
	context := queryTracesContext{
		BeginCommitNumber: beginCommit,
		EndCommitNumber:   endCommit,
		QueryPlan:         plan,
	}

	// Expand the template for the SQL.
	var b bytes.Buffer
	if err := s.unpreparedStatements[queryTracesIDOnly].Execute(&b, context); err != nil {
		return nil, skerr.Wrapf(err, "failed to expand trace names template")
	}

	// Execute the query.
	rows, err := s.db.Query(ctx, b.String())
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	go func() {
		defer close(outParams)

		for rows.Next() {
			var jsonParams string
			if err := rows.Scan(&jsonParams); err != nil {
				sklog.Errorf("Failed to scan traceName: %s", skerr.Wrap(err))
				return
			}

			p := paramtools.Params{}
			if err := json.Unmarshal([]byte(jsonParams), &p); err != nil {
				sklog.Errorf("Failed to parse traceName: %s", skerr.Wrap(err))
				continue
			}
			outParams <- p

		}
		if err := rows.Err(); err != nil {
			sklog.Errorf("Failed while reading traceNames: %s", skerr.Wrap(err))
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

	// Populate the context for the SQL template.
	readTracesContext := readTracesContext{
		BeginCommitNumber: beginCommit,
		EndCommitNumber:   endCommit,
		TraceIDs:          make([]traceIDForSQL, 0, len(keys)),
	}

	for _, traceName := range keys {
		readTracesContext.TraceIDs = append(readTracesContext.TraceIDs, traceIDForSQLFromTraceName(traceName))
	}
	// Expand the template for the SQL.
	var b bytes.Buffer
	if err := s.unpreparedStatements[readTraces].Execute(&b, readTracesContext); err != nil {
		return nil, skerr.Wrapf(err, "failed to expand read traces template")
	}

	// Execute the query.
	rows, err := s.db.Query(context.TODO(), b.String())
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	for rows.Next() {
		var jsonParams string
		var commitNumber types.CommitNumber
		var val float64
		if err := rows.Scan(&jsonParams, &commitNumber, &val); err != nil {
			return nil, skerr.Wrap(err)
		}

		p := paramtools.Params{}
		if err := json.Unmarshal([]byte(jsonParams), &p); err != nil {
			sklog.Warningf("Invalid JSON params found in query response: %s", err)
			continue
		}
		traceName, err := query.MakeKey(p)
		if err != nil {
			sklog.Warningf("Invalid trace name found in query response: %s", err)
			continue
		}
		ret[traceName][s.OffsetFromCommitNumber(commitNumber)] = float32(val)
	}
	if err := rows.Err(); err != nil {
		return nil, skerr.Wrap(err)
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
	err := s.db.QueryRow(context.TODO(), statements[traceCount], beginCommit, endCommit).Scan(&ret)
	return ret, skerr.Wrap(err)
}

// WriteIndices implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) WriteIndices(ctx context.Context, tileNumber types.TileNumber) error {
	// TODO(jcgregorio) This func should be removed from the interface since it only applied to BigTableTraceStore.
	return nil
}

// updateSourceFile writes the filename into the SourceFiles table and returns
// the sourceFileIDFromSQL of that filename.
func (s *SQLTraceStore) updateSourceFile(filename string) (sourceFileIDFromSQL, error) {
	ret := badSourceFileIDFromSQL
	_, err := s.db.Exec(context.TODO(), statements[insertIntoSourceFiles], filename)
	if err != nil {
		return ret, skerr.Wrap(err)
	}
	err = s.db.QueryRow(context.TODO(), statements[getSourceFileID], filename).Scan(&ret)
	if err != nil {
		return ret, skerr.Wrap(err)
	}

	return ret, nil
}

// WriteTraces implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) WriteTraces(commitNumber types.CommitNumber, params []paramtools.Params, values []float32, _ paramtools.ParamSet, source string, _ time.Time) error {
	defer timer.NewWithSummary("perfserver_sqltracestore_writeTraces", s.writeTracesMetric).Stop()
	// Get the row id for the source file.
	sourceID, err := s.updateSourceFile(source)
	if err != nil {
		return skerr.Wrap(err)
	}

	t := timer.NewWithSummary("perfserver_sqltracestore_buildTracesContexts", s.buildTracesContextsMetric)
	// Build the 'context' which will be used to populate the SQL template.
	namesTemplateContext := make([]replaceTraceNamesContext, 0, len(params))
	valuesTemplateContext := make([]replaceTraceValuesContext, 0, len(params))

	for i, p := range params {
		traceName, err := query.MakeKey(p)
		if err != nil {
			continue
		}
		traceID := traceIDForSQLFromTraceName(traceName)
		jsonParams, err := json.Marshal(p)
		if err != nil {
			continue
		}
		valuesTemplateContext = append(valuesTemplateContext, replaceTraceValuesContext{
			MD5HexTraceID: traceID,
			CommitNumber:  commitNumber,
			Val:           values[i],
			SourceFileID:  sourceID,
		})

		if !s.cache.Exists(string(traceID)) {
			namesTemplateContext = append(namesTemplateContext, replaceTraceNamesContext{
				MD5HexTraceID: traceID,
				JSONParams:    string(jsonParams),
			})
		}
	}
	t.Stop()

	defer timer.NewWithSummary("perfserver_sqltracestore_writeTraces_sql", s.writeTracesMetricSQL).Stop()
	sklog.Infof("About to format %d trace names", len(params))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if len(namesTemplateContext) > 0 {

		err = util.ChunkIter(len(namesTemplateContext), 100, func(startIdx int, endIdx int) error {
			var b bytes.Buffer
			if err := s.unpreparedStatements[replaceTraceNames].Execute(&b, namesTemplateContext[startIdx:endIdx]); err != nil {
				return skerr.Wrapf(err, "failed to expand trace names template on slice [%d, %d]", startIdx, endIdx)
			}
			sql := b.String()

			sklog.Infof("About to write %d trace names with sql of length %d", len(params), len(sql))
			if _, err := s.db.Exec(ctx, sql); err != nil {
				return skerr.Wrapf(err, "Executing: %q", b.String())
			}
			return nil
		})

		if err != nil {
			return err
		}

		for _, entry := range namesTemplateContext {
			s.cache.Add(string(entry.MD5HexTraceID), "1")
		}
	}

	sklog.Infof("About to format %d trace values", len(params))

	err = util.ChunkIter(len(valuesTemplateContext), writeTracesChunkSize, func(startIdx int, endIdx int) error {
		var b bytes.Buffer
		if err := s.unpreparedStatements[replaceTraceValues].Execute(&b, valuesTemplateContext[startIdx:endIdx]); err != nil {
			return skerr.Wrapf(err, "failed to expand trace values template")
		}

		sql := b.String()
		sklog.Infof("About to write %d trace values with sql of length %d", len(params), len(sql))
		if _, err := s.db.Exec(ctx, sql); err != nil {
			return skerr.Wrapf(err, "Executing: %q", b.String())
		}
		return nil
	})

	if err != nil {
		return err
	}

	sklog.Info("Finished writing trace values.")

	return err
}

// Confirm that *SQLTraceStore fulfills the tracestore.TraceStore interface.
var _ tracestore.TraceStore = (*SQLTraceStore)(nil)
