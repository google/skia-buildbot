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
match the given query on a tile. This is done using the InMemoryTraceParams.

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
*/
package sqltracestore

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
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
)

const (
	// cacheMetricsRefreshDuration controls how often we update the metrics for
	// in-memory caches.
	cacheMetricsRefreshDuration = 15 * time.Second

	writeTracesValuesChunkSize    = 100
	writeTracesParamSetsChunkSize = 100
	// The number of parallel writes when writing traces data.
	writeTracesParallelPoolSize = 5

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
// Note the \x prefix which tells the database that this is hex encoded.
type traceIDForSQL string

var badTraceIDFromSQL traceIDForSQL = ""

// traceIDForSQLInBytes is the md5 hash of a trace name.
type traceIDForSQLInBytes [md5.Size]byte

// Calculates the traceIDForSQL for the given trace name, e.g. "\x00112233...".
// Note the \x prefix which tells the database that this is hex encoded.
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
	insertIntoParamSets
	getSourceFileID
	getLatestTile
	paramSetForTile
	getSource
	getSources
	readTraces
	getLastNSources
	deleteCommit
	countCommitInCommitNumberRange
	getCommitsFromCommitNumberRange
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
        ON CONFLICT (trace_id, commit_number) DO UPDATE
        SET trace_id=EXCLUDED.trace_id, commit_number=EXCLUDED.commit_number, val=EXCLUDED.val, source_file_id=EXCLUDED.source_file_id
        `,
	insertIntoTraceValues2: `INSERT INTO
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
         ON CONFLICT (trace_id, commit_number) DO UPDATE
         SET trace_id=EXCLUDED.trace_id, commit_number=EXCLUDED.commit_number, val=EXCLUDED.val, source_file_id=EXCLUDED.source_file_id,
            benchmark=EXCLUDED.benchmark, bot=EXCLUDED.bot, test=EXCLUDED.test, subtest_1=EXCLUDED.subtest_1, subtest_2=EXCLUDED.subtest_2, subtest_3=EXCLUDED.subtest_3
        `,
	readTraces: `
        SELECT
            trace_id,
            commit_number,
            val,
            source_file_id
        FROM
            TraceValues
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
        INNER JOIN SourceFiles ON SourceFiles.source_file_id = TraceValues.source_file_id
        WHERE
            TraceValues.trace_id = '{{ .MD5HexTraceID }}'
            AND TraceValues.commit_number = {{ .CommitNumber }}`,
	getSources: `
        SELECT
            TraceValues.commit_number, SourceFiles.source_file
        FROM
            TraceValues
        INNER JOIN SourceFiles ON SourceFiles.source_file_id = TraceValues.source_file_id
        WHERE
            TraceValues.trace_id = '{{ .MD5HexTraceID }}'
            AND TraceValues.commit_number IN `,
	insertIntoParamSets: `
        INSERT INTO
            ParamSets (tile_number, param_key, param_value)
        VALUES
            {{ range $index, $element :=  . -}}
                {{ if $index }},{{end}}
                ( {{ $element.TileNumber }}, '{{ $element.Key }}', '{{ $element.Value }}' )
            {{ end }}
        ON CONFLICT (tile_number, param_key, param_value)
        DO NOTHING`,
	paramSetForTile: `
        SELECT
           param_key, param_value
        FROM
            ParamSets
        WHERE
            tile_number = {{ .TileNumber }}`,
}

// replaceTraceValuesContext is the context for the replaceTraceValues template.
type insertIntoTraceValuesContext struct {
	// The MD5 sum of the trace name as a hex string, i.e.
	// "\xfe385b159ff55dca481069805e5ff050". Note the leading \x which
	// the database will use to know the string is in hex.
	MD5HexTraceID traceIDForSQL

	CommitNumber types.CommitNumber
	Val          float32
	SourceFileID sourceFileIDFromSQL
}

// replaceTraceValuesContext is the context for the replaceTraceValues2 template.
type insertIntoTraceValuesContext2 struct {
	// The MD5 sum of the trace name as a hex string, i.e.
	// "\xfe385b159ff55dca481069805e5ff050". Note the leading \x which
	// the database will use to know the string is in hex.
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
	// the database will use to know the string is in hex.
	MD5HexTraceID traceIDForSQL
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
	// the database will use to know the string is in hex.
	MD5HexTraceID traceIDForSQL
}

// getSourcesContext is the context for the getSourceIds template.
type getSourcesContext struct {
	// The MD5 sum of the trace name as a hex string, i.e.
	// "\xfe385b159ff55dca481069805e5ff050". Note the leading \x which
	// query will use to know the string is in hex.
	MD5HexTraceID traceIDForSQL
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
        ON CONFLICT (source_file_id)
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
            ParamSets
        ORDER BY
            tile_number DESC
        LIMIT
            1;`,
	getLastNSources: `
        SELECT
            SourceFiles.source_file, TraceValues.commit_number
        FROM
            TraceValues
            INNER JOIN
                SourceFiles
            ON
                TraceValues.source_file_id = SourceFiles.source_file_id
        WHERE
            TraceValues.trace_id=$1
        ORDER BY
            TraceValues.commit_number DESC
        LIMIT
            $2`,
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

// SQLTraceStore implements tracestore.TraceStore backed onto an SQL database.
type SQLTraceStore struct {
	// db is the SQL database instance.
	db                  pool.Pool
	inMemoryTraceParams *InMemoryTraceParams

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

	traceParamStore tracestore.TraceParamStore

	// metrics
	writeTracesMetric                      metrics2.Float64SummaryMetric
	writeTracesMetricSQL                   metrics2.Float64SummaryMetric
	buildTracesContextsMetric              metrics2.Float64SummaryMetric
	cacheMissMetric                        metrics2.Counter
	orderedParamSetsCacheMissMetric        metrics2.Counter
	queryRestrictionMinKeyInPlan           metrics2.Float64SummaryMetric
	orderedParamSetCacheLen                metrics2.Int64Metric
	commitSliceFromCommitNumberRangeCalled metrics2.Counter
}

// New returns a new *SQLTraceStore.
func New(db pool.Pool, datastoreConfig config.DataStoreConfig, traceParamStore tracestore.TraceParamStore,
	inMemoryTraceParams *InMemoryTraceParams) (*SQLTraceStore, error) {
	unpreparedStatements := map[statement]*template.Template{}
	queryTemplates := templates
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

	if inMemoryTraceParams == nil {
		return nil, skerr.Fmt("inMemoryTraceParams cannot be nil")
	}

	ret := &SQLTraceStore{
		db:                                     db,
		inMemoryTraceParams:                    inMemoryTraceParams,
		unpreparedStatements:                   unpreparedStatements,
		statements:                             statements,
		tileSize:                               datastoreConfig.TileSize,
		cache:                                  cache,
		orderedParamSetCache:                   paramSetCache,
		traceParamStore:                        traceParamStore,
		writeTracesMetric:                      metrics2.GetFloat64SummaryMetric("perfserver_sqltracestore_write_traces"),
		writeTracesMetricSQL:                   metrics2.GetFloat64SummaryMetric("perfserver_sqltracestore_write_traces_sql"),
		buildTracesContextsMetric:              metrics2.GetFloat64SummaryMetric("perfserver_sqltracestore_build_traces_context"),
		cacheMissMetric:                        metrics2.GetCounter("perfserver_sqltracestore_cache_miss"),
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

// GetSourceIds returns the source ids for the given traces and commits.
// The returned object is a map where the key is the name of the trace and value is a map of commit number to the source file name.
func (s *SQLTraceStore) GetSourceIds(ctx context.Context, commitNumbers []types.CommitNumber, traceNames []string) (map[string]map[types.CommitNumber]string, error) {
	sourceInfo := map[string]map[types.CommitNumber]string{}
	for _, traceName := range traceNames {
		sourcesForTrace, err := s.GetSources(ctx, traceName, commitNumbers)
		if err != nil {
			return nil, err
		}
		sourceInfo[traceName] = sourcesForTrace
	}

	return sourceInfo, nil
}

// GetSources returns the source files for a given trace and list of commits.
func (s *SQLTraceStore) GetSources(ctx context.Context, traceName string, commits []types.CommitNumber) (map[types.CommitNumber]string, error) {
	traceID := traceIDForSQLFromTraceName(traceName)

	sourceContext := getSourcesContext{
		MD5HexTraceID: traceID,
	}

	var b bytes.Buffer
	if err := s.unpreparedStatements[getSources].Execute(&b, sourceContext); err != nil {
		return nil, skerr.Wrapf(err, "failed to expand get source template")
	}
	sql := b.String()

	var sb strings.Builder
	for _, commit := range commits {
		sb.WriteString(fmt.Sprintf("%d,", commit))
	}
	commitString := sb.String()
	sql = sql + fmt.Sprintf("(%s)", commitString[:len(commitString)-1])
	rows, err := s.db.Query(ctx, sql)
	if err != nil {
		return nil, skerr.Wrapf(err, "commitNumber=%v traceName=%q traceID=%q", commits, traceName, traceID)
	}

	sourceData := map[types.CommitNumber]string{}
	for rows.Next() {
		var commitNumber types.CommitNumber
		var sourceFile string
		if err := rows.Scan(&commitNumber, &sourceFile); err != nil {
			return nil, err
		}
		sourceData[commitNumber] = sourceFile
	}

	return sourceData, nil
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
func (s *SQLTraceStore) QueryTraces(ctx context.Context, tileNumber types.TileNumber, q *query.Query, traceCache *tracecache.TraceCache) (types.TraceSet, []provider.Commit, map[string]*types.TraceSourceInfo, error) {
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
		return nil, nil, nil, skerr.Wrapf(err, "Failed to get list of traceIDs matching query.")
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

	s.inMemoryTraceParams.QueryTraceIDs(ctx, tileNumber, q, outParams)
	return outParams, nil
}

// ReadTraces implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) ReadTraces(ctx context.Context, tileNumber types.TileNumber, traceNames []string) (types.TraceSet, []provider.Commit, map[string]*types.TraceSourceInfo, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.ReadTraces")
	defer span.End()

	defer timer.New("ReadTraces").Stop()

	beginCommit, endCommit := types.TileCommitRangeForTileNumber(tileNumber, s.tileSize)
	return s.ReadTracesForCommitRange(ctx, traceNames, beginCommit, endCommit)
}

// ReadTracesForCommitRange implements the tracestore.TraceStore interface.
func (s *SQLTraceStore) ReadTracesForCommitRange(ctx context.Context, traceNames []string, beginCommit types.CommitNumber, endCommit types.CommitNumber) (types.TraceSet, []provider.Commit, map[string]*types.TraceSourceInfo, error) {
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
func (s *SQLTraceStore) readTracesByChannelForCommitRange(ctx context.Context, traceNamesChan <-chan string, beginCommit types.CommitNumber, endCommit types.CommitNumber) (types.TraceSet, []provider.Commit, map[string]*types.TraceSourceInfo, error) {
	ctx, span := trace.StartSpan(ctx, "sqltracestore.readTracesByChannelForCommitRange")
	defer span.End()

	// The return value, protected by mutex.
	ret := types.TraceSet{}

	// Validate the begin and end commit numbers.
	if beginCommit > endCommit {
		// Empty the traceNames channel.
		for range <-traceNamesChan {
		}
		return nil, nil, nil, skerr.Fmt("Invalid commit range, [%d, %d] should be [%d, %d]", beginCommit, endCommit, endCommit, beginCommit)
	}

	commits, err := s.commitSliceFromCommitNumberRange(ctx, beginCommit, endCommit)
	if err != nil {
		return nil, nil, nil, skerr.Fmt("Cannot count commit within the commit range, [%d, %d]", beginCommit, endCommit)
	}

	// Map from the [md5.Size]byte representation of a trace id to the trace name.
	//
	// Protected by mutex.
	traceNameMap := map[traceIDForSQLInBytes]string{}

	// Protects traceNameMap and ret.
	var mutex sync.Mutex
	traceNames := []string{}
	var traceIDsForQuery []traceIDForSQL
	for traceName := range traceNamesChan {
		traceNames = append(traceNames, traceName)
		traceIDBytes := traceIDForSQLInBytesFromTraceName(traceName)
		traceNameMap[traceIDBytes] = traceName
		traceIDsForQuery = append(traceIDsForQuery, traceIDForSQLFromTraceName(traceName))
	}

	if len(traceNames) == 0 {
		return types.TraceSet{}, commits, nil, nil
	}

	for _, name := range traceNames {
		if !query.IsValid(name) {
			sklog.Errorf("Invalid trace name: %q", name)
			continue
		}
		mutex.Lock()
		ret[name] = vec32.New(len(commits))
		mutex.Unlock()
	}

	sourceFileMap := map[string]*types.TraceSourceInfo{}

	// Iterate over the traceIDs in chunks and query the database in parallel.
	err = util.ChunkIterParallelPool(ctx, len(traceIDsForQuery), 5, 10, func(ctx context.Context, startIdx, endIdx int) error {
		chunk := traceIDsForQuery[startIdx:endIdx]
		if err := s.readTracesChunk(ctx, beginCommit, endCommit, commits, chunk, &mutex, traceNameMap, &ret, sourceFileMap); err != nil {
			return skerr.Wrap(err)
		}
		return nil
	})

	if err != nil {
		span.SetStatus(trace.Status{
			Code:    trace.StatusCodeInternal,
			Message: err.Error(),
		})
		return nil, nil, nil, skerr.Wrap(err)
	}

	return ret, commits, sourceFileMap, nil
}

// readTracesChunk updates the passed in TraceSet with all the values loaded for
// the given slice of trace ids.
//
// The mutex protects 'ret' and 'traceNameMap'.
func (s *SQLTraceStore) readTracesChunk(ctx context.Context, beginCommit types.CommitNumber, endCommit types.CommitNumber, commits []provider.Commit, chunk []traceIDForSQL, mutex *sync.Mutex, traceNameMap map[traceIDForSQLInBytes]string, ret *types.TraceSet, sourceFileMap map[string]*types.TraceSourceInfo) error {
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

	// Create a local map to store results from this chunk. This avoids
	// holding the main lock while iterating over every row.
	localTraces := types.TraceSet{}
	var traceIDArray traceIDForSQLInBytes
	commitToIndexMap := map[types.CommitNumber]int{}
	localSourceFileMap := map[string]*types.TraceSourceInfo{}
	for i, commit := range commits {
		commitToIndexMap[commit.CommitNumber] = i
	}

	for rows.Next() {
		var traceIDInBytes []byte
		var commitNumber types.CommitNumber
		var val float64
		var sourceFileId int64
		if err := rows.Scan(&traceIDInBytes, &commitNumber, &val, &sourceFileId); err != nil {
			return skerr.Wrap(err)
		}

		// pgx can't Scan into an array, but Go can't use a slice as a map key, so
		// we Scan into a byte slice and then copy into a byte array to use
		// as the index into the map.
		copy(traceIDArray[:], traceIDInBytes)

		// Note: We read traceNameMap without a lock. This is safe because the map is
		// fully populated before the goroutines are dispatched and is not written to after.
		traceName := traceNameMap[traceIDArray]

		if localTraces[traceName] == nil {
			localTraces[traceName] = vec32.New(len(commits))
		}
		localTraces[traceName][commitToIndexMap[commitNumber]] = float32(val)
		if _, ok := localSourceFileMap[traceName]; !ok {
			localSourceFileMap[traceName] = types.NewTraceSourceInfo()
		}
		localSourceFileMap[traceName].Add(commitNumber, sourceFileId)
	}
	if err := rows.Err(); err != nil {
		return skerr.Wrap(err)
	}

	mutex.Lock()
	defer mutex.Unlock()

	// Merge the locally collected results into the final shared result map.
	for traceName, localValues := range localTraces {
		// The slice in the final 'ret' map was already created before this goroutine started.
		// We just need to carefully copy the values we found into it.
		for i, v := range localValues {
			if v != vec32.MissingDataSentinel {
				(*ret)[traceName][i] = v
			}
		}
	}
	for traceName, localSourceInfo := range localSourceFileMap {
		if _, ok := sourceFileMap[traceName]; !ok {
			sourceFileMap[traceName] = localSourceInfo
		} else {
			sourceFileMap[traceName].CopyFrom(localSourceInfo)
		}
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
	// the TraceValues table.
	t := timer.NewWithSummary("perfserver_sqltracestore_build_traces_contexts", s.buildTracesContextsMetric)
	valuesTemplateContext := make([]insertIntoTraceValuesContext, 0, len(params))

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

	}
	t.Stop()

	// Now that the contexts are built, execute the SQL in batches.
	defer timer.NewWithSummary("perfserver_sqltracestore_write_traces_sql_insert", s.writeTracesMetricSQL).Stop()

	sklog.Infof("Writing %d trace params entries", len(traceParams))
	traceParamsError := s.traceParamStore.WriteTraceParams(ctx, traceParams)
	if traceParamsError != nil {
		// Log and ignore this error while we release and test this feature.
		// TODO(ashwinpv): Return the error once we have fully tested.
		sklog.Infof("Error writing trace params: %v", traceParamsError)
	}
	sklog.Infof("About to format %d trace values", len(valuesTemplateContext))

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
