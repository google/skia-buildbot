/*
Package sqltracestore implements a tracestore.TraceStore on top of SQL. We'll
look that the SQL schema used to explain how SQLTraceStore maps traces into an
SQL database.

We store the name of every source file that has been ingested in the SourceFiles
table so we can use the shorter 64 bit source_file_id in other tables.

    SourceFiles (
        source_file_id INT PRIMARY KEY DEFAULT unique_rowid(),
        source_file TEXT UNIQUE NOT NULL
    )

Each trace name, which is a structured key (See /infra/go/query) of the
form,key1=value1,key2=value2,..., is stored either as the md5 hash of the trace
name, i.e. trace_id = md5(trace_name) or as the series of key=value pairs that
make up the params of the key.

When we store the values of each trace in the TraceValues table, use the
trace_id and the commit_number as the primary key. We also store not only the
value but the id of the source file that the value came from.

    CREATE TABLE IF NOT EXISTS TraceValues (
        trace_id BYTES,
        -- Id of the trace name from TraceIDS.
        commit_number INT,
        -- A types.CommitNumber.
        val REAL,
        -- The floating point measurement.
        source_file_id INT,
        -- Id of the source filename, from SourceFiles.
        PRIMARY KEY (trace_id, commit_number)
    );

Just using this table we can construct some useful queries. For example we can
count the number of traces in a single tile, in this case the 0th tile in a
system with a tileSize of 256:

    SELECT
        COUNT(DISTINCT trace_id)
    FROM
        TraceValues
    WHERE
        commit_number >= 0 AND commit_number < 256;

The Postings table is our inverted index for looking up which trace ids contain
which key=value pairs. For a good introduction to postings and search
https://www.tbray.org/ongoing/When/200x/2003/06/18/HowSearchWorks is a good
resource.

Remember that each trace name is a structured key of the
form,arch=x86,config=8888,..., and that over time traces may come and go, i.e.
we may stop running a test, or start running new tests, so if we want to make
searching for traces efficient we need to be aware of how those trace ids change
over time. The answer is to break our store in Tiles, i.e. blocks of commits of
tileSize length, and then for each Tile we keep an inverted index of the trace
ids.

In the table below we store a key_value which is the literal "key=value" part of
a trace name, along with the tile_number and the md5 trace_id. Note that
tile_number is just int(commitNumber/tileSize).

    CREATE TABLE IF NOT EXISTS Postings (
        -- A types.TileNumber.
        tile_number INT,
        -- A key value pair from a structured key, e.g. "config=8888".
        key_value STRING NOT NULL,
        -- md5(trace_name)
        trace_id BYTES,
        PRIMARY KEY (tile_number, key_value, trace_id)
    );

Finally, to make it fast to turn UI queries into SQL queries we store the
ParamSet representing all the trace names in the Tile.

    CREATE TABLE IF NOT EXISTS ParamSets (
        tile_number INT,
        param_key STRING,
        param_value STRING,
        PRIMARY KEY (tile_number, param_key, param_value),
        INDEX (tile_number DESC),
    );

So for example to build a ParamSet for a tile:

    SELECT
        param_key, param_value
    FROM
        ParamSets
    WHERE
        tile_number=0;

To find the most recent tile:

    SELECT
        tile_number
    FROM
        ParamSets
    ORDER BY
        tile_number DESC LIMIT 1;


Finally to query for traces we first find the trace_ids
of all the traces that would match the given query on a tile.

	SELECT
	    encode(trace_id, 'hex')
	FROM
	    Postings
	WHERE
	    key_value IN ('config=8888', 'config=565')
	    AND tile_number = 0
	INTERSECT
	SELECT
	    encode(trace_id, 'hex')
	FROM
	    Postings
	WHERE
	    key_value IN ('arch=x86', 'arch=risc-v')
	    AND tile_number = 0;

Then once you have all the trace_ids, load the values
from the TraceValues table.

	SELECT
	    trace_id,
	    commit_number,
	    val
	FROM
	    TraceValues
	WHERE
	    tracevalues.commit_number >= 0
	    AND tracevalues.commit_number < 256
	    AND tracevalues.trace_id IN (
	        '\xfe385b159ff55dca481069805e5ff050',
	        '\x277262a9236d571883d47dab102070bc'
	    );

Look in migrations/cdb.sql for more example of raw queries using a simple example dataset.
*/
package sqltracestore

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"

	lru "github.com/hashicorp/golang-lru"
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

// timeNow allows controlling time during tests.
var timeNow = time.Now

const writeTracesChunkSize = 100

// defaultCacheSize is the size of the in-memory LRU cache if no size was
// specified in the config file.
const defaultCacheSize = 20 * 1000 * 1000

const orderedParamSetCacheSize = 100

const orderedParamSetCacheTTL = 5 * time.Minute

type orderedParamSetCacheEntry struct {
	expires         time.Time // When this entry expires.
	orderedParamSet *paramtools.OrderedParamSet
}

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

func traceIDForSQLFromTraceIDAsBytes(traceNameBytes []byte) traceIDForSQL {
	return traceIDForSQL(fmt.Sprintf("\\x%x", traceNameBytes))
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
	insertIntoTraceValues
	insertIntoPostings
	insertIntoParamSets
	getSourceFileID
	getLatestTile
	paramSetForTile
	getSource
	traceCount
	queryTracesIDOnly
	readTraces
)

var templates = map[statement]string{
	insertIntoTraceValues: `INSERT INTO
            TraceValues (trace_id, commit_number, val, source_file_id)
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
	queryTracesIDOnly: `
		{{ $tileNumber := .TileNumber }}
        SELECT
			key_value, trace_id
        FROM
            Postings@by_trace_id
        WHERE
			tile_number = {{ $tileNumber }}
			AND trace_id IN (
			{{ range $index, $element := .QueryPlan }}
				{{ if $index }} INTERSECT {{ end }}
				SELECT
					trace_id
				FROM
					Postings
				WHERE
					tile_number = {{ $tileNumber }}
					AND key_value IN
                	(
                    	{{ range $index, $value :=  $element.Values -}}
							{{ if $index }},{{end}}
							'{{ $element.Key }}={{ $value }}'
                    	{{ end }}
                	)
            {{ end }}
			)
		ORDER BY
			trace_id`,
	readTraces: `
        SELECT
            trace_id,
            commit_number,
            val
        FROM
            TraceValues
        WHERE
            commit_number >= {{ .BeginCommitNumber }}
            AND commit_number < {{ .EndCommitNumber }}
            AND trace_id IN
            (
                {{ range $index, $trace_id :=  .TraceIDs -}}
					{{ if $index }},{{end}}
					'{{ $trace_id }}'
                {{ end }}
            )
        `,
	getSource: `
        SELECT
            SourceFiles.source_file
        FROM
            TraceValues
        INNER LOOKUP JOIN SourceFiles ON SourceFiles.source_file_id = TraceValues.source_file_id
        WHERE
            TraceValues.trace_id = '{{ .MD5HexTraceID }}'
            AND TraceValues.commit_number = {{ .CommitNumber }}`,
	insertIntoPostings: `
        INSERT INTO
            Postings (tile_number, key_value, trace_id)
        VALUES
            {{ range $index, $element :=  . -}}
                {{ if $index }},{{end}}
                ( {{ $element.TileNumber }}, '{{ $element.Key }}={{ $element.Value }}', '{{ $element.MD5HexTraceID }}' )
            {{ end }}
        ON CONFLICT
        DO NOTHING`,
	insertIntoParamSets: `
        INSERT INTO
            ParamSets (tile_number, param_key, param_value)
        VALUES
            {{ range $index, $element :=  . -}}
                {{ if $index }},{{end}}
                ( {{ $element.TileNumber }}, '{{ $element.Key }}', '{{ $element.Value }}' )
            {{ end }}
        ON CONFLICT
        DO NOTHING`,
}

// replaceTraceValuesContext is the context for the replaceTraceValues template.
type insertIntoTraceValuesContext struct {
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

// queryPlanContext is used in queryTracesContext.
type queryPlanContext struct {
	Key    string
	Values []string
}

// queryTracesContext is the context for the queryTraces template.
type queryTracesContext struct {
	TileNumber types.TileNumber
	QueryPlan  []queryPlanContext
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

// insertIntoTilesContext is the context for the insertIntoTiles template.
type insertIntoPostingsContext struct {
	TileNumber types.TileNumber

	// Key is a Params key.
	Key string

	// Value is the value for the Params key above.
	Value string

	// The MD5 sum of the trace name as a hex string, i.e.
	// "\xfe385b159ff55dca481069805e5ff050". Note the leading \x which
	// CockroachDB will use to know the string is in hex.
	MD5HexTraceID traceIDForSQL

	// cacheKey is the key for this entry in the local LRU cache.
	cacheKey string
}

type insertIntoParamSetsContext struct {
	TileNumber types.TileNumber
	Key        string
	Value      string

	// cacheKey is the key for this entry in the local LRU cache.
	cacheKey string
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
	getLatestTile: `
		SELECT
    		tile_number
		FROM
    		ParamSets@by_tile_number
		ORDER BY
    		tile_number DESC
		LIMIT
    		1;`,
	paramSetForTile: `
        SELECT
           param_key, param_value
        FROM
            ParamSets
        WHERE
            tile_number = $1`,
	traceCount: `
        SELECT
            COUNT(DISTINCT trace_id)
        FROM
            TraceValues
        WHERE
          commit_number >= $1 AND commit_number <= $2`,
}

// SQLTraceStore implements tracestore.TraceStore backed onto an SQL database.
type SQLTraceStore struct {
	// db is the SQL database instance.
	db *pgxpool.Pool

	// unpreparedStatements are parsed templates that can be used to construct SQL statements.
	unpreparedStatements map[statement]*template.Template

	// A cache from md5(trace_name) -> true if the trace_name has already been
	// written to the TraceNames table.
	//
	// And from md5(trace_name)+tile_number -> true if the trace_name has
	// already been written to the Tiles table.
	//
	// And from (tile_number, param_key, param_value) -> true if the param has
	// been written to the ParamSets tables.
	cache cache.Cache

	// orderedParamSetCache is a cache for OrderedParamSets that have a TTL. The
	// cache maps tileNumber -> orderedParamSetCacheEntry.
	orderedParamSetCache *lru.Cache

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
	paramSetCache, err := lru.New(orderedParamSetCacheSize)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &SQLTraceStore{
		db:                        db,
		unpreparedStatements:      unpreparedStatements,
		tileSize:                  datastoreConfig.TileSize,
		cache:                     cache,
		orderedParamSetCache:      paramSetCache,
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
	defer timer.New("GetLatestTile").Stop()
	tileNumber := types.BadTileNumber
	if err := s.db.QueryRow(context.TODO(), statements[getLatestTile]).Scan(&tileNumber); err != nil {
		return types.BadTileNumber, skerr.Wrap(err)
	}
	return tileNumber, nil
}

func (s *SQLTraceStore) paramSetForTile(tileNumber types.TileNumber) (paramtools.ParamSet, error) {
	defer timer.New("GetOrderedParamSet").Stop()

	rows, err := s.db.Query(context.TODO(), statements[paramSetForTile], tileNumber)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed querying - tileNumber=%d", tileNumber)
	}
	ret := paramtools.NewParamSet()
	for rows.Next() {
		var key string
		var value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, skerr.Wrapf(err, "Failed scanning row - tileNumber=%d", tileNumber)
		}
		ret.AddParams(paramtools.Params{key: value})
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

// ClearOrderedParamSetCache is only used for tests.
func (s *SQLTraceStore) ClearOrderedParamSetCache() {
	s.orderedParamSetCache.Purge()
}

// GetOrderedParamSet implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) GetOrderedParamSet(ctx context.Context, tileNumber types.TileNumber) (*paramtools.OrderedParamSet, error) {

	now := timeNow()
	iEntry, ok := s.orderedParamSetCache.Get(tileNumber)
	if ok {
		entry := iEntry.(orderedParamSetCacheEntry)
		if entry.expires.After(now) {
			return entry.orderedParamSet, nil
		}
		_ = s.orderedParamSetCache.Remove(tileNumber)
	}
	ps, err := s.paramSetForTile(tileNumber)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	keys := ps.Keys()
	sort.Strings(keys)
	ret := &paramtools.OrderedParamSet{
		ParamSet: ps,
		KeyOrder: keys,
	}

	_ = s.orderedParamSetCache.Add(tileNumber, orderedParamSetCacheEntry{
		expires:         now.Add(orderedParamSetCacheTTL),
		orderedParamSet: ret,
	})

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
	traceNames := []string{}
	pChan, err := s.QueryTracesIDOnlyByIndex(ctx, tileNumber, q)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get list of traceIDs matching query.")
	}

	t := timer.New("QueryTracesIDOnlyByIndex - Complete")
	for p := range pChan {
		traceName, err := query.MakeKey(p)
		if err != nil {
			sklog.Warningf("Invalid trace name found in query response: %s", err)
			continue
		}
		traceNames = append(traceNames, traceName)
	}
	t.Stop()
	return s.ReadTraces(tileNumber, traceNames)
}

// QueryTracesIDOnlyByIndex implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) QueryTracesIDOnlyByIndex(ctx context.Context, tileNumber types.TileNumber, q *query.Query) (<-chan paramtools.Params, error) {
	defer timer.New("QueryTracesIDOnlyByIndex").Stop()
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
	context := queryTracesContext{
		TileNumber: tileNumber,
		QueryPlan:  []queryPlanContext{},
	}

	for key, values := range plan {
		context.QueryPlan = append(context.QueryPlan, queryPlanContext{
			Key:    key,
			Values: values,
		})
	}

	// Expand the template for the SQL.
	var b bytes.Buffer
	if err := s.unpreparedStatements[queryTracesIDOnly].Execute(&b, context); err != nil {
		return nil, skerr.Wrapf(err, "failed to expand trace names template")
	}

	sql := b.String()
	fmt.Println(sql)
	// Execute the query.
	rows, err := s.db.Query(ctx, sql)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	go func() {
		defer close(outParams)

		p := paramtools.Params{}

		// We build up the Params for each matching trace id row-by-row. That
		// is, each row contains the trace_id of a trace that matches the query,
		// and a single key=value pair from the trace name.

		// As we scan we keep track of the current trace_id we are on, since the
		// query returns rows ordered by trace_id we know they are all grouped
		// together.
		currentTraceID := badTraceIDFromSQL
		for rows.Next() {
			var keyValue string
			var traceIDAsBytes []byte
			if err := rows.Scan(&keyValue, &traceIDAsBytes); err != nil {
				sklog.Errorf("Failed to scan traceName: %s", skerr.Wrap(err))
				return
			}
			nextTraceID := traceIDForSQLFromTraceIDAsBytes(traceIDAsBytes)
			// If we hit a new trace_id then emit the current Params we have
			// built so far and start on a fresh Params.
			if currentTraceID != nextTraceID {
				// Don't emit on the first row, i.e. before we've actually done
				// any work.
				if currentTraceID != badTraceIDFromSQL {
					outParams <- p
				}
				p = paramtools.Params{}
				currentTraceID = nextTraceID
			}

			// Add to the current Params.
			parts := strings.SplitN(keyValue, "=", 2)
			if len(parts) != 2 {
				sklog.Warningf("Found invalid key=value pair in Postings: %q", keyValue)
				continue
			}
			p[parts[0]] = parts[1]
		}
		if err := rows.Err(); err != nil {
			if err == pgx.ErrNoRows {
				return
			}
			sklog.Errorf("Failed while reading traceNames: %s", skerr.Wrap(err))
			return
		}
		// Make sure to emit the last trace id.
		if currentTraceID != badTraceIDFromSQL {
			outParams <- p
		}
	}()

	return outParams, nil
}

// ReadTraces implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) ReadTraces(tileNumber types.TileNumber, traceNames []string) (types.TraceSet, error) {
	defer timer.New("ReadTraces").Stop()
	// TODO(jcgregorio) Should be broken into batches so we don't exceed the SQL
	// engine limit on query sizes.
	ret := types.TraceSet{}

	// Get the traceIDs for the given keys.
	beginCommit, endCommit := types.TileCommitRangeForTileNumber(tileNumber, s.tileSize)

	// Populate the context for the SQL template.
	readTracesContext := readTracesContext{
		BeginCommitNumber: beginCommit,
		EndCommitNumber:   endCommit,
		TraceIDs:          make([]traceIDForSQL, 0, len(traceNames)),
	}

	// TODO(jcgregorio) Use []byte instead of traceIDForSQL.
	traceNameMap := map[traceIDForSQL]string{}
	for _, key := range traceNames {
		if !query.ValidateKey(key) {
			return nil, skerr.Fmt("Invalid key stored in shortcut: %q", key)
		}

		// TODO(jcgregorio) Replace this vec32.New() with a
		// https://golang.org/pkg/sync/#Pool since this is our most used/reused
		// type of memory.
		ret[key] = vec32.New(int(s.tileSize))
		traceID := traceIDForSQLFromTraceName(key)
		traceNameMap[traceID] = key
		readTracesContext.TraceIDs = append(readTracesContext.TraceIDs, traceID)
	}

	// Expand the template for the SQL.
	var b bytes.Buffer
	if err := s.unpreparedStatements[readTraces].Execute(&b, readTracesContext); err != nil {
		return nil, skerr.Wrapf(err, "failed to expand readTraces template")
	}

	sql := b.String()
	// Execute the query.
	rows, err := s.db.Query(context.TODO(), sql)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	for rows.Next() {
		var traceIDInBytes []byte
		var commitNumber types.CommitNumber
		var val float64
		// Does pgx convert the []byte into the '\x....' format?
		if err := rows.Scan(&traceIDInBytes, &commitNumber, &val); err != nil {
			return nil, skerr.Wrap(err)
		}

		traceID := traceIDForSQLFromTraceIDAsBytes(traceIDInBytes)

		if err != nil {
			sklog.Warningf("Invalid trace name found in query response: %s", err)
			continue
		}
		ret[traceNameMap[traceID]][s.OffsetFromCommitNumber(commitNumber)] = float32(val)
	}
	if err == pgx.ErrNoRows {
		return ret, nil
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

func cacheKeyForPostings(tileNumber types.TileNumber, paramKey string, paramValue string, traceID traceIDForSQL) string {
	return fmt.Sprintf("%d-%s-%s%s", tileNumber, traceID, paramKey, paramValue)
}

func cacheKeyForParamSets(tileNumber types.TileNumber, key, value string) string {
	return fmt.Sprintf("%d-%q-%q", tileNumber, key, value)
}

// WriteTraces implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) WriteTraces(commitNumber types.CommitNumber, params []paramtools.Params, values []float32, ps paramtools.ParamSet, source string, _ time.Time) error {
	defer timer.NewWithSummary("perfserver_sqltracestore_writeTraces", s.writeTracesMetric).Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	tileNumber := s.TileNumber(commitNumber)

	// Write ParamSet.
	paramSetsContext := []insertIntoParamSetsContext{}
	for paramKey, paramValues := range ps {
		for _, paramValue := range paramValues {
			cacheKey := cacheKeyForParamSets(tileNumber, paramKey, paramValue)
			if !s.cache.Exists(cacheKey) {
				paramSetsContext = append(paramSetsContext, insertIntoParamSetsContext{
					TileNumber: tileNumber,
					Key:        paramKey,
					Value:      paramValue,
					cacheKey:   cacheKey,
				})
			}
		}
	}

	if len(paramSetsContext) > 0 {
		var b bytes.Buffer
		if err := s.unpreparedStatements[insertIntoParamSets].Execute(&b, paramSetsContext); err != nil {
			return skerr.Wrapf(err, "failed to expand paramsets template")
		}

		sql := b.String()

		sklog.Infof("About to write %d paramset entries with sql of length %d", len(paramSetsContext), len(sql))
		if _, err := s.db.Exec(ctx, sql); err != nil {
			return skerr.Wrapf(err, "Executing: %q", b.String())
		}
		for _, ele := range paramSetsContext {
			s.cache.Add(ele.cacheKey, "1")
		}
	}

	// Write the source file entry and the id.
	sourceID, err := s.updateSourceFile(source)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Build the 'context's which will be used to populate the SQL templates for
	// the TraceValues and Postings tables.
	t := timer.NewWithSummary("perfserver_sqltracestore_buildTracesContexts", s.buildTracesContextsMetric)
	valuesTemplateContext := make([]insertIntoTraceValuesContext, 0, len(params))
	postingsTemplateContext := []insertIntoPostingsContext{} // We have no idea how long this will be.

	for i, p := range params {
		traceName, err := query.MakeKey(p)
		if err != nil {
			continue
		}
		traceID := traceIDForSQLFromTraceName(traceName)
		valuesTemplateContext = append(valuesTemplateContext, insertIntoTraceValuesContext{
			MD5HexTraceID: traceID,
			CommitNumber:  commitNumber,
			Val:           values[i],
			SourceFileID:  sourceID,
		})

		for paramKey, paramValue := range p {
			cacheKey := cacheKeyForPostings(tileNumber, paramKey, paramValue, traceID)
			if !s.cache.Exists(cacheKey) {
				postingsTemplateContext = append(postingsTemplateContext, insertIntoPostingsContext{
					TileNumber:    tileNumber,
					Key:           paramKey,
					Value:         paramValue,
					MD5HexTraceID: traceID,
					cacheKey:      cacheKey,
				})
			}
		}
	}
	t.Stop()

	// Now that the contexts are built, execute the SQL in batches.
	defer timer.NewWithSummary("perfserver_sqltracestore_writeTraces_sql", s.writeTracesMetricSQL).Stop()
	sklog.Infof("About to format %d trace names", len(params))

	if len(postingsTemplateContext) > 0 {
		err := util.ChunkIter(len(postingsTemplateContext), writeTracesChunkSize, func(startIdx int, endIdx int) error {
			var b bytes.Buffer
			if err := s.unpreparedStatements[insertIntoPostings].Execute(&b, postingsTemplateContext[startIdx:endIdx]); err != nil {
				return skerr.Wrapf(err, "failed to expand tiles template on slice [%d, %d]", startIdx, endIdx)
			}
			sql := b.String()

			sklog.Infof("About to write %d tiles with sql of length %d", endIdx-startIdx, len(sql))
			if _, err := s.db.Exec(ctx, sql); err != nil {
				return skerr.Wrapf(err, "Executing: %q", b.String())
			}
			return nil
		})

		if err != nil {
			return err
		}

		for _, entry := range postingsTemplateContext {
			s.cache.Add(entry.cacheKey, "1")
		}
	}

	sklog.Infof("About to format %d trace values", len(params))

	err = util.ChunkIter(len(valuesTemplateContext), writeTracesChunkSize, func(startIdx int, endIdx int) error {
		var b bytes.Buffer
		if err := s.unpreparedStatements[insertIntoTraceValues].Execute(&b, valuesTemplateContext[startIdx:endIdx]); err != nil {
			return skerr.Wrapf(err, "failed to expand trace values template")
		}

		sql := b.String()
		sklog.Infof("About to write %d trace values with sql of length %d", endIdx-startIdx, len(sql))
		if _, err := s.db.Exec(ctx, sql); err != nil {
			return skerr.Wrapf(err, "Executing: %q", b.String())
		}
		return nil
	})

	if err != nil {
		return err
	}

	sklog.Info("Finished writing trace values.")

	return nil
}

// Confirm that *SQLTraceStore fulfills the tracestore.TraceStore interface.
var _ tracestore.TraceStore = (*SQLTraceStore)(nil)
