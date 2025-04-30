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
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/cache/local"
	"go.skia.org/infra/go/cache/memcached"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/tracecache"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
	"golang.org/x/sync/errgroup"
)

const (
	// cacheMetricsRefreshDuration controls how often we update the metrics for
	// in-memory caches.
	cacheMetricsRefreshDuration = 15 * time.Second

	// CockroachDB can be sensitive to the number of VALUES in a single INSERT
	// statement. These values were experimentally determined to be good when 6
	// ingesters running 20 parallel Go routines were ingesting a large amout of
	// data. Note that values over 200 caused the insert rate to drop precipitously,
	// going from 20,000 qps with a batch size of 100 down to 400 qps with a batch
	// size of 200.
	writeTracesValuesChunkSize    = 100
	writeTracesPostingsChunkSize  = 100
	writeTracesParamSetsChunkSize = 100

	// queryTracesChunkSize is the number of traces we try to read trace values for
	// at a time.
	queryTracesChunkSize = 10000

	// queryTraceParamsChunkSize is the number of traces we try to convert into
	// params in a single request.
	queryTraceParamsChunkSize = 2000

	// Number of parallel requests sent to the database when servicing a single
	// query. 30 matches the max number of cores we use on a clustering instance.
	poolSize = 30

	queryTracesIDOnlyByIndexChannelSize = 10000

	// defaultCacheSize is the size of the in-memory LRU caches.
	defaultCacheSize = 40 * 1000 * 1000

	orderedParamSetCacheSize = 100

	orderedParamSetCacheTTL = 5 * time.Minute

	// Keep this small. Queries that have a small number of matches will return
	// quickly and those are the important queries.
	countingQueryDuration = 5 * time.Second

	// Max number of trace_ids to add to a query to speed it up. This is a rough
	// guess, some testing should be done to validate the right size for this
	// const.
	countOptimizationThreshold = 10000

	// Max no of traceIds to store in a single cache entry.
	maxTraceIdsInCache = 200
)

type orderedParamSetCacheEntry struct {
	expires  time.Time // When this entry expires.
	paramSet paramtools.ReadOnlyParamSet
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
	insertIntoTraceValues2
	insertIntoPostings
	insertIntoParamSets
	getSourceFileID
	getLatestTile
	paramSetForTile
	getSource
	traceCount
	queryTraceIDs
	queryTraceIDsByKeyValue
	readTraces
	getLastNSources
	getTraceIDsBySource
	countMatchingTraces
	restrictClause
	deleteCommit
	countCommitInCommitNumberRange
	getCommitsFromCommitNumberRange
)

var templates = map[statement]string{
	insertIntoTraceValues: `UPSERT INTO
            TraceValues (trace_id, commit_number, val, source_file_id)
        VALUES
        {{ range $index, $element :=  . -}}
            {{ if $index }},{{end}}
            (
                '{{ $element.MD5HexTraceID }}', {{ $element.CommitNumber }}, {{ $element.Val }}, {{ $element.SourceFileID }}
            )
        {{ end }}
        `,
	insertIntoTraceValues2: `UPSERT INTO
            TraceValues2 (trace_id, commit_number, val, source_file_id, benchmark, bot, test, subtest_1, subtest_2, subtest_3)
        VALUES
        {{ range $index, $element :=  . -}}
            {{ if $index }},{{end}}
            (
                '{{ $element.MD5HexTraceID }}', {{ $element.CommitNumber }}, {{ $element.Val }}, {{ $element.SourceFileID }},
				'{{ $element.Benchmark }}', '{{ $element.Bot }}', '{{ $element.Test }}', '{{ $element.Subtest_1 }}',
				'{{ $element.Subtest_2 }}', '{{ $element.Subtest_3 }}'
            )
        {{ end }}
        `,
	queryTraceIDs: `
        {{ $key := .Key }}
        SELECT
            trace_id
        FROM
            Postings@primary
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
            {{ .RestrictClause }}
		ORDER BY trace_id`,
	queryTraceIDsByKeyValue: `
		{{ $key := .Key }}
		SELECT
			trace_id
		FROM
			Postings@by_key_value
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
	countMatchingTraces: `
        {{ $key := .Key }}
        SELECT
            count(*)
        FROM (
            SELECT
               *
            FROM
               Postings
            WHERE
               tile_number = {{ .TileNumber }}
               AND key_value IN
               (
                  {{ range $index, $value :=  .Values -}}
                     {{ if $index }},{{end}}
                     '{{ $key }}={{ $value }}'
                  {{ end }}
               )
            LIMIT {{ .CountOptimizationThreshold }}
        )`,
	restrictClause: `
    AND trace_ID IN
    ({{ range $index, $value := .Values -}}
            {{ if $index }},{{end}}
            '{{ $value }}'
    {{ end }})`,
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

// replaceTraceValuesContext is the context for the replaceTraceValues2 template.
type insertIntoTraceValuesContext2 struct {
	// The MD5 sum of the trace name as a hex string, i.e.
	// "\xfe385b159ff55dca481069805e5ff050". Note the leading \x which
	// CockroachDB will use to know the string is in hex.
	MD5HexTraceID traceIDForSQL

	CommitNumber types.CommitNumber
	Val          float32
	SourceFileID sourceFileIDFromSQL
	Benchmark    string
	Bot          string
	Test         string
	Subtest_1    string
	Subtest_2    string
	Subtest_3    string
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
	TileNumber     types.TileNumber
	Key            string
	Values         []string
	AsOf           string
	RestrictClause string
}

// queryTraceIDsByKeyValueContext is the context for the queryTraceIDsByKeyValueContext template.
type queryTraceIDsByKeyValueContext struct {
	TileNumber types.TileNumber
	Key        string
	Values     []string
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

// countMatchingTraces is the context for the countMatchingTraces template.
type countMatchingTracesContext struct {
	TileNumber                 types.TileNumber
	Key                        string
	Values                     []string
	AsOf                       string
	CountOptimizationThreshold int64
}

// restrictClauseContext is the context for the restrictClause template.
type restrictClauseContext struct {
	Key    string
	Values []traceIDForSQL
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
	getLastNSources: `
        SELECT
            SourceFiles.source_file, TraceValues.commit_number
        FROM
            TraceValues@primary
            INNER LOOKUP JOIN
                SourceFiles@primary
            ON
                TraceValues.source_file_id = SourceFiles.source_file_id
        WHERE
            TraceValues.trace_id=$1
        ORDER BY
            TraceValues.commit_number DESC
        LIMIT
            $2`,
	getTraceIDsBySource: `
        SELECT
            Postings.key_value, Postings.trace_id
        FROM
            SourceFiles@by_source_file
            INNER LOOKUP JOIN
                TraceValues@by_source_file_id
            ON
                TraceValues.source_file_id = SourceFiles.source_file_id
            INNER LOOKUP JOIN
                Postings@by_trace_id
            ON
                TraceValues.trace_id = Postings.trace_id
        WHERE
            SourceFiles.source_file = $1
        AND
            Postings.tile_number= $2
        ORDER BY
            Postings.trace_id`,
	countCommitInCommitNumberRange: `
		SELECT
			count(*)
		FROM
			Commits
		WHERE
			commit_number >= $1
			AND commit_number <= $2`,
	getCommitsFromCommitNumberRange: `
		SELECT
			commit_number, git_hash, commit_time, author, subject
		FROM
			Commits
		WHERE
			commit_number >= $1
			AND commit_number <= $2
		ORDER BY
			commit_number ASC
		`,
	deleteCommit: `
		DELETE FROM
			Commits
		WHERE
			commit_number = $1
		`,
}

type timeProvider func() time.Time

// Statement to add to enable follower reads.
// See https://www.cockroachlabs.com/docs/v20.1/as-of-system-time
// and https://www.cockroachlabs.com/docs/v20.1/follower-reads#run-queries-that-use-follower-reads
const followerReadsStatement = "AS OF SYSTEM TIME '-5s'"

// SQLTraceStore implements tracestore.TraceStore backed onto an SQL database.
type SQLTraceStore struct {
	// db is the SQL database instance.
	db pool.Pool

	// unpreparedStatements are parsed templates that can be used to construct SQL statements.
	unpreparedStatements map[statement]*template.Template

	// statements are already constructed SQL statements.
	statements map[statement]string

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

	// This is set to true if the datastore is Spanner.
	isSpanner bool

	traceParamStore tracestore.TraceParamStore

	// metrics
	writeTracesMetric                      metrics2.Float64SummaryMetric
	writeTracesMetricSQL                   metrics2.Float64SummaryMetric
	buildTracesContextsMetric              metrics2.Float64SummaryMetric
	cacheMissMetric                        metrics2.Counter
	orderedParamSetsCacheMissMetric        metrics2.Counter
	queryUsesRestrictClause                metrics2.Counter
	queryRestrictionMinKeyInPlan           metrics2.Float64SummaryMetric
	orderedParamSetCacheLen                metrics2.Int64Metric
	commitSliceFromCommitNumberRangeCalled metrics2.Counter
}

// New returns a new *SQLTraceStore.
//
// We presume all migrations have been run against db before this function is
// called.
func New(db pool.Pool, datastoreConfig config.DataStoreConfig, traceParamStore tracestore.TraceParamStore) (*SQLTraceStore, error) {
	unpreparedStatements := map[statement]*template.Template{}
	queryTemplates := templates
	if datastoreConfig.DataStoreType == config.SpannerDataStoreType {
		statements = spannerStatements
		queryTemplates = spannerTemplates
	}
	for key, tmpl := range queryTemplates {
		t, err := template.New("").Parse(tmpl)
		if err != nil {
			return nil, skerr.Wrapf(err, "parsing template %v, %q", key, tmpl)
		}
		unpreparedStatements[key] = t
	}

	var cache cache.Cache
	var err error
	if datastoreConfig.CacheConfig != nil && len(datastoreConfig.CacheConfig.MemcachedServers) > 0 {
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
		db:                                     db,
		unpreparedStatements:                   unpreparedStatements,
		statements:                             statements,
		tileSize:                               datastoreConfig.TileSize,
		cache:                                  cache,
		orderedParamSetCache:                   paramSetCache,
		enableFollowerReads:                    datastoreConfig.EnableFollowerReads,
		isSpanner:                              datastoreConfig.DataStoreType == config.SpannerDataStoreType,
		traceParamStore:                        traceParamStore,
		writeTracesMetric:                      metrics2.GetFloat64SummaryMetric("perfserver_sqltracestore_write_traces"),
		writeTracesMetricSQL:                   metrics2.GetFloat64SummaryMetric("perfserver_sqltracestore_write_traces_sql"),
		buildTracesContextsMetric:              metrics2.GetFloat64SummaryMetric("perfserver_sqltracestore_build_traces_context"),
		cacheMissMetric:                        metrics2.GetCounter("perfserver_sqltracestore_cache_miss"),
		queryUsesRestrictClause:                metrics2.GetCounter("perfserver_sqltracestore_restrict_clause_used"),
		orderedParamSetsCacheMissMetric:        metrics2.GetCounter("perfserver_sqltracestore_ordered_paramsets_cache_miss"),
		queryRestrictionMinKeyInPlan:           metrics2.GetFloat64SummaryMetric("perfserver_sqltracestore_min_key_in_plan"),
		orderedParamSetCacheLen:                metrics2.GetInt64Metric("perfserver_sqltracestore_ordered_paramset_cache_len"),
		commitSliceFromCommitNumberRangeCalled: metrics2.GetCounter("perfserver_sqltracestore_commit_slice_from_commit_number_range_called"),
	}

	return ret, nil
}

// StartBackgroundMetricsGathering runs continuously in the background and gathers
// metrics related to param sets in the database.
func (s *SQLTraceStore) StartBackgroundMetricsGathering() {
	for range time.Tick(cacheMetricsRefreshDuration) {
		s.orderedParamSetCacheLen.Update(int64(s.orderedParamSetCache.Len()))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		tileNumber, err := s.GetLatestTile(ctx)
		if err != nil {
			sklog.Errorf("Failed to load latest tile when calculating metrics: %s", err)
			cancel()
			continue
		}
		s.updateParamSetMetricsForTile(ctx, tileNumber)
		s.updateParamSetMetricsForTile(ctx, tileNumber-1)
		cancel()
	}
}

func (s *SQLTraceStore) updateParamSetMetricsForTile(ctx context.Context, tileNumber types.TileNumber) {
	ps, err := s.GetParamSet(ctx, tileNumber)
	if err != nil {
		sklog.Errorf("Failed to load ParamSet when calculating metrics: %s")
		return
	}
	metrics2.GetInt64Metric("perfserver_sqltracestore_paramset_size", map[string]string{"tileNumber": fmt.Sprintf("%d", tileNumber)}).Update(int64(ps.Size()))
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
	if err := s.db.QueryRow(ctx, s.statements[getLatestTile]).Scan(&tileNumber); err != nil {
		return types.BadTileNumber, skerr.Wrap(err)
	}
	return tileNumber, nil
}

func (s *SQLTraceStore) paramSetForTile(ctx context.Context, tileNumber types.TileNumber) (paramtools.ReadOnlyParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.GetParamSet")
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
	ps := paramtools.NewParamSet()
	for rows.Next() {
		var key string
		var value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, skerr.Wrapf(err, "Failed scanning row - tileNumber=%d", tileNumber)
		}
		// This is safe because the paramsets table enforces uniqueness already
		ps[key] = append(ps[key], value)
	}
	ps.Normalize()
	ret := ps.Freeze()
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

// GetParamSet implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) GetParamSet(ctx context.Context, tileNumber types.TileNumber) (paramtools.ReadOnlyParamSet, error) {
	defer timer.New("GetParamSet").Stop()
	ctx, span := trace.StartSpan(ctx, "sqltracestore.GetParamSet")
	defer span.End()

	now := now.Now(ctx)
	iEntry, ok := s.orderedParamSetCache.Get(tileNumber)
	if ok {
		if entry, ok := iEntry.(orderedParamSetCacheEntry); ok && entry.expires.After(now) {
			return entry.paramSet, nil
		}
		_ = s.orderedParamSetCache.Remove(tileNumber)
	}
	ps, err := s.paramSetForTile(ctx, tileNumber)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	_ = s.orderedParamSetCache.Add(tileNumber, orderedParamSetCacheEntry{
		expires:  now.Add(orderedParamSetCacheTTL),
		paramSet: ps,
	})

	return ps, nil
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

// GetLastNSources implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) GetLastNSources(ctx context.Context, traceID string, n int) ([]tracestore.Source, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.GetLastNSources")
	defer span.End()

	traceIDAsBytes := traceIDForSQLInBytesFromTraceName(traceID)
	rows, err := s.db.Query(ctx, s.statements[getLastNSources], traceIDAsBytes[:], n)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed for traceID=%q and n=%d", traceID, n)
	}

	ret := []tracestore.Source{}
	for rows.Next() {
		var filename string
		var commitNumber types.CommitNumber
		if err := rows.Scan(&filename, &commitNumber); err != nil {
			return nil, skerr.Wrapf(err, "Failed scanning for traceID=%q and n=%d", traceID, n)
		}
		ret = append(ret, tracestore.Source{
			Filename:     filename,
			CommitNumber: commitNumber,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, skerr.Wrap(err)
	}
	return ret, nil
}

// GetTraceIDsBySource implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) GetTraceIDsBySource(ctx context.Context, sourceFilename string, tileNumber types.TileNumber) ([]string, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.GetTraceIDsBySource")
	defer span.End()

	rows, err := s.db.Query(ctx, s.statements[getTraceIDsBySource], sourceFilename, tileNumber)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed for sourceFilename=%q and tileNumber=%d", sourceFilename, tileNumber)
	}

	// We queried the Postings table, build up each traceid from all the
	// key=value pairs returned.
	var currentTraceIDAsBytes []byte
	p := paramtools.Params{}
	ret := []string{}
	for rows.Next() {
		var keyValue string
		var traceIDAsBytes []byte
		if err := rows.Scan(&keyValue, &traceIDAsBytes); err != nil {
			return nil, skerr.Wrapf(err, "Failed scanning for sourceFilename=%q and tileNumber=%d", sourceFilename, tileNumber)
		}
		// If we hit a new trace_id then we have a complete traceID.
		if !bytes.Equal(currentTraceIDAsBytes, traceIDAsBytes) {
			if currentTraceIDAsBytes == nil {
				// This is the first time going through this loop.
				currentTraceIDAsBytes = make([]byte, len(traceIDAsBytes))
			} else {
				// Since traceIDAsBytes changed we are done building up the
				// params for the traceID, so convert the params into a string.
				traceID, err := query.MakeKey(p)
				if err != nil {
					return nil, skerr.Wrap(err)
				}
				ret = append(ret, traceID)
			}
			p = paramtools.Params{}
			copy(currentTraceIDAsBytes, traceIDAsBytes)
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
		return nil, skerr.Wrap(err)
	}

	// Make sure to get the last trace id.
	if currentTraceIDAsBytes != nil {
		traceID, err := query.MakeKey(p)
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		ret = append(ret, traceID)
	}

	return ret, nil
}

// countCommitInCommitNumberRange counts the number of commits in a given commit number range.
func (s *SQLTraceStore) countCommitInCommitNumberRange(ctx context.Context, begin, end types.CommitNumber) (int, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.countCommitInCommitNumberRange")
	defer span.End()

	var count int
	if err := s.db.QueryRow(ctx, s.statements[countCommitInCommitNumberRange], begin, end).Scan(&count); err != nil {
		return 0, skerr.Wrap(err)
	}
	return count, nil
}

// OffsetFromCommitNumber implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) OffsetFromCommitNumber(commitNumber types.CommitNumber) int32 {
	return int32(commitNumber) % s.tileSize
}

// QueryTraces implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) QueryTraces(ctx context.Context, tileNumber types.TileNumber, q *query.Query, traceCache *tracecache.TraceCache) (types.TraceSet, []provider.Commit, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.QueryTraces")
	defer span.End()

	traceNames := make(chan string, queryTracesIDOnlyByIndexChannelSize)

	var pChan <-chan paramtools.Params
	var err error
	cacheTraceIds := false
	if traceCache != nil {
		sklog.Infof("Trace cache is enabled.")
		pChan, err = s.getTraceIdChannelFromCache(ctx, traceCache, tileNumber, q)
		if err != nil {
			// If there is an error getting data from cache, log it and fall back to the regular db search.
			sklog.Infof("Error retrieving trace id params from cache %v. Falling back to db search.", err)
			pChan, err = s.QueryTracesIDOnly(ctx, tileNumber, q)
			cacheTraceIds = true
		}
	} else {
		pChan, err = s.QueryTracesIDOnly(ctx, tileNumber, q)
	}

	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Failed to get list of traceIDs matching query.")
	}

	// Start a Go routine that converts Params into a trace name and then feeds
	// those trace names into the traceNames channel.
	go func() {
		defer timer.New("QueryTracesIDOnly - Complete").Stop()
		traceIdsToCache := []paramtools.Params{}
		for p := range pChan {
			if cacheTraceIds {
				traceIdsToCache = append(traceIdsToCache, p)
			}
			traceName, err := query.MakeKey(p)
			if err != nil {
				sklog.Warningf("Invalid trace name found in query response: %s", err)
				continue
			}
			traceNames <- traceName
		}
		close(traceNames)
		if cacheTraceIds && len(traceIdsToCache) > 0 && len(traceIdsToCache) <= maxTraceIdsInCache {
			sklog.Infof("Adding %d traceIds to the cache for query %v", len(traceIdsToCache), q)
			err := traceCache.CacheTraceIds(ctx, tileNumber, q, traceIdsToCache)
			if err != nil {
				// Log the error and continue.
				sklog.Errorf("Error adding traceIds to the cache for query %v: %v", q, err)
			}
		}
	}()

	beginCommit, endCommit := types.TileCommitRangeForTileNumber(tileNumber, s.tileSize)
	return s.readTracesByChannelForCommitRange(ctx, traceNames, beginCommit, endCommit)
}

// planCount is used in restrictByCounting to find how many traces match each
// part of the query plan.
type planCount struct {
	key    string
	values []string
	count  int64
}

// planCountSlice is a slice of planCounts, that is sortable, since we want to
// find the key in the plan with the smallest number of matches.
type planCountSlice []*planCount

func (p planCountSlice) Len() int { return len(p) }
func (p planCountSlice) Less(i, j int) bool {
	if p[i].count == p[j].count {
		return strings.Compare(p[i].key, p[j].key) == -1
	}
	return p[i].count < p[j].count
}
func (p planCountSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// planDisposition encodes the disposition of the plan, i.e. is it still worth
// running, or can it be skipped.
type planDisposition int

const (
	skippable planDisposition = iota
	runnable
)

// restrictByCounting analyzes the query plan buy running each part of the plan
// under a count(*) query, and then returing the key of the part of the query
// with the smallest number of matches.
//
// An AND clause to be appended to a WHERE clause is returned that contains all
// the IDs of the part of the plan with the smallest number of matches, along
// with the name of the key that had the smallest number of matches, and the
// disposition of the plan.
//
// If the count queries take too long, or all the keys return too many matches,
// then both the returned clause and key name will be the empty string.
func (s *SQLTraceStore) restrictByCounting(ctx context.Context, tileNumber types.TileNumber, plan paramtools.ParamSet) (string, string, planDisposition) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.restrictByCounting")
	defer span.End()

	if len(plan) < 2 {
		return "", "", runnable
	}

	ctx, cancel := context.WithTimeout(ctx, countingQueryDuration)
	defer cancel()

	// mutex protects planCounts and planDisposition.
	var mutex sync.Mutex
	// TODO(jcgregorio) To speed this up even more we should have an LRU cache
	// with a timeout that caches (key,values,tileNumber) -> count. The value
	// shouldn't change much and we don't need an exact count, only an
	// approximation.
	planCounts := make([]*planCount, 0, len(plan))
	planDisposition := runnable

	// For each key in the plan run a separate Go routine that counts how many
	// traces in the tile match that query, storing the results for each key in
	// planCounts.
	var wg sync.WaitGroup
	for key, values := range plan {
		wg.Add(1)
		go func(key string, values []string) {
			defer wg.Done()
			context := countMatchingTracesContext{
				TileNumber:                 tileNumber,
				Key:                        key,
				Values:                     values,
				AsOf:                       "",
				CountOptimizationThreshold: countOptimizationThreshold,
			}
			if s.enableFollowerReads {
				context.AsOf = followerReadsStatement
			}

			// Expand the template for the SQL.
			var b bytes.Buffer
			if err := s.unpreparedStatements[countMatchingTraces].Execute(&b, context); err != nil {
				sklog.Warningf("failed to expand countMatchingTraces template: %s", err)
				return
			}
			sql := b.String()
			row := s.db.QueryRow(ctx, sql)
			var count int64
			if err := row.Scan(&count); err != nil {
				sklog.Warningf("failed to retrieve count in countMatchingTraces: %s", err)
				return
			}
			if count == 0 {
				// There are no traces that match this part of the query, so we
				// know no traces will match the entire query, so set our
				// disposition to 'skippable' so the full query is never run.
				mutex.Lock()
				defer mutex.Unlock()
				planDisposition = skippable
				return
			}
			mutex.Lock()
			defer mutex.Unlock()
			planCounts = append(planCounts, &planCount{
				key:    key,
				values: values,
				count:  count,
			})
		}(key, values)
	}
	wg.Wait()
	if planDisposition == skippable {
		return "", "", skippable
	}
	if len(planCounts) == 0 {
		return "", "", runnable
	}

	sort.Sort(planCountSlice(planCounts))

	// optimal is the key with the smallest number of matches to the plan.
	optimal := planCounts[0]

	s.queryRestrictionMinKeyInPlan.Observe(float64(optimal.count))
	span.AddAttributes(trace.Float64Attribute("minKeyInPlan", float64(optimal.count)))

	// We want to avoid create too large of an "AND IN ()" clause, so if there
	// are too many matches for the optimal key then just skip the restrict
	// clause completely.
	if optimal.count >= countOptimizationThreshold {
		return "", "", runnable
	}

	// Now that we know the key in the plan with the smallest number of matching
	// trace_ids, we can go back to the database and query for all those
	// matching trace_ids.
	context := queryTraceIDsByKeyValueContext{
		TileNumber: tileNumber,
		Key:        optimal.key,
		Values:     optimal.values,
		AsOf:       "",
	}
	if s.enableFollowerReads {
		context.AsOf = followerReadsStatement
	}

	// Expand the template for the SQL.
	var b bytes.Buffer
	if err := s.unpreparedStatements[queryTraceIDsByKeyValue].Execute(&b, context); err != nil {
		sklog.Warningf("Failed to expand queryTraceIDsByKeyValue template: %s", err)
		return "", "", runnable

	}
	sql := b.String()
	rows, err := s.db.Query(ctx, sql)
	if err != nil {
		return "", "", runnable
	}

	ids := make([]traceIDForSQL, 0, optimal.count)
	for rows.Next() {
		var traceIDAsBytes []byte
		if err := rows.Scan(&traceIDAsBytes); err != nil {
			sklog.Errorf("Failed to scan traceIDAsBytes: %s", skerr.Wrap(err))
			return "", "", runnable
		}
		if err := rows.Err(); err != nil {
			if err == pgx.ErrNoRows {
				return "", "", runnable
			}
			sklog.Errorf("Failed while reading traceIDAsBytes: %s", skerr.Wrap(err))
			return "", "", runnable
		}
		ids = append(ids, traceIDForSQLFromTraceIDAsBytes(traceIDAsBytes))
	}

	// Now format the matching trace_ids for the optimal key into an "AND
	// trace_id IN (...)" clause to speed up all the other queries in the plan.
	b.Reset()
	err = s.unpreparedStatements[restrictClause].Execute(&b, restrictClauseContext{
		Key:    optimal.key,
		Values: ids,
	})
	if err != nil {
		sklog.Errorf("Failed to expand the restrictClause template: %s", err)
		return "", "", runnable
	}
	s.queryUsesRestrictClause.Inc(1)
	return b.String(), optimal.key, runnable
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

	ps, err := s.GetParamSet(ctx, tileNumber)
	if err != nil {
		close(outParams)
		return outParams, skerr.Wrap(err)
	}

	plan, err := q.QueryPlan(ps)
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
		sklog.Info("QueryPlan returns an empty list")
		close(outParams)
		return outParams, nil
	}

	// Sanitize our inputs.
	if err := query.ValidateParamSet(plan); err != nil {
		return nil, skerr.Wrapf(err, "invalid query %#v", *q)
	}

	// Do a quick pre-flight to find if we can add a "AND trace_id IN (...)"
	// clause to all query parts to speed them up.
	traceIDRestriction, skipKey, planDisposition := s.restrictByCounting(ctx, tileNumber, plan)
	if planDisposition == skippable {
		// We know this query won't match any traces in this tile.
		sklog.Info("restrictByCounting returns an skippable planDisposition")
		close(outParams)
		return outParams, nil
	}

	optimizeSQLTraceStore := config.Config != nil && config.Config.OptimizeSQLTraceStore

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
		// If we are using a restrict clause then all the trace_ids for that key
		// are included all the other queries, so we can skip querying on that
		// key directly.
		if key == skipKey {
			continue
		}

		// Expand the template for the SQL.
		var b bytes.Buffer
		// Query trace ids through index by_key_value if the traceIDRestriction is empty,
		// otherwise, query trace ids through the primary key, which will reduce the SQL query statement time.
		if len(traceIDRestriction) == 0 {
			context := queryTraceIDsByKeyValueContext{
				TileNumber: tileNumber,
				Key:        key,
				Values:     values,
				AsOf:       "",
			}
			if s.enableFollowerReads {
				context.AsOf = followerReadsStatement
			}
			if err := s.unpreparedStatements[queryTraceIDsByKeyValue].Execute(&b, context); err != nil {
				return nil, skerr.Wrapf(err, "failed to expand queryTraceIDsByKeyValue template")
			}
		} else {
			context := queryTraceIDsContext{
				TileNumber:     tileNumber,
				Key:            key,
				Values:         values,
				AsOf:           "",
				RestrictClause: traceIDRestriction,
			}
			if s.enableFollowerReads {
				context.AsOf = followerReadsStatement
			}
			if err := s.unpreparedStatements[queryTraceIDs].Execute(&b, context); err != nil {
				return nil, skerr.Wrapf(err, "failed to expand queryTraceIDs template")
			}
		}
		sql := b.String()
		queryCtx, querySpan := trace.StartSpan(ctx, "sqltracestore.QueryTracesIDOnly.ExecuteSQLQuery")
		rows, err := s.db.Query(queryCtx, sql)
		querySpan.End()
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		ch := make(chan traceIDForSQL)
		if optimizeSQLTraceStore {
			ch = make(chan traceIDForSQL, queryTracesIDOnlyByIndexChannelSize)
		}
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

	// traceIDsCh supplies the relevant trace ids matching the query.
	// Now let's collect the unique traceIds from the channel and then get the params
	// for those traces.
	traceIdsMap := map[traceIDForSQL]bool{}
	uniqueTraceIds := []string{}
	for hexEncodedTraceID := range traceIDsCh {
		if _, ok := traceIdsMap[hexEncodedTraceID]; !ok {
			uniqueTraceIds = append(uniqueTraceIds, string(hexEncodedTraceID))
		}
		traceIdsMap[hexEncodedTraceID] = true
	}
	// Populate the outParams channel with the params for the traceIds.
	err = s.populateParamsForTraces(ctx, uniqueTraceIds, outParams)
	if err != nil {
		sklog.Errorf("Error converting traceIds to params: %v", err)
		return outParams, err
	}

	return outParams, nil
}

// populateParamsForTraces reads the params for the hex encoded traceIds and posts them on
// the outParams channel.
func (s *SQLTraceStore) populateParamsForTraces(ctx context.Context, traceIds []string, outParams chan paramtools.Params) error {
	// The goroutine below handles reading of the trace params in chunks.
	// We run this in a separate thread so that the upstream code that reads the outParams channel
	// can start reading this information without having to wait for all the traceIds to be completely read.
	go func() {
		ctx, span := trace.StartSpan(ctx, "sqltracestore.ReadTraceParams")
		defer span.End()

		// Close the channel when this goroutine completes signalling end of data.
		defer close(outParams)

		span.AddAttributes(trace.Int64Attribute("trace_count", int64(len(traceIds))))
		err := util.ChunkIterParallelPool(ctx, len(traceIds), queryTraceParamsChunkSize, poolSize, func(ctx context.Context, startIdx, endIdx int) error {
			traceIdChunk := traceIds[startIdx:endIdx]
			params, err := s.traceParamStore.ReadParams(ctx, traceIdChunk)
			if err != nil {
				sklog.Errorf("Error reading params:%v", err)
				return err
			}

			// Report the params for the current chunk.
			for _, param := range params {
				outParams <- param
			}

			return nil
		})
		if err != nil {
			sklog.Errorf("Error retrieving trace ids: %v", err)
		}
	}()
	return nil
}

// ReadTraces implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) ReadTraces(ctx context.Context, tileNumber types.TileNumber, traceNames []string) (types.TraceSet, []provider.Commit, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.ReadTraces")
	defer span.End()

	defer timer.New("ReadTraces").Stop()

	beginCommit, endCommit := types.TileCommitRangeForTileNumber(tileNumber, s.tileSize)
	return s.ReadTracesForCommitRange(ctx, traceNames, beginCommit, endCommit)
}

// ReadTracesForCommitRange implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) ReadTracesForCommitRange(ctx context.Context, traceNames []string, beginCommit types.CommitNumber, endCommit types.CommitNumber) (types.TraceSet, []provider.Commit, error) {
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
func (s *SQLTraceStore) readTracesByChannelForCommitRange(ctx context.Context, traceNames <-chan string, beginCommit types.CommitNumber, endCommit types.CommitNumber) (types.TraceSet, []provider.Commit, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.readTracesByChannelForCommitRange")
	defer span.End()

	// The return value, protected by mutex.
	ret := types.TraceSet{}

	// Validate the begin and end commit numbers.
	if beginCommit > endCommit {
		// Empty the traceNames channel.
		for range traceNames {
		}
		return nil, nil, skerr.Fmt("Invalid commit range, [%d, %d] should be [%d, %d]", beginCommit, endCommit, endCommit, beginCommit)
	}

	commits, err := s.commitSliceFromCommitNumberRange(ctx, beginCommit, endCommit)
	if err != nil {
		return nil, nil, skerr.Fmt("Cannot count commit within the commit range, [%d, %d]", beginCommit, endCommit)
	}

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
				if err := s.readTracesChunk(ctx, beginCommit, endCommit, commits, chunk, &mutex, traceNameMap, &ret); err != nil {
					return skerr.Wrap(err)
				}
			}
			return nil
		})
	}

	// Now break up the incoming trace ids into chuck for the workers.
	currentChunk := []traceIDForSQL{}
	for key := range traceNames {
		if !query.IsValid(key) {
			sklog.Errorf("Invalid key: %q", key)
			continue
		}

		mutex.Lock()
		// Make space in ret for the values.
		ret[key] = vec32.New(len(commits))

		// Update the map from the full name of the trace and id in traceIDForSQLInBytes form.
		traceNameMap[traceIDForSQLInBytesFromTraceName(key)] = key
		mutex.Unlock()

		trID := traceIDForSQLFromTraceName(key)
		currentChunk = append(currentChunk, trID)
		if len(currentChunk) >= queryTracesChunkSize {
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
		return nil, nil, skerr.Wrap(err)
	}

	return ret, commits, nil
}

// readTracesChunk updates the passed in TraceSet with all the values loaded for
// the given slice of trace ids.
//
// The mutex protects 'ret' and 'traceNameMap'.
func (s *SQLTraceStore) readTracesChunk(ctx context.Context, beginCommit types.CommitNumber, endCommit types.CommitNumber, commits []provider.Commit, chunk []traceIDForSQL, mutex *sync.Mutex, traceNameMap map[traceIDForSQLInBytes]string, ret *types.TraceSet) error {
	if len(chunk) == 0 {
		return nil
	}
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
	queryCtx, querySpan := trace.StartSpan(ctx, "sqltracestore.ReadTraces.Chunk.ExecuteSQLQuery")
	rows, err := s.db.Query(queryCtx, sql)
	querySpan.End()
	if err != nil {
		return skerr.Wrapf(err, "SQL: %q", sql)
	}

	var traceIDArray traceIDForSQLInBytes
	commitToIndexMap := map[types.CommitNumber]int{}
	for i, commit := range commits {
		commitToIndexMap[commit.CommitNumber] = i
	}

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
		(*ret)[traceNameMap[traceIDArray]][commitToIndexMap[commitNumber]] = float32(val)
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
	err := s.db.QueryRow(ctx, s.statements[traceCount], tileNumber).Scan(&ret)
	span.AddAttributes(trace.Int64Attribute("count", ret))
	return ret, skerr.Wrap(err)
}

// updateSourceFile writes the filename into the SourceFiles table and returns
// the sourceFileIDFromSQL of that filename.
func (s *SQLTraceStore) updateSourceFile(ctx context.Context, filename string) (sourceFileIDFromSQL, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.updateSourceFile")
	defer span.End()

	ret := badSourceFileIDFromSQL
	// We want to ensure that there is only one entry per source file.
	// We do that by checking for existence of a row first and do the insert
	// only if there are no rows.
	err := s.db.QueryRow(ctx, s.statements[getSourceFileID], filename).Scan(&ret)
	if err == pgx.ErrNoRows {
		_, err = s.db.Exec(ctx, s.statements[insertIntoSourceFiles], filename)

		if err != nil {
			return ret, skerr.Wrap(err)
		}

		// We can potentially get rid of this read by returning the id in the
		// insert statement above.
		err = s.db.QueryRow(ctx, s.statements[getSourceFileID], filename).Scan(&ret)
		if err != nil {
			return ret, skerr.Wrap(err)
		}
	} else if err != nil {
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

	ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
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
		err := util.ChunkIter(len(paramSetsContext), writeTracesParamSetsChunkSize, func(startIdx int, endIdx int) error {
			ctx, span := trace.StartSpan(ctx, "sqltracestore.WriteTraces.writeParamSetsChunk")
			defer span.End()

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

	traceParams := map[string]paramtools.Params{}
	for i, p := range params {
		traceName, err := query.MakeKey(p)
		if err != nil {
			sklog.Errorf("Somehow still invalid: %v", p)
			continue
		}
		traceID := traceIDForSQLFromTraceName(traceName)
		traceParams[string(traceID)] = p
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
		var err error
		if s.isSpanner {
			err = util.ChunkIterParallelPool(ctx, len(postingsTemplateContext), writeTracesPostingsChunkSize, writePostingsParallelPoolSize, func(ctx context.Context, startIdx int, endIdx int) error {
				ctx, span := trace.StartSpan(ctx, "sqltracestore.WriteTraces.writePostingsChunkParallel")
				defer span.End()

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
		} else {
			err = util.ChunkIter(len(postingsTemplateContext), writeTracesPostingsChunkSize, func(startIdx int, endIdx int) error {
				ctx, span := trace.StartSpan(ctx, "sqltracestore.WriteTraces.writePostingsChunk")
				defer span.End()

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
		}

		if err != nil {
			return err
		}

		for _, entry := range postingsTemplateContext {
			s.cache.Add(entry.cacheKey)
		}
	}

	sklog.Infof("Writing %d trace params entries", len(traceParams))
	traceParamsError := s.traceParamStore.WriteTraceParams(ctx, traceParams)
	if traceParamsError != nil {
		// Log and ignore this error while we release and test this feature.
		// TODO(ashwinpv): Return the error once we have fully tested.
		sklog.Infof("Error writing trace params: %v", traceParamsError)
	}
	sklog.Infof("About to format %d trace values", len(valuesTemplateContext))

	if s.isSpanner {
		err = util.ChunkIterParallelPool(ctx, len(valuesTemplateContext), writeTracesValuesChunkSize, writeTracesParallelPoolSize, func(ctx context.Context, startIdx int, endIdx int) error {
			ctx, span := trace.StartSpan(ctx, "sqltracestore.WriteTraces.writeTraceValuesChunkParallel")
			defer span.End()

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
	} else {
		err = util.ChunkIter(len(valuesTemplateContext), writeTracesValuesChunkSize, func(startIdx int, endIdx int) error {
			ctx, span := trace.StartSpan(ctx, "sqltracestore.WriteTraces.writeTraceValuesChunk")
			defer span.End()

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
	}

	if err != nil {
		return err
	}

	sklog.Info("Finished writing trace values.")

	return nil
}

func (s *SQLTraceStore) WriteTraces2(ctx context.Context, commitNumber types.CommitNumber, params []paramtools.Params, values []float32, ps paramtools.ParamSet, source string, _ time.Time) error {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.WriteTraces2")
	defer span.End()

	defer timer.NewWithSummary("perfserver_sqltracestore_write_traces2", s.writeTracesMetric).Stop()

	ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()

	// Write the source file entry and the id.
	sourceID, err := s.updateSourceFile(ctx, source)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Build the 'context's which will be used to populate the SQL templates for
	// the TraceValues table.
	t := timer.NewWithSummary("perfserver_sqltracestore_build_traces_contexts2", s.buildTracesContextsMetric)
	valuesTemplateContext2 := make([]insertIntoTraceValuesContext2, 0, len(params))

	for i, p := range params {
		traceName, err := query.MakeKey(p)
		if err != nil {
			sklog.Errorf("Somehow still invalid: %v", p)
			continue
		}
		traceID := traceIDForSQLFromTraceName(traceName)
		insertContext := insertIntoTraceValuesContext2{
			MD5HexTraceID: traceID,
			CommitNumber:  commitNumber,
			Val:           values[i],
			SourceFileID:  sourceID,
		}
		if v, ok := p["benchmark"]; ok {
			insertContext.Benchmark = v
		}
		if v, ok := p["bot"]; ok {
			insertContext.Bot = v
		}
		if v, ok := p["test"]; ok {
			insertContext.Test = v
		}
		if v, ok := p["subtest_1"]; ok {
			insertContext.Subtest_1 = v
		}
		if v, ok := p["subtest_2"]; ok {
			insertContext.Subtest_2 = v
		}
		if v, ok := p["subtest_3"]; ok {
			insertContext.Subtest_3 = v
		}
		valuesTemplateContext2 = append(valuesTemplateContext2, insertContext)
	}
	t.Stop()

	// Now that the contexts are built, execute the SQL in batches.
	defer timer.NewWithSummary("perfserver_sqltracestore_write_traces2_sql_insert", s.writeTracesMetricSQL).Stop()

	sklog.Infof("About to format %d trace values 2", len(valuesTemplateContext2))

	err = util.ChunkIterParallelPool(ctx, len(valuesTemplateContext2), writeTracesValuesChunkSize, writeTracesParallelPoolSize, func(ctx context.Context, startIdx int, endIdx int) error {
		ctx, span := trace.StartSpan(ctx, "sqltracestore.WriteTraces2.writeTraceValuesChunkParallel")
		defer span.End()

		var b bytes.Buffer
		if err := s.unpreparedStatements[insertIntoTraceValues2].Execute(&b, valuesTemplateContext2[startIdx:endIdx]); err != nil {
			return skerr.Wrapf(err, "failed to expand trace values2 template")
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

	sklog.Info("Finished writing trace values 2.")

	return nil
}

// commitSliceFromCommitNumberRange returns a slice of Commits that fall in the range
// [begin, end], i.e  inclusive of both begin and end.
func (s *SQLTraceStore) commitSliceFromCommitNumberRange(ctx context.Context, begin, end types.CommitNumber) ([]provider.Commit, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.commitSliceFromCommitNumberRange")
	defer span.End()

	s.commitSliceFromCommitNumberRangeCalled.Inc(1)
	rows, err := s.db.Query(ctx, s.statements[getCommitsFromCommitNumberRange], begin, end)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to query for commit slice in range %v-%v", begin, end)
	}
	defer rows.Close()
	ret := []provider.Commit{}
	for rows.Next() {
		var c provider.Commit
		if err := rows.Scan(&c.CommitNumber, &c.GitHash, &c.Timestamp, &c.Author, &c.Subject); err != nil {
			return nil, skerr.Wrapf(err, "Failed to read row in range %v-%v", begin, end)
		}
		ret = append(ret, c)
	}
	return ret, nil
}

// deleteCommit delete a commit from Commits table.
// this method is for testing only.
func (s *SQLTraceStore) deleteCommit(ctx context.Context, commitNumber types.CommitNumber) error {
	commandTag, err := s.db.Exec(ctx, s.statements[deleteCommit], commitNumber)
	if err != nil {
		return skerr.Wrapf(err, "Failed to delete the commit %v", commitNumber)
	}
	if commandTag.RowsAffected() != 1 {
		return skerr.Fmt("Failed to delete the commit %v", commitNumber)
	}
	return nil
}

// getTraceIdChannelFromCache returns a params channel containing the trace params retrieved from the cache.
func (s *SQLTraceStore) getTraceIdChannelFromCache(ctx context.Context, traceCache *tracecache.TraceCache, tileNumber types.TileNumber, query *query.Query) (<-chan paramtools.Params, error) {
	traceIdsFromCache, err := traceCache.GetTraceIds(ctx, tileNumber, query)
	if err != nil {
		return nil, err
	}

	traceIdsChannel := make(chan paramtools.Params, queryTracesIDOnlyByIndexChannelSize)
	if traceIdsFromCache != nil {
		sklog.Infof("Retrieved %d trace ids from cache for tile %d and query %v", len(traceIdsFromCache), tileNumber, query)
		go func() {
			for _, traceId := range traceIdsFromCache {
				traceIdsChannel <- traceId
			}
			close(traceIdsChannel)
		}()
		return traceIdsChannel, nil
	} else {
		// Make sure the channel is closed in the case where there is a cache miss.
		close(traceIdsChannel)
		return nil, skerr.Fmt("No traceIds found in the cache.")
	}
}

// Confirm that *SQLTraceStore fulfills the tracestore.TraceStore interface.
var _ tracestore.TraceStore = (*SQLTraceStore)(nil)
