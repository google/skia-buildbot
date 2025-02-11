package caching

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/types"
)

// matchingTracesCacheDataProvider provides a struct to manage caching data for matching traces in search requests.
type matchingTracesCacheDataProvider struct {
	db           *pgxpool.Pool
	corpora      []string
	commitWindow int
	mutex        sync.RWMutex
	dbType       config.DatabaseType

	// This caches the trace ids that are publicly visible.
	publiclyVisibleTraces map[schema.MD5Hash]struct{}
	isPublicView          bool
}

// MatchingTracesQueryContext provides a struct representing the search query.
type MatchingTracesQueryContext struct {
	IncludeUntriaged                 bool
	IncludeNegative                  bool
	IncludePositive                  bool
	IncludeIgnored                   bool
	OnlyIncludeDigestsProducedAtHead bool
	Corpus                           string
	TraceValues                      paramtools.ParamSet
}

// NewMatchingTracesCacheDataProvider returns a new instance of the matchingTracesCacheDataProvider.
func NewMatchingTracesCacheDataProvider(db *pgxpool.Pool, corpora []string, commitWindow int) *matchingTracesCacheDataProvider {
	return &matchingTracesCacheDataProvider{
		db:           db,
		corpora:      corpora,
		commitWindow: commitWindow,
	}
}

// SetDatabaseType sets the database type for the current configuration.
func (s *matchingTracesCacheDataProvider) SetDatabaseType(dbType config.DatabaseType) {
	s.dbType = dbType
}

// SetPublicTraces sets the given traces as the publicly visible ones.
func (s *matchingTracesCacheDataProvider) SetPublicTraces(traces map[schema.MD5Hash]struct{}) {
	s.isPublicView = true
	s.publiclyVisibleTraces = traces
}

// GetCacheData returns the matchingTraces data to be written to the cache.
func (prov *matchingTracesCacheDataProvider) GetCacheData(ctx context.Context, firstCommitId string) (map[string]string, error) {
	cacheMap := map[string]string{}
	for _, corpus := range prov.corpora {
		sklog.Infof("Getting data for corpus: %s", corpus)
		// Create query contexts for the data to be cached. Note that we are only caching
		// data on HEAD branch and just one flag set at a time. This is intentional to
		// avoid storing too many combinations of the data in the cache. If there are
		// combinations requested, we can retrieve the data for each flag from the cache
		// and perform the union in memory. For example, if the request is for
		// Untriaged + Negative digests, we retrieve those separately and return the
		// combined set. We can take advantage of the fact that these are mutually exclusive.
		queryContexts := map[string]MatchingTracesQueryContext{
			// Untriaged digests.
			MatchingUntriagedTracesKey(corpus): {
				OnlyIncludeDigestsProducedAtHead: true,
				IncludeUntriaged:                 true,
				Corpus:                           corpus,
				TraceValues: paramtools.ParamSet{
					"source_type": []string{corpus},
				},
			},
			// Negative digests.
			MatchingNegativeTracesKey(corpus): {
				OnlyIncludeDigestsProducedAtHead: true,
				IncludeNegative:                  true,
				Corpus:                           corpus,
				TraceValues: paramtools.ParamSet{
					"source_type": []string{corpus},
				},
			},
			// Positive digests.
			MatchingPositiveTracesKey(corpus): {
				OnlyIncludeDigestsProducedAtHead: true,
				IncludePositive:                  true,
				Corpus:                           corpus,
				TraceValues: paramtools.ParamSet{
					"source_type": []string{corpus},
				},
			},
			// Ignored digests.
			MatchingIgnoredTracesKey(corpus): {
				OnlyIncludeDigestsProducedAtHead: true,
				IncludeIgnored:                   true,
				Corpus:                           corpus,
				TraceValues: paramtools.ParamSet{
					"source_type": []string{corpus},
				},
			},
		}

		for cacheKey, queryContext := range queryContexts {
			sklog.Infof("Getting data for context: %v", queryContext)
			digests, err := prov.getMatchingDigestsAndTracesFromDB(ctx, queryContext)
			if err != nil {
				return nil, skerr.Wrapf(err, "Error getting untriaged digests.")
			}

			if len(digests) > 0 {
				jsonStr, err := common.ToJSON(digests)
				if err != nil {
					return nil, skerr.Wrapf(err, "Error converting digests into json.")
				}
				sklog.Infof("Adding %d matching digests to cache key: %s.", len(digests), cacheKey)
				cacheMap[cacheKey] = jsonStr
			}
		}
	}

	return cacheMap, nil
}

// getMatchingDigestsAndTracesFromDB returns matching digests and traces from the database.
func (s *matchingTracesCacheDataProvider) getMatchingDigestsAndTracesFromDB(ctx context.Context, queryContext MatchingTracesQueryContext) ([]common.DigestWithTraceAndGrouping, error) {
	ctx, span := trace.StartSpan(ctx, "getMatchingDigestsAndTracesFromDB_cacheManager")
	defer span.End()

	statement := `WITH
MatchingDigests AS (
	SELECT grouping_id, digest FROM Expectations
	WHERE label = ANY($1)
),`
	tracesBlock, args := s.matchingTracesStatement(ctx, queryContext)
	statement += tracesBlock
	statement += `
SELECT trace_id, MatchingDigests.grouping_id, MatchingTraces.digest FROM
MatchingDigests
JOIN
MatchingTraces ON MatchingDigests.grouping_id = MatchingTraces.grouping_id AND
  MatchingDigests.digest = MatchingTraces.digest`

	var triageStatuses []schema.ExpectationLabel
	if queryContext.IncludeUntriaged {
		triageStatuses = append(triageStatuses, schema.LabelUntriaged)
	}
	if queryContext.IncludeNegative {
		triageStatuses = append(triageStatuses, schema.LabelNegative)
	}
	if queryContext.IncludePositive {
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

// matchingTracesStatement returns a SQL snippet that includes a WITH table called MatchingTraces.
// This table will have rows containing trace_id, grouping_id, and digest of traces that match
// the given search criteria. The second parameter is the arguments that need to be included
// in the query. This code knows to start using numbered parameters at 2.
func (s *matchingTracesCacheDataProvider) matchingTracesStatement(ctx context.Context, queryContext MatchingTracesQueryContext) (string, []interface{}) {
	var keyFilters []common.FilterSets
	isSpanner := s.dbType == config.Spanner
	for key, values := range queryContext.TraceValues {
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
	if queryContext.IncludeIgnored {
		ignoreStatuses = append(ignoreStatuses, true)
	}
	corpus := sql.Sanitize(queryContext.TraceValues[types.CorpusField][0])
	if queryContext.OnlyIncludeDigestsProducedAtHead {
		if len(keyFilters) == 0 {
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
