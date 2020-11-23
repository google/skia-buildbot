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


To query for traces we first find the trace_ids of all the traces that would
match the given query on a tile.

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

Then once you have all the trace_ids, load the values from the TraceValues
table.

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

Look in migrations/cdb.sql for more example of raw queries using a simple
example dataset.
*/
package sqltracestore

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
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
	"go.skia.org/infra/perf/go/cache/memcached"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
	"golang.org/x/sync/errgroup"
)

// cacheMetricsRefreshDuration controls how often we update the metrics for
// in-memory caches.
const cacheMetricsRefreshDuration = 15 * time.Second

// CockroachDB can be sensitive to the number of VALUES in a single INSERT
// statement. These values were experimentally determined to be good when 6
// ingesters running 20 parallel Go routines were ingesting a large amout of
// data. Note that values over 200 caused the insert rate to drop precipitously,
// going from 20,000 qps with a batch size of 100 down to 400 qps with a batch
// size of 200.
const writeTracesChunkSize = 100

// See writeTracesChunkSize.
const readTracesChunkSize = 100

// queryTraceIDsChunkSize is the number of traceids we try to resolve into
// Params at one time.
const queryTraceIDsChunkSize = 1000

// Number of parallel requests sent to the database when servicing a single
// query. 30 matches the max number of cores we use on a clustering instance.
const poolSize = 30

const queryTracesIDOnlyByIndexChannelSize = 1000

// defaultCacheSize is the size of the in-memory LRU caches.
const defaultCacheSize = 40 * 1000 * 1000

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

// traceIDForSQLInBytes is the md5 hash of a trace name.
type traceIDForSQLInBytes [md5.Size]byte

// Calculates the traceIDForSQL for the given trace name, e.g. "\x00112233...".
// Note the \x prefix which tells CockroachDB that this is hex encoded.
func traceIDForSQLFromTraceName(traceName string) traceIDForSQL {
	b := md5.Sum([]byte(traceName))
	return traceIDForSQL(fmt.Sprintf("\\x%x", b))
}

func traceIDForSQLInBytesFromTraceName(traceName string) traceIDForSQLInBytes {
	return md5.Sum([]byte(traceName))
}

func traceIDForSQLFromTraceIDAsBytes(b []byte) traceIDForSQL {
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
	insertIntoTraceValues
	insertIntoPostings
	insertIntoParamSets
	getSourceFileID
	getLatestTile
	paramSetForTile
	getSource
	traceCount
	queryTraceIDs
	convertTraceIDs
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
	convertTraceIDs: `
        {{ $tileNumber := .TileNumber }}
        SELECT
            key_value, trace_id
        FROM
            Postings@by_trace_id
            {{ .AsOf }}
        WHERE
            tile_number = {{ $tileNumber }}
            AND trace_id IN (
                {{ range $index, $trace_id :=  .TraceIDs -}}
                    {{ if $index }},{{ end -}}
                    '{{ $trace_id }}'
                {{ end -}}
            )
        ORDER BY
            trace_id
    `,
	queryTraceIDs: `
        {{ $key := .Key }}
        SELECT
            trace_id
        FROM
            Postings
            {{ .AsOf }}
        WHERE
            tile_number = {{ .TileNumber }}
            AND key_value IN
            (
                {{ range $index, $value :=  .Values -}}
                    {{ if $index }},{{end}}
                    '{{ $key }}={{ $value }}'
                {{ end }}
            )
        ORDER BY trace_id`,
	readTraces: `
        SELECT
            trace_id,
            commit_number,
            val
        FROM
            TraceValues
            {{ .AsOf }}
        WHERE
            commit_number >= {{ .BeginCommitNumber }}
            AND commit_number <= {{ .EndCommitNumber }}
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
	paramSetForTile: `
        SELECT
           param_key, param_value
        FROM
            ParamSets
            {{ .AsOf }}
        WHERE
            tile_number = {{ .TileNumber }}`,
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

// queryTraceIDsContext is the context for the queryTraceIDsContext template.
type queryTraceIDsContext struct {
	TileNumber types.TileNumber
	Key        string
	Values     []string
	AsOf       string
}

// convertTracesContext is the context for the convertTraceIDs template.
type convertTraceIDsContext struct {
	TileNumber types.TileNumber
	TraceIDs   []traceIDForSQL
	AsOf       string
}

// readTracesContext is the context for the readTraces template.
type readTracesContext struct {
	BeginCommitNumber types.CommitNumber
	EndCommitNumber   types.CommitNumber
	TraceIDs          []traceIDForSQL
	AsOf              string
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

	// cacheKey is the key for this entry in the local LRU cache. It is not used
	// as part of the SQL template.
	cacheKey string
}

// insertIntoParamSetsContext is the context for the insertIntoParamSets template.
type insertIntoParamSetsContext struct {
	TileNumber types.TileNumber
	Key        string
	Value      string

	// cacheKey is the key for this entry in the local LRU cache. It is not used
	// as part of the SQL template.
	cacheKey string
}

// paramSetForTileContext is the context for the paramSetForTile template.
type paramSetForTileContext struct {
	TileNumber types.TileNumber
	AsOf       string
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
	traceCount: `
        SELECT
            COUNT(DISTINCT trace_id)
        FROM
            Postings
        WHERE
          tile_number = $1`,
}

type timeProvider func() time.Time

// Statement to add to enable follower reads.
// See https://www.cockroachlabs.com/docs/v20.1/as-of-system-time
// and https://www.cockroachlabs.com/docs/v20.1/follower-reads#run-queries-that-use-follower-reads
const followerReadsStatement = "AS OF SYSTEM TIME '-5s'"

// SQLTraceStore implements tracestore.TraceStore backed onto an SQL database.
type SQLTraceStore struct {
	// db is the SQL database instance.
	db *pgxpool.Pool

	// timeNow allows controlling time during tests.
	timeNow timeProvider

	// unpreparedStatements are parsed templates that can be used to construct SQL statements.
	unpreparedStatements map[statement]*template.Template

	// And from md5(trace_name)+tile_number -> true if the trace_name has
	// already been written to the Postings table.
	//
	// And from (tile_number, paramKey, paramValue) -> true if the param has
	// been written to the ParamSets tables.
	cache cache.Cache

	// orderedParamSetCache is a cache for OrderedParamSets that have a TTL. The
	// cache maps tileNumber -> orderedParamSetCacheEntry.
	orderedParamSetCache *lru.Cache

	// tileSize is the number of commits per Tile.
	tileSize int32

	// enableFollowerReads, if true, means older data in the database can be
	// used to respond to queries.
	// See https://www.cockroachlabs.com/docs/v20.1/as-of-system-time
	// and https://www.cockroachlabs.com/docs/v20.1/follower-reads#run-queries-that-use-follower-reads
	enableFollowerReads bool

	// metrics
	writeTracesMetric               metrics2.Float64SummaryMetric
	writeTracesMetricSQL            metrics2.Float64SummaryMetric
	buildTracesContextsMetric       metrics2.Float64SummaryMetric
	cacheMissMetric                 metrics2.Counter
	orderedParamSetsCacheMissMetric metrics2.Counter
	orderedParamSetCacheLen         metrics2.Int64Metric
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

	var cache cache.Cache
	var err error
	if len(datastoreConfig.CacheConfig.MemcachedServers) > 0 {
		cache, err = memcached.New(datastoreConfig.CacheConfig.MemcachedServers, datastoreConfig.CacheConfig.Namespace)
	} else {
		cache, err = local.New(defaultCacheSize)
	}
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to build cache.")
	}

	paramSetCache, err := lru.New(orderedParamSetCacheSize)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	ret := &SQLTraceStore{
		db:                              db,
		timeNow:                         time.Now,
		unpreparedStatements:            unpreparedStatements,
		tileSize:                        datastoreConfig.TileSize,
		cache:                           cache,
		orderedParamSetCache:            paramSetCache,
		enableFollowerReads:             datastoreConfig.EnableFollowerReads,
		writeTracesMetric:               metrics2.GetFloat64SummaryMetric("perfserver_sqltracestore_write_traces"),
		writeTracesMetricSQL:            metrics2.GetFloat64SummaryMetric("perfserver_sqltracestore_write_traces_sql"),
		buildTracesContextsMetric:       metrics2.GetFloat64SummaryMetric("perfserver_sqltracestore_build_traces_context"),
		cacheMissMetric:                 metrics2.GetCounter("perfserver_sqltracestore_cache_miss"),
		orderedParamSetsCacheMissMetric: metrics2.GetCounter("perfserver_sqltracestore_ordered_paramsets_cache_miss"),
		orderedParamSetCacheLen:         metrics2.GetInt64Metric("perfserver_sqltracestore_ordered_paramset_cache_len"),
	}

	// Update metrics periodically.
	go func() {
		for range time.Tick(cacheMetricsRefreshDuration) {
			ret.orderedParamSetCacheLen.Update(int64(ret.orderedParamSetCache.Len()))
			ctx := context.Background()
			tileNumber, err := ret.GetLatestTile(ctx)
			if err != nil {
				sklog.Errorf("Failed to load latest tile when calculating metrics: %s")
				continue
			}
			ret.updateParamSetMetricsForTile(ctx, tileNumber)
			ret.updateParamSetMetricsForTile(ctx, tileNumber-1)
		}
	}()

	return ret, nil
}

func (s *SQLTraceStore) updateParamSetMetricsForTile(ctx context.Context, tileNumber types.TileNumber) {
	ops, err := s.GetOrderedParamSet(ctx, tileNumber)
	if err != nil {
		sklog.Errorf("Failed to load ParamSet when calculating metrics: %s")
		return
	}
	metrics2.GetInt64Metric("perfserver_sqltracestore_paramset_size", map[string]string{"tileNumber": fmt.Sprintf("%d", tileNumber)}).Update(int64(ops.ParamSet.Size()))
}

// CommitNumberOfTileStart implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) CommitNumberOfTileStart(commitNumber types.CommitNumber) types.CommitNumber {
	tileNumber := types.TileNumberFromCommitNumber(commitNumber, s.tileSize)
	beginCommit, _ := types.TileCommitRangeForTileNumber(tileNumber, s.tileSize)
	return beginCommit
}

// GetLatestTile implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) GetLatestTile(ctx context.Context) (types.TileNumber, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.GetLatestTile")
	defer span.End()

	tileNumber := types.BadTileNumber
	if err := s.db.QueryRow(ctx, statements[getLatestTile]).Scan(&tileNumber); err != nil {
		return types.BadTileNumber, skerr.Wrap(err)
	}
	return tileNumber, nil
}

func (s *SQLTraceStore) paramSetForTile(ctx context.Context, tileNumber types.TileNumber) (paramtools.ParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.GetOrderedParamSet")
	defer span.End()

	defer timer.New(fmt.Sprintf("paramSetForTile-%d", tileNumber)).Stop()

	context := paramSetForTileContext{
		TileNumber: tileNumber,
		AsOf:       "",
	}

	if s.enableFollowerReads {
		context.AsOf = followerReadsStatement
	}

	// Expand the template for the SQL.
	var b bytes.Buffer
	if err := s.unpreparedStatements[paramSetForTile].Execute(&b, context); err != nil {
		return nil, skerr.Wrapf(err, "failed to expand paramSetForTile template")
	}
	sql := b.String()

	rows, err := s.db.Query(ctx, sql)
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
	ret.Normalize()
	if err == pgx.ErrNoRows {
		return ret, nil
	}
	if err := rows.Err(); err != nil {
		return nil, skerr.Wrapf(err, "Other failure - tileNumber=%d", tileNumber)
	}

	return ret, nil
}

// ClearOrderedParamSetCache is only used for tests.
func (s *SQLTraceStore) ClearOrderedParamSetCache() {
	s.orderedParamSetCache.Purge()
}

// GetOrderedParamSet implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) GetOrderedParamSet(ctx context.Context, tileNumber types.TileNumber) (*paramtools.OrderedParamSet, error) {
	defer timer.New("GetOrderedParamSet").Stop()
	ctx, span := trace.StartSpan(ctx, "sqltracestore.GetOrderedParamSet")
	defer span.End()

	now := s.timeNow()
	iEntry, ok := s.orderedParamSetCache.Get(tileNumber)
	if ok {
		if entry, ok := iEntry.(orderedParamSetCacheEntry); ok && entry.expires.After(now) {
			return entry.orderedParamSet, nil
		}
		_ = s.orderedParamSetCache.Remove(tileNumber)
	}
	ps, err := s.paramSetForTile(ctx, tileNumber)
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

// QueryTraces implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) QueryTraces(ctx context.Context, tileNumber types.TileNumber, q *query.Query) (types.TraceSet, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.QueryTraces")
	defer span.End()

	traceNames := make(chan string, queryTracesIDOnlyByIndexChannelSize)
	pChan, err := s.QueryTracesIDOnly(ctx, tileNumber, q)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get list of traceIDs matching query.")
	}

	// Start a Go routine that converts Params into a trace name and then feeds
	// those trace names into the traceNames channel.
	go func() {
		defer timer.New("QueryTracesIDOnly - Complete").Stop()
		for p := range pChan {
			traceName, err := query.MakeKey(p)
			if err != nil {
				sklog.Warningf("Invalid trace name found in query response: %s", err)
				continue
			}
			traceNames <- traceName
		}
		close(traceNames)
	}()

	beginCommit, endCommit := types.TileCommitRangeForTileNumber(tileNumber, s.tileSize)
	return s.readTracesByChannelForCommitRange(ctx, traceNames, beginCommit, endCommit)
}

// QueryTracesIDOnly implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) QueryTracesIDOnly(ctx context.Context, tileNumber types.TileNumber, q *query.Query) (<-chan paramtools.Params, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.QueryTracesIDOnly")
	defer span.End()

	defer timer.New("QueryTracesIDOnlyByIndex").Stop()
	outParams := make(chan paramtools.Params, queryTracesIDOnlyByIndexChannelSize)
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
		// that don't appear in a tile, which means the query won't work on this
		// tile, but it may still work on other tiles, so we just don't return
		// any results for this tile.
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

	// This query is done in two parts because the CDB query planner seems to
	// pick a really bad plan a large percentage of the time.

	// First find the encoded trace ids that match the query. Break apart the
	// QueryPlan and do each group of OR's as individual queries to the
	// database, but then stream the results and do the ANDs here on the server.
	// That's because CDB complains about the amount of RAM that doing the AND
	// can require. For example, the query 'source_type=svg&sub_result=min_ms'
	// requires merging two lists that are both over 200k.
	unionChannels := []<-chan traceIDForSQL{}
	i := 0
	for key, values := range plan {
		context := queryTraceIDsContext{
			TileNumber: tileNumber,
			Key:        key,
			Values:     values,
			AsOf:       "",
		}
		if s.enableFollowerReads {
			context.AsOf = followerReadsStatement
		}

		// Expand the template for the SQL.
		var b bytes.Buffer
		if err := s.unpreparedStatements[queryTraceIDs].Execute(&b, context); err != nil {
			return nil, skerr.Wrapf(err, "failed to expand queryTraceIDs template")
		}
		sql := b.String()
		rows, err := s.db.Query(ctx, sql)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		ch := make(chan traceIDForSQL)
		unionChannels = append(unionChannels, ch)

		go func(ch chan traceIDForSQL, rows pgx.Rows) {
			_, span := trace.StartSpan(ctx, "sqltracestore.QueryTracesIDOnly.PerKeyWorker")
			defer span.End()

			defer close(ch)

			for rows.Next() {
				var traceIDAsBytes []byte
				if err := rows.Scan(&traceIDAsBytes); err != nil {
					sklog.Errorf("Failed to scan traceIDAsBytes: %s", skerr.Wrap(err))
					return
				}
				if err := rows.Err(); err != nil {
					if err == pgx.ErrNoRows {
						return
					}
					sklog.Errorf("Failed while reading traceIDAsBytes: %s", skerr.Wrap(err))
					return
				}
				ch <- traceIDForSQLFromTraceIDAsBytes(traceIDAsBytes)
			}
		}(ch, rows)
		i++
	}

	// Now AND together the results of all the unionChannels.
	traceIDsCh := newIntersect(ctx, unionChannels)

	// And then convert the results of the AND into Params, which we sent out
	// the channel we'll return from this function.
	chunkChannel := make(chan []traceIDForSQL)
	go func() {
		defer close(outParams)

		// Start a pool of workers.
		g, ctx := errgroup.WithContext(ctx)
		for i := 0; i < poolSize; i++ {
			g.Go(func() error {
				ctx, span := trace.StartSpan(ctx, "sqltracestore.QueryTracesIDOnly.convertToParamsWorker")
				defer span.End()

				for chunk := range chunkChannel {
					if err := ctx.Err(); err != nil {
						return skerr.Wrap(err)
					}
					if err := s.convertHexTraceIdsToParams(ctx, chunk, tileNumber, outParams); err != nil {
						// Drain the channel before returning the err.
						for range chunkChannel {
						}
						return skerr.Wrap(err)
					}
				}
				return nil
			})
		}

		// Now feed the pool of workers chunks to resolve.
		currentChunk := []traceIDForSQL{}
		for hexEncodedTraceID := range traceIDsCh {
			currentChunk = append(currentChunk, hexEncodedTraceID)
			if len(currentChunk) >= queryTraceIDsChunkSize {
				chunkChannel <- currentChunk
				currentChunk = []traceIDForSQL{}
			}
		}
		// Now handle any remaining values in the currentChunk.
		chunkChannel <- currentChunk

		close(chunkChannel)
		if err := g.Wait(); err != nil {
			sklog.Error(err)
		}
	}()

	return outParams, nil
}

// convertHexTraceIdsToParams queries the database for all the key=value pairs
// that make up the currentBatch of traceIDs, build Params for each trace id,
// and then emits those Params on the 'outParams' channel.
func (s *SQLTraceStore) convertHexTraceIdsToParams(ctx context.Context, currentBatch []traceIDForSQL, tileNumber types.TileNumber, outParams chan paramtools.Params) error {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.QueryTracesIDOnly.convertHexTraceIdsToParams")
	defer span.End()

	context := convertTraceIDsContext{
		TileNumber: tileNumber,
		TraceIDs:   currentBatch,
		AsOf:       "",
	}
	if s.enableFollowerReads {
		context.AsOf = followerReadsStatement
	}

	// Expand the template for the SQL.
	var b bytes.Buffer
	if err := s.unpreparedStatements[convertTraceIDs].Execute(&b, context); err != nil {
		return skerr.Wrapf(err, "failed to expand convertTraceIDs template")
	}
	sql := b.String()
	rows, err := s.db.Query(ctx, sql)
	if err != nil {
		return skerr.Wrapf(err, "SQL: %q", sql)
	}

	// We build up the Params for each matching trace id row-by-row. That
	// is, each row contains the trace_id of a trace that matches the query,
	// and a single key=value pair from the trace name.

	// As we scan we keep track of the current trace_id we are on, since the
	// query returns rows ordered by trace_id we know they are all grouped
	// together.
	var currentTraceID []byte
	p := paramtools.Params{}
	for rows.Next() {
		var keyValue string
		var traceIDAsBytes []byte
		if err := rows.Scan(&keyValue, &traceIDAsBytes); err != nil {
			sklog.Errorf("Failed to scan traceName: %s", skerr.Wrap(err))
			return err
		}
		// If we hit a new trace_id then emit the current Params we have
		// built so far and start on a fresh Params.
		if !bytes.Equal(currentTraceID, traceIDAsBytes) {
			// Don't emit on the first row, i.e. before we've actually done
			// any work.
			if currentTraceID != nil {
				outParams <- p
			} else {
				currentTraceID = make([]byte, len(traceIDAsBytes))
			}
			p = paramtools.Params{}
			copy(currentTraceID, traceIDAsBytes)
		}

		// Add to the current Params.
		parts := strings.SplitN(keyValue, "=", 2)
		if len(parts) != 2 {
			sklog.Warningf("Found invalid key=value pair in Postings: %q", keyValue)
			continue
		}
		p[parts[0]] = parts[1]
	}
	if err := rows.Err(); err != nil && err != pgx.ErrNoRows {
		return skerr.Wrapf(err, "Failed while reading traceNames")
	}

	// Make sure to emit the last trace id.
	if currentTraceID != nil {
		outParams <- p
	}

	return nil
}

// ReadTraces implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) ReadTraces(ctx context.Context, tileNumber types.TileNumber, traceNames []string) (types.TraceSet, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.ReadTraces")
	defer span.End()

	defer timer.New("ReadTraces").Stop()

	beginCommit, endCommit := types.TileCommitRangeForTileNumber(tileNumber, s.tileSize)
	return s.ReadTracesForCommitRange(ctx, traceNames, beginCommit, endCommit)
}

// ReadTracesForCommitRange implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) ReadTracesForCommitRange(ctx context.Context, traceNames []string, beginCommit types.CommitNumber, endCommit types.CommitNumber) (types.TraceSet, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.ReadTracesForCommitRange")
	defer span.End()

	defer timer.New("ReadTraces").Stop()

	traceNamesChannel := make(chan string, len(traceNames))

	for _, traceName := range traceNames {
		traceNamesChannel <- traceName
	}
	close(traceNamesChannel)

	return s.readTracesByChannelForCommitRange(ctx, traceNamesChannel, beginCommit, endCommit)
}

// readTracesByChannelForCommitRange reads the traceNames from a channel so we
// don't have to wait for the full list of trace ids to be ready first.
//
// It works by reading in a number of traceNames into a chunk and then passing
// that chunk of trace names to a worker pool that reads all the trace values
// for the given trace names.
func (s *SQLTraceStore) readTracesByChannelForCommitRange(ctx context.Context, traceNames <-chan string, beginCommit types.CommitNumber, endCommit types.CommitNumber) (types.TraceSet, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.readTracesByChannelForCommitRange")
	defer span.End()

	// The return value, protected by mutex.
	ret := types.TraceSet{}

	// Validate the begin and end commit numbers.
	if beginCommit > endCommit {
		// Empty the traceNames channel.
		for range traceNames {
		}
		return nil, skerr.Fmt("Invalid commit range, [%d, %d] should be [%d, %d]", beginCommit, endCommit, endCommit, beginCommit)
	}

	// How long should the traces be in the response?
	traceLength := endCommit - beginCommit + 1

	// Map from the [md5.Size]byte representation of a trace id to the trace name.
	//
	// Protected by mutex.
	traceNameMap := map[traceIDForSQLInBytes]string{}

	// Protects traceNameMap and ret.
	var mutex sync.Mutex

	// chunkChannel is used to distribute work to the workers.
	chunkChannel := make(chan []traceIDForSQL, queryTracesIDOnlyByIndexChannelSize)

	// Start the workers that do the actual querying when given chunks of trace ids.
	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < poolSize; i++ {
		g.Go(func() error {
			ctx, span := trace.StartSpan(ctx, "sqltracestore.ReadTraces.Worker")
			defer span.End()

			for chunk := range chunkChannel {
				if err := s.readTracesChunk(ctx, beginCommit, endCommit, chunk, &mutex, traceNameMap, &ret); err != nil {
					return skerr.Wrap(err)
				}
			}
			return nil
		})
	}

	// Now break up the incoming trace ids into chuck for the workers.
	currentChunk := []traceIDForSQL{}
	for key := range traceNames {
		if !query.ValidateKey(key) {
			sklog.Errorf("Invalid key: %q", key)
			continue
		}

		mutex.Lock()
		// Make space in ret for the values.
		ret[key] = vec32.New(int(traceLength))

		// Update the map from the full name of the trace and id in traceIDForSQLInBytes form.
		traceNameMap[traceIDForSQLInBytesFromTraceName(key)] = key
		mutex.Unlock()

		trID := traceIDForSQLFromTraceName(key)
		currentChunk = append(currentChunk, trID)
		if len(currentChunk) > queryTraceIDsChunkSize {
			chunkChannel <- currentChunk
			currentChunk = []traceIDForSQL{}
		}
	}
	// Now handle any remaining values in the currentChunk.
	if len(currentChunk) >= 0 {
		chunkChannel <- currentChunk
	}
	close(chunkChannel)

	if err := g.Wait(); err != nil {
		span.SetStatus(trace.Status{
			Code:    trace.StatusCodeInternal,
			Message: err.Error(),
		})
		// Empty the traceNames channel.
		for range traceNames {
		}
		return nil, skerr.Wrap(err)
	}

	return ret, nil
}

// readTracesChunk updates the passed in TraceSet with all the values loaded for
// the given slice of trace ids.
//
// The mutex protects 'ret' and 'traceNameMap'.
func (s *SQLTraceStore) readTracesChunk(ctx context.Context, beginCommit types.CommitNumber, endCommit types.CommitNumber, chunk []traceIDForSQL, mutex *sync.Mutex, traceNameMap map[traceIDForSQLInBytes]string, ret *types.TraceSet) error {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.ReadTraces.Chunk")
	span.AddAttributes(trace.Int64Attribute("chunk_length", int64(len(chunk))))
	defer span.End()
	// Populate the context for the SQL template.
	readTracesContext := readTracesContext{
		BeginCommitNumber: beginCommit,
		EndCommitNumber:   endCommit,
		TraceIDs:          chunk,
		AsOf:              "",
	}
	if s.enableFollowerReads {
		readTracesContext.AsOf = followerReadsStatement
	}

	// Expand the template for the SQL.
	var b bytes.Buffer
	if err := s.unpreparedStatements[readTraces].Execute(&b, readTracesContext); err != nil {
		return skerr.Wrapf(err, "failed to expand readTraces template")
	}

	sql := b.String()
	// Execute the query.
	rows, err := s.db.Query(ctx, sql)
	if err != nil {
		return skerr.Wrapf(err, "SQL: %q", sql)
	}

	var traceIDArray traceIDForSQLInBytes
	for rows.Next() {
		var traceIDInBytes []byte
		var commitNumber types.CommitNumber
		var val float64
		if err := rows.Scan(&traceIDInBytes, &commitNumber, &val); err != nil {
			return skerr.Wrap(err)
		}

		if err != nil {
			sklog.Warningf("Invalid trace name found in query response: %s", err)
			continue
		}
		// pgx can't Scan into an array, but Go can't use a slice as a map key, so
		// we Scan into a byte slice and then copy into a byte array to use
		// as the index into the map.
		copy(traceIDArray[:], traceIDInBytes)
		mutex.Lock()
		(*ret)[traceNameMap[traceIDArray]][commitNumber-beginCommit] = float32(val)
		mutex.Unlock()
	}
	if err := rows.Err(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
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
	ctx, span := trace.StartSpan(ctx, "sqltracestore.TraceCount")
	defer span.End()

	var ret int64
	err := s.db.QueryRow(ctx, statements[traceCount], tileNumber).Scan(&ret)
	span.AddAttributes(trace.Int64Attribute("count", ret))
	return ret, skerr.Wrap(err)
}

// updateSourceFile writes the filename into the SourceFiles table and returns
// the sourceFileIDFromSQL of that filename.
func (s *SQLTraceStore) updateSourceFile(ctx context.Context, filename string) (sourceFileIDFromSQL, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.updateSourceFile")
	defer span.End()

	ret := badSourceFileIDFromSQL
	_, err := s.db.Exec(ctx, statements[insertIntoSourceFiles], filename)
	if err != nil {
		return ret, skerr.Wrap(err)
	}
	err = s.db.QueryRow(ctx, statements[getSourceFileID], filename).Scan(&ret)
	if err != nil {
		return ret, skerr.Wrap(err)
	}

	return ret, nil
}

func cacheKeyForPostings(tileNumber types.TileNumber, traceID traceIDForSQL) string {
	return fmt.Sprintf("%d-%s", tileNumber, traceID)
}

func cacheKeyForParamSets(tileNumber types.TileNumber, paramKey, paramValue string) string {
	return fmt.Sprintf("%d-%q-%q", tileNumber, paramKey, paramValue)
}

// WriteTraces implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) WriteTraces(ctx context.Context, commitNumber types.CommitNumber, params []paramtools.Params, values []float32, ps paramtools.ParamSet, source string, _ time.Time) error {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.WriteTraces")
	defer span.End()

	defer timer.NewWithSummary("perfserver_sqltracestore_write_traces", s.writeTracesMetric).Stop()

	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	tileNumber := s.TileNumber(commitNumber)

	// Write ParamSet.
	paramSetsContext := []insertIntoParamSetsContext{}
	for paramKey, paramValues := range ps {
		for _, paramValue := range paramValues {
			cacheKey := cacheKeyForParamSets(tileNumber, paramKey, paramValue)
			if !s.cache.Exists(cacheKey) {
				s.cacheMissMetric.Inc(1)
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
		err := util.ChunkIter(len(paramSetsContext), writeTracesChunkSize, func(startIdx int, endIdx int) error {
			chunk := paramSetsContext[startIdx:endIdx]
			var b bytes.Buffer
			if err := s.unpreparedStatements[insertIntoParamSets].Execute(&b, chunk); err != nil {
				return skerr.Wrapf(err, "failed to expand paramsets template in slice [%d, %d]", startIdx, endIdx)
			}

			sql := b.String()

			sklog.Infof("About to write %d paramset entries with sql of length %d", endIdx-startIdx, len(sql))
			if _, err := s.db.Exec(ctx, sql); err != nil {
				return skerr.Wrapf(err, "Executing: %q", b.String())
			}
			for _, ele := range chunk {
				s.cache.Add(ele.cacheKey)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	// Write the source file entry and the id.
	sourceID, err := s.updateSourceFile(ctx, source)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Build the 'context's which will be used to populate the SQL templates for
	// the TraceValues and Postings tables.
	t := timer.NewWithSummary("perfserver_sqltracestore_build_traces_contexts", s.buildTracesContextsMetric)
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

		cacheKey := cacheKeyForPostings(tileNumber, traceID)
		if !s.cache.Exists(cacheKey) {
			s.cacheMissMetric.Inc(1)
			for paramKey, paramValue := range p {
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
	defer timer.NewWithSummary("perfserver_sqltracestore_write_traces_sql_insert", s.writeTracesMetricSQL).Stop()
	sklog.Infof("About to format %d postings names", len(params))

	if len(postingsTemplateContext) > 0 {
		err := util.ChunkIter(len(postingsTemplateContext), writeTracesChunkSize, func(startIdx int, endIdx int) error {
			var b bytes.Buffer
			if err := s.unpreparedStatements[insertIntoPostings].Execute(&b, postingsTemplateContext[startIdx:endIdx]); err != nil {
				return skerr.Wrapf(err, "failed to expand postings template on slice [%d, %d]", startIdx, endIdx)
			}
			sql := b.String()

			if _, err := s.db.Exec(ctx, sql); err != nil {
				return skerr.Wrapf(err, "Executing: %q", b.String())
			}
			return nil
		})

		if err != nil {
			return err
		}

		for _, entry := range postingsTemplateContext {
			s.cache.Add(entry.cacheKey)
		}
	}

	sklog.Infof("About to format %d trace values", len(params))

	err = util.ChunkIter(len(valuesTemplateContext), writeTracesChunkSize, func(startIdx int, endIdx int) error {
		var b bytes.Buffer
		if err := s.unpreparedStatements[insertIntoTraceValues].Execute(&b, valuesTemplateContext[startIdx:endIdx]); err != nil {
			return skerr.Wrapf(err, "failed to expand trace values template")
		}

		sql := b.String()
		if _, err := s.db.Exec(ctx, sql); err != nil {
			return skerr.Wrapf(err, "Executing: %q", sql)
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
