package providers

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/search/caching"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/types"
)

// TraceDigestsProvider provides a struct to retrieve trace and digests information for search.
type TraceDigestsProvider struct {
	db                       *pgxpool.Pool
	windowLength             int
	materializedViewProvider *MaterializedViewProvider
	cacheManager             *caching.SearchCacheManager
	mutex                    sync.RWMutex
	dbType                   config.DatabaseType

	// This caches the trace ids that are publicly visible.
	publiclyVisibleTraces map[schema.MD5Hash]struct{}
	isPublicView          bool

	// Metrics
	search_cache_enabled_counter metrics2.Counter
	search_cache_hit_counter     metrics2.Counter
	search_cache_miss_counter    metrics2.Counter
}

// NewTraceDigestsProvider returns a new instance of TraceDigestsProvider.
func NewTraceDigestsProvider(db *pgxpool.Pool, windowLength int, materializedViewProvider *MaterializedViewProvider, cacheManager *caching.SearchCacheManager) *TraceDigestsProvider {
	return &TraceDigestsProvider{
		db:                       db,
		windowLength:             windowLength,
		materializedViewProvider: materializedViewProvider,
		cacheManager:             cacheManager,

		search_cache_enabled_counter: metrics2.GetCounter("gold_search_cache_enabled"),
		search_cache_hit_counter:     metrics2.GetCounter("gold_search_cache_hit"),
		search_cache_miss_counter:    metrics2.GetCounter("gold_search_cache_miss"),
	}
}

// SetDatabaseType sets the database type for the current configuration.
func (s *TraceDigestsProvider) SetDatabaseType(dbType config.DatabaseType) {
	s.dbType = dbType
	s.cacheManager.SetDatabaseType(dbType)
}

// SetPublicTraces sets the given traces as the publicly visible ones.
func (s *TraceDigestsProvider) SetPublicTraces(traces map[schema.MD5Hash]struct{}) {
	s.isPublicView = true
	s.publiclyVisibleTraces = traces
	s.cacheManager.SetPublicTraces(traces)
}

// GetMatchingDigestsAndTraces returns the tuples of digest+traceID that match the given query.
// The order of the result is arbitrary.
func (s *TraceDigestsProvider) GetMatchingDigestsAndTraces(ctx context.Context, includeUntriagedDigests, includeNegativeDigests, includePositiveDigests bool) ([]common.DigestWithTraceAndGrouping, error) {
	ctx, span := trace.StartSpan(ctx, "getMatchingDigestsAndTraces")
	defer span.End()

	q := common.GetQuery(ctx)
	searchQueryContext := caching.MatchingTracesQueryContext{
		IncludeUntriaged:                 includeUntriagedDigests,
		IncludeNegative:                  includeNegativeDigests,
		IncludePositive:                  includePositiveDigests,
		IncludeIgnored:                   q.IncludeIgnoredTraces,
		OnlyIncludeDigestsProducedAtHead: q.OnlyIncludeDigestsProducedAtHead,
		TraceValues:                      q.TraceValues,
		Corpus:                           sql.Sanitize(q.TraceValues[types.CorpusField][0]),
	}
	if !areQueryResultsCached(searchQueryContext) {
		sklog.Infof("The current query %v is not supported by cache. Fall back to database search.", searchQueryContext)
		return s.getMatchingDigestsAndTracesFromDB(ctx, includeUntriagedDigests, includeNegativeDigests, includePositiveDigests)
	}

	s.search_cache_enabled_counter.Inc(1)

	sklog.Infof("Retrieving matching digests from cache for query context: %v", searchQueryContext)
	results, err := s.cacheManager.GetMatchingDigestsAndTraces(ctx, searchQueryContext)
	if err != nil {
		sklog.Errorf("Error retrieving search data from cache: %v", err)
	}
	if results == nil {
		// This is either an error during retrieving data from cache or
		// a cache miss. Let's fall back to the database search.
		sklog.Info("No data returned from cache or there was an error. Falling back to databases search.")
		s.search_cache_miss_counter.Inc(1)
		return s.getMatchingDigestsAndTracesFromDB(ctx, includeUntriagedDigests, includeNegativeDigests, includePositiveDigests)
	}

	s.search_cache_hit_counter.Inc(1)
	return results, err
}

// areQueryResultsCached returns true if we are caching the results for the
// given query.
// Currently only the digests produced on HEAD are supported.
func areQueryResultsCached(queryContext caching.MatchingTracesQueryContext) bool {
	return queryContext.OnlyIncludeDigestsProducedAtHead
}

// getMatchingDigestsAndTraces returns the tuples of digest+traceID that match the given query.
// The order of the result is arbitrary.
func (s *TraceDigestsProvider) getMatchingDigestsAndTracesFromDB(ctx context.Context, includeUntriagedDigests, includeNegativeDigests, includePositiveDigests bool) ([]common.DigestWithTraceAndGrouping, error) {
	ctx, span := trace.StartSpan(ctx, "getMatchingDigestsAndTracesFromDB")
	defer span.End()

	statement := `WITH
MatchingDigests AS (
	SELECT grouping_id, digest FROM Expectations
	WHERE label = ANY($1)
),`
	tracesBlock, args := s.matchingTracesStatement(ctx)
	statement += tracesBlock
	statement += `
SELECT trace_id, MatchingDigests.grouping_id, MatchingTraces.digest FROM
MatchingDigests
JOIN
MatchingTraces ON MatchingDigests.grouping_id = MatchingTraces.grouping_id AND
  MatchingDigests.digest = MatchingTraces.digest`

	var triageStatuses []schema.ExpectationLabel
	if includeUntriagedDigests {
		triageStatuses = append(triageStatuses, schema.LabelUntriaged)
	}
	if includeNegativeDigests {
		triageStatuses = append(triageStatuses, schema.LabelNegative)
	}
	if includePositiveDigests {
		triageStatuses = append(triageStatuses, schema.LabelPositive)
	}
	arguments := append([]interface{}{triageStatuses}, args...)

	rows, err := s.db.Query(ctx, statement, arguments...)
	if err != nil {
		return nil, skerr.Wrapf(err, "searching for query with args %v", arguments)
	}
	defer rows.Close()
	var rv []common.DigestWithTraceAndGrouping
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	var traceKey schema.MD5Hash
	for rows.Next() {
		var row common.DigestWithTraceAndGrouping
		if err := rows.Scan(&row.TraceID, &row.GroupingID, &row.Digest); err != nil {
			return nil, skerr.Wrap(err)
		}
		if s.publiclyVisibleTraces != nil {
			copy(traceKey[:], row.TraceID)
			if _, ok := s.publiclyVisibleTraces[traceKey]; !ok {
				continue
			}
		}
		rv = append(rv, row)
	}
	return rv, nil
}

// getTracesWithUntriagedDigestsAtHead identifies all untriaged digests being produced at head
// within the current window and returns all traces responsible for that behavior, clustered by the
// digest+grouping at head. This clustering allows us to better identify the commit(s) that caused
// the change, even with sparse data.
func (s *TraceDigestsProvider) GetTracesWithUntriagedDigestsAtHead(ctx context.Context, corpus string) (map[common.GroupingDigestKey][]schema.TraceID, error) {
	ctx, span := trace.StartSpan(ctx, "getTracesWithUntriagedDigestsAtHead")
	defer span.End()

	byBlameData, err := s.cacheManager.GetByBlameData(ctx, string(common.GetFirstCommitID(ctx)), corpus)
	if err != nil {
		sklog.Errorf("Error encountered when retrieving ByBlame data from cache: %v", err)
		return nil, err
	}

	sklog.Debugf("Retrieved %d items from search cache for corpus %s", len(byBlameData), corpus)
	rv := map[common.GroupingDigestKey][]schema.TraceID{}
	var key common.GroupingDigestKey
	groupingKey := key.GroupingID[:]
	digestKey := key.Digest[:]
	for _, data := range byBlameData {
		copy(groupingKey, data.GroupingID)
		copy(digestKey, data.Digest)
		rv[key] = append(rv[key], data.TraceID)
	}

	return rv, nil
}

// GetTracesForGroupingAndDigest finds the traces on the primary branch which produced the given
// digest in the provided grouping. If no such trace exists (recently), a single result will be
// added with no trace, to all the UI to at least show the image and possible diff data.
func (s *TraceDigestsProvider) GetTracesForGroupingAndDigest(ctx context.Context, grouping paramtools.Params, digest types.Digest) ([]common.DigestWithTraceAndGrouping, error) {
	ctx, span := trace.StartSpan(ctx, "getTracesForGroupingAndDigest")
	defer span.End()

	_, groupingID := sql.SerializeMap(grouping)
	digestBytes, err := sql.DigestToBytes(digest)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	tableName := "TiledTraceDigests@grouping_digest_idx"
	if s.dbType == config.Spanner {
		tableName = "TiledTraceDigests"
	}
	statement := fmt.Sprintf(`SELECT trace_id FROM %s
WHERE tile_id >= $1 AND grouping_id = $2 AND digest = $3
`, tableName)
	rows, err := s.db.Query(ctx, statement, common.GetFirstTileID(ctx), groupingID, digestBytes)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var results []common.DigestWithTraceAndGrouping
	for rows.Next() {
		var traceID schema.TraceID
		if err := rows.Scan(&traceID); err != nil {
			return nil, skerr.Wrap(err)
		}
		results = append(results, common.DigestWithTraceAndGrouping{
			TraceID:    traceID,
			GroupingID: groupingID,
			Digest:     digestBytes,
		})
	}
	if len(results) == 0 {
		// Add in a result that has at least the digest and groupingID. This can be helpful
		// for when an image was produced a long time ago, but not in recent traces. It
		// allows the UI to at least show the image and some diff information.
		results = append(results, common.DigestWithTraceAndGrouping{
			GroupingID: groupingID,
			Digest:     digestBytes,
		})
	}
	return results, nil
}

// GetTracesFromCLThatProduced returns a common.DigestWithTraceAndGrouping for all traces that produced
// the provided digest in the given grouping on the most recent PS for the given CL.
func (s *TraceDigestsProvider) GetTracesFromCLThatProduced(ctx context.Context, grouping paramtools.Params, digest types.Digest) ([]common.DigestWithTraceAndGrouping, error) {
	ctx, span := trace.StartSpan(ctx, "getTracesFromCLThatProduced")
	defer span.End()

	_, groupingID := sql.SerializeMap(grouping)
	digestBytes, err := sql.DigestToBytes(digest)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	const statement = `WITH
MostRecentPS AS (
	SELECT patchset_id FROM Patchsets WHERE changelist_id = $1
	ORDER BY created_ts DESC, ps_order DESC
	LIMIT 1
)
SELECT secondary_branch_trace_id, options_id FROM SecondaryBranchValues
JOIN MostRecentPS ON SecondaryBranchValues.version_name = MostRecentPS.patchset_id
WHERE branch_name = $1 AND grouping_id = $2 AND digest = $3`
	rows, err := s.db.Query(ctx, statement, common.GetQualifiedCL(ctx), groupingID, digestBytes)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var results []common.DigestWithTraceAndGrouping
	for rows.Next() {
		var traceID schema.TraceID
		var optionsID schema.OptionsID
		if err := rows.Scan(&traceID, &optionsID); err != nil {
			return nil, skerr.Wrap(err)
		}
		results = append(results, common.DigestWithTraceAndGrouping{
			TraceID:    traceID,
			GroupingID: groupingID,
			Digest:     digestBytes,
			OptionsID:  optionsID,
		})
	}
	return results, nil
}

// matchingTracesStatement returns a SQL snippet that includes a WITH table called MatchingTraces.
// This table will have rows containing trace_id, grouping_id, and digest of traces that match
// the given search criteria. The second parameter is the arguments that need to be included
// in the query. This code knows to start using numbered parameters at 2.
func (s *TraceDigestsProvider) matchingTracesStatement(ctx context.Context) (string, []interface{}) {
	var keyFilters []common.FilterSets
	q := common.GetQuery(ctx)
	isSpanner := s.dbType == config.Spanner
	for key, values := range q.TraceValues {
		if key == types.CorpusField {
			continue
		}
		if key != sql.Sanitize(key) {
			sklog.Infof("key %q did not pass sanitization", key)
			continue
		}
		keyFilters = append(keyFilters, common.FilterSets{Key: key, Values: values})
	}
	ignoreStatuses := []bool{false}
	if q.IncludeIgnoredTraces {
		ignoreStatuses = append(ignoreStatuses, true)
	}
	corpus := sql.Sanitize(q.TraceValues[types.CorpusField][0])
	materializedView := s.materializedViewProvider.GetMaterializedView(UnignoredRecentTracesView, corpus)
	if q.OnlyIncludeDigestsProducedAtHead {
		if len(keyFilters) == 0 {
			if materializedView != "" && !q.IncludeIgnoredTraces {
				return "MatchingTraces AS (SELECT * FROM " + materializedView + ")", nil
			}
			// Corpus is being used as a string
			args := []interface{}{common.GetFirstCommitID(ctx), ignoreStatuses, corpus}
			return `
MatchingTraces AS (
	SELECT trace_id, grouping_id, digest FROM ValuesAtHead
	WHERE most_recent_commit_id >= $2 AND
		matches_any_ignore_rule = ANY($3) AND
		corpus = $4
)`, args
		}
		if materializedView != "" && !q.IncludeIgnoredTraces {
			return JoinedTracesStatement(keyFilters, corpus, isSpanner) + `
MatchingTraces AS (
	SELECT JoinedTraces.trace_id, grouping_id, digest FROM ` + materializedView + `
	JOIN JoinedTraces ON JoinedTraces.trace_id = ` + materializedView + `.trace_id
)`, nil
		}
		// Corpus is being used as a JSONB value here
		args := []interface{}{common.GetFirstCommitID(ctx), ignoreStatuses}
		return JoinedTracesStatement(keyFilters, corpus, isSpanner) + `
MatchingTraces AS (
	SELECT ValuesAtHead.trace_id, grouping_id, digest FROM ValuesAtHead
	JOIN JoinedTraces ON ValuesAtHead.trace_id = JoinedTraces.trace_id
	WHERE most_recent_commit_id >= $2 AND
		matches_any_ignore_rule = ANY($3)
)`, args
	} else {
		if len(keyFilters) == 0 {
			if materializedView != "" && !q.IncludeIgnoredTraces {
				args := []interface{}{common.GetFirstTileID(ctx)}
				return `MatchingTraces AS (
	SELECT DISTINCT TiledTraceDigests.trace_id, TiledTraceDigests.grouping_id, TiledTraceDigests.digest
	FROM TiledTraceDigests
	JOIN ` + materializedView + ` ON ` + materializedView + `.trace_id = TiledTraceDigests.trace_id
	WHERE tile_id >= $2
)`, args
			}
			// Corpus is being used as a string
			args := []interface{}{common.GetFirstCommitID(ctx), ignoreStatuses, corpus, common.GetFirstTileID(ctx)}
			return `
TracesOfInterest AS (
	SELECT trace_id, grouping_id FROM ValuesAtHead
	WHERE matches_any_ignore_rule = ANY($3) AND
		most_recent_commit_id >= $2 AND
		corpus = $4
),
MatchingTraces AS (
	SELECT DISTINCT TiledTraceDigests.trace_id, TracesOfInterest.grouping_id, TiledTraceDigests.digest
	FROM TiledTraceDigests
	JOIN TracesOfInterest ON TracesOfInterest.trace_id = TiledTraceDigests.trace_id
	WHERE tile_id >= $5
)
`, args
		}
		if materializedView != "" && !q.IncludeIgnoredTraces {
			args := []interface{}{common.GetFirstTileID(ctx)}
			return JoinedTracesStatement(keyFilters, corpus, isSpanner) + `
TracesOfInterest AS (
	SELECT JoinedTraces.trace_id, grouping_id FROM ` + materializedView + `
	JOIN JoinedTraces ON JoinedTraces.trace_id = ` + materializedView + `.trace_id
),
MatchingTraces AS (
	SELECT DISTINCT TiledTraceDigests.trace_id, TracesOfInterest.grouping_id, TiledTraceDigests.digest
	FROM TiledTraceDigests
	JOIN TracesOfInterest on TracesOfInterest.trace_id = TiledTraceDigests.trace_id
	WHERE tile_id >= $2
)`, args
		}
		// Corpus is being used as a JSONB value here
		args := []interface{}{common.GetFirstTileID(ctx), ignoreStatuses}
		return JoinedTracesStatement(keyFilters, corpus, isSpanner) + `
TracesOfInterest AS (
	SELECT Traces.trace_id, grouping_id FROM Traces
	JOIN JoinedTraces ON Traces.trace_id = JoinedTraces.trace_id
	WHERE matches_any_ignore_rule = ANY($3)
),
MatchingTraces AS (
	SELECT DISTINCT TiledTraceDigests.trace_id, TracesOfInterest.grouping_id, TiledTraceDigests.digest
	FROM TiledTraceDigests
	JOIN TracesOfInterest on TracesOfInterest.trace_id = TiledTraceDigests.trace_id
	WHERE tile_id >= $2
)`, args
	}
}

// JoinedTracesStatement returns a SQL snippet that includes a WITH table called JoinedTraces.
// This table contains just the trace_ids that match the given filters. filters is expected to
// have keys which passed sanitization (it will sanitize the values). The snippet will include
// other tables that will be unioned and intersected to create the appropriate rows. This is
// similar to the technique we use for ignore rules, chosen to maximize consistent performance
// by using the inverted key index. The keys and values are hardcoded into the string instead
// of being passed in as arguments because kjlubick@ was not able to use the placeholder values
// to compare JSONB types removed from a JSONB object to a string while still using the indexes.
func JoinedTracesStatement(filters []common.FilterSets, corpus string, isSpanner bool) string {
	statement := ""
	for i, filter := range filters {
		statement += fmt.Sprintf("U%d AS (\n", i)
		for j, value := range filter.Values {
			if j != 0 {
				statement += "\tUNION\n"
			}
			if isSpanner {
				statement += fmt.Sprintf("\tSELECT trace_id FROM Traces WHERE keys ->> '%s' = '%s'\n", filter.Key, sql.Sanitize(value))
			} else {
				statement += fmt.Sprintf("\tSELECT trace_id FROM Traces WHERE keys -> '%s' = '%q'\n", filter.Key, sql.Sanitize(value))
			}

		}
		statement += "),\n"
	}
	statement += "JoinedTraces AS (\n"
	for i := range filters {
		statement += fmt.Sprintf("\tSELECT trace_id FROM U%d\n\tINTERSECT\n", i)
	}
	// Include a final intersect for the corpus. The calling logic will make sure a JSONB value
	// (i.e. a quoted string) is in the arguments slice.
	if isSpanner {
		statement += "\tSELECT trace_id FROM Traces where keys ->> 'source_type' = '" + corpus + "'\n),\n"
	} else {
		statement += "\tSELECT trace_id FROM Traces where keys -> 'source_type' = '\"" + corpus + "\"'\n),\n"
	}

	return statement
}
