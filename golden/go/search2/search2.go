// Package search2 encapsulates various queries we make against Gold's data. It is backed
// by the SQL database and aims to replace the current search package.
package search2

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/search"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
	web_frontend "go.skia.org/infra/golden/go/web/frontend"
)

type API interface {
	// NewAndUntriagedSummaryForCL returns a summarized look at the new digests produced by a CL
	// (that is, digests not currently on the primary branch for this grouping at all) as well as
	// how many of the newly produced digests are currently untriaged.
	NewAndUntriagedSummaryForCL(ctx context.Context, qCLID string) (NewAndUntriagedSummary, error)

	// ChangelistLastUpdated returns the timestamp that the given CL was updated. It returns an
	// error if the CL does not exist.
	ChangelistLastUpdated(ctx context.Context, qCLID string) (time.Time, error)
}

// NewAndUntriagedSummary is a summary of the results associated with a given CL. It focuses on
// the untriaged and new images produced.
type NewAndUntriagedSummary struct {
	// ChangelistID is the nonqualified id of the CL.
	ChangelistID string
	// PatchsetSummaries is a summary for all Patchsets for which we have data.
	PatchsetSummaries []PatchsetNewAndUntriagedSummary
	// LastUpdated returns the timestamp of the CL, which corresponds to the last datapoint for
	// this CL.
	LastUpdated time.Time
	// Outdated is set to true if the value that was previously cached was out of date and is
	// currently being recalculated. We do this to return something quickly to the user (even if
	// something like the
	Outdated bool
}

// PatchsetNewAndUntriagedSummary is the summary for a specific PS. It focuses on the untriaged
// and new images produced.
type PatchsetNewAndUntriagedSummary struct {
	// NewImages is the number of new images (digests) that were produced by this patchset by
	// non-ignored traces and not seen on the primary branch.
	NewImages int
	// NewUntriagedImages is the number of NewImages which are still untriaged. It is less than or
	// equal to NewImages.
	NewUntriagedImages int
	// TotalUntriagedImages is the number of images produced by this patchset by non-ignored traces
	// that are untriaged. This includes images that are untriaged and observed on the primary
	// branch (i.e. might not be the fault of this CL/PS). It is greater than or equal to
	// NewUntriagedImages.
	TotalUntriagedImages int
	// PatchsetID is the nonqualified id of the patchset. This is usually a git hash.
	PatchsetID string
	// PatchsetOrder is represents the chronological order the patchsets are in. It starts at 1.
	PatchsetOrder int
}

const (
	commitCacheSize          = 5_000
	optionsGroupingCacheSize = 50_000
	traceCacheSize           = 1_000_000
)

type Impl struct {
	db           *pgxpool.Pool
	windowLength int

	// Protects the caches
	mutex sync.RWMutex
	// This caches the digests seen per grouping on the primary branch.
	digestsOnPrimary map[groupingDigestKey]struct{}

	commitCache          *lru.Cache
	optionsGroupingCache *lru.Cache
	traceCache           *lru.Cache
}

// New returns an implementation of API.
func New(sqlDB *pgxpool.Pool, windowLength int) *Impl {
	cc, err := lru.New(commitCacheSize)
	if err != nil {
		panic(err) // should only happen if commitCacheSize is negative.
	}
	gc, err := lru.New(optionsGroupingCacheSize)
	if err != nil {
		panic(err) // should only happen if optionsGroupingCacheSize is negative.
	}
	tc, err := lru.New(traceCacheSize)
	if err != nil {
		panic(err) // should only happen if traceCacheSize is negative.
	}
	return &Impl{
		db:                   sqlDB,
		windowLength:         windowLength,
		digestsOnPrimary:     map[groupingDigestKey]struct{}{},
		commitCache:          cc,
		optionsGroupingCache: gc,
		traceCache:           tc,
	}
}

type groupingDigestKey struct {
	groupingID schema.MD5Hash
	digest     schema.MD5Hash
}

// StartCacheProcess loads the caches used for searching and starts a gorotuine to keep those
// up to date.
func (s *Impl) StartCacheProcess(ctx context.Context, interval time.Duration, commitsWithData int) error {
	if err := s.updateCaches(ctx, commitsWithData); err != nil {
		return skerr.Wrapf(err, "setting up initial cache values")
	}
	go util.RepeatCtx(ctx, interval, func(ctx context.Context) {
		err := s.updateCaches(ctx, commitsWithData)
		if err != nil {
			sklog.Errorf("Could not update caches: %s", err)
		}
	})
	return nil
}

// updateCaches loads the digestsOnPrimary cache.
func (s *Impl) updateCaches(ctx context.Context, commitsWithData int) error {
	ctx, span := trace.StartSpan(ctx, "search2_UpdateCaches", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	tile, err := s.getStartingTile(ctx, commitsWithData)
	if err != nil {
		return skerr.Wrapf(err, "geting tile to search")
	}
	onPrimary, err := s.getDigestsOnPrimary(ctx, tile)
	if err != nil {
		return skerr.Wrapf(err, "getting digests on primary branch")
	}
	s.mutex.Lock()
	s.digestsOnPrimary = onPrimary
	s.mutex.Unlock()
	sklog.Infof("Digests on Primary cache refreshed with %d entries", len(onPrimary))
	return nil
}

// getStartingTile returns the commit ID which is the beginning of the tile of interest (so we
// get enough data to do our comparisons).
func (s *Impl) getStartingTile(ctx context.Context, commitsWithDataToSearch int) (schema.TileID, error) {
	ctx, span := trace.StartSpan(ctx, "getStartingTile")
	defer span.End()
	if commitsWithDataToSearch <= 0 {
		return 0, nil
	}
	row := s.db.QueryRow(ctx, `SELECT tile_id FROM CommitsWithData
AS OF SYSTEM TIME '-0.1s'
ORDER BY commit_id DESC
LIMIT 1 OFFSET $1`, commitsWithDataToSearch)
	var lc pgtype.Int4
	if err := row.Scan(&lc); err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil // not enough commits seen, so start at tile 0.
		}
		return 0, skerr.Wrapf(err, "getting latest commit")
	}
	if lc.Status == pgtype.Null {
		// There are no commits with data, so start at tile 0.
		return 0, nil
	}
	return schema.TileID(lc.Int), nil
}

// getDigestsOnPrimary returns a map of all distinct digests on the primary branch.
func (s *Impl) getDigestsOnPrimary(ctx context.Context, tile schema.TileID) (map[groupingDigestKey]struct{}, error) {
	ctx, span := trace.StartSpan(ctx, "getDigestsOnPrimary")
	defer span.End()
	rows, err := s.db.Query(ctx, `
SELECT DISTINCT grouping_id, digest FROM
TiledTraceDigests WHERE tile_id >= $1`, tile)
	if err != nil {
		if err == pgx.ErrNoRows {
			return map[groupingDigestKey]struct{}{}, nil
		}
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	rv := map[groupingDigestKey]struct{}{}
	var digest schema.DigestBytes
	var grouping schema.GroupingID
	var key groupingDigestKey
	keyGrouping := key.groupingID[:]
	keyDigest := key.digest[:]
	for rows.Next() {
		err := rows.Scan(&grouping, &digest)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		copy(keyGrouping, grouping)
		copy(keyDigest, digest)
		rv[key] = struct{}{}
	}
	return rv, nil
}

// NewAndUntriagedSummaryForCL queries all the patchsets in parallel (to keep the query less
// complex). If there are no patchsets for the provided CL, it returns an error.
func (s *Impl) NewAndUntriagedSummaryForCL(ctx context.Context, qCLID string) (NewAndUntriagedSummary, error) {
	ctx, span := trace.StartSpan(ctx, "search2_NewAndUntriagedSummaryForCL")
	defer span.End()

	patchsets, err := s.getPatchsets(ctx, qCLID)
	if err != nil {
		return NewAndUntriagedSummary{}, skerr.Wrap(err)
	}
	if len(patchsets) == 0 {
		return NewAndUntriagedSummary{}, skerr.Fmt("CL %q not found", qCLID)
	}

	eg, ctx := errgroup.WithContext(ctx)
	rv := make([]PatchsetNewAndUntriagedSummary, len(patchsets))
	for i, p := range patchsets {
		idx, ps := i, p
		eg.Go(func() error {
			sum, err := s.getSummaryForPS(ctx, qCLID, ps.id)
			if err != nil {
				return skerr.Wrap(err)
			}
			sum.PatchsetID = sql.Unqualify(ps.id)
			sum.PatchsetOrder = ps.order
			rv[idx] = sum
			return nil
		})
	}
	var updatedTS time.Time
	eg.Go(func() error {
		var err error
		updatedTS, err = s.ChangelistLastUpdated(ctx, qCLID)
		return skerr.Wrap(err)
	})
	if err := eg.Wait(); err != nil {
		return NewAndUntriagedSummary{}, skerr.Wrapf(err, "Getting counts for CL %q and %d PS", qCLID, len(patchsets))
	}
	return NewAndUntriagedSummary{
		ChangelistID:      sql.Unqualify(qCLID),
		PatchsetSummaries: rv,
		LastUpdated:       updatedTS.UTC(),
	}, nil
}

type psIDAndOrder struct {
	id    string
	order int
}

// getPatchsets returns the qualified ids and orders of the patchsets sorted by ps_order.
func (s *Impl) getPatchsets(ctx context.Context, qualifiedID string) ([]psIDAndOrder, error) {
	ctx, span := trace.StartSpan(ctx, "getPatchsets")
	defer span.End()
	rows, err := s.db.Query(ctx, `SELECT patchset_id, ps_order
FROM Patchsets WHERE changelist_id = $1 ORDER BY ps_order ASC`, qualifiedID)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting summary for cl %q", qualifiedID)
	}
	defer rows.Close()
	var rv []psIDAndOrder
	for rows.Next() {
		var row psIDAndOrder
		if err := rows.Scan(&row.id, &row.order); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, row)
	}
	return rv, nil
}

// getSummaryForPS looks at all the data produced for a given PS and returns the a summary of the
// newly produced digests and untriaged digests.
func (s *Impl) getSummaryForPS(ctx context.Context, clid, psID string) (PatchsetNewAndUntriagedSummary, error) {
	ctx, span := trace.StartSpan(ctx, "getSummaryForPS")
	defer span.End()
	const statement = `
WITH
  CLDigests AS (
    SELECT secondary_branch_trace_id, digest, grouping_id
    FROM SecondaryBranchValues
    WHERE branch_name = $1 and version_name = $2
  ),
  NonIgnoredCLDigests AS (
    -- We only want to count a digest once per grouping, no matter how many times it shows up
    -- because group those together (by trace) in the frontend UI.
    SELECT DISTINCT digest, CLDigests.grouping_id
    FROM CLDigests
    JOIN Traces
    ON secondary_branch_trace_id = trace_id
    WHERE Traces.matches_any_ignore_rule = False
  ),
  CLExpectations AS (
    SELECT grouping_id, digest, label
    FROM SecondaryBranchExpectations
    WHERE branch_name = $1
  ),
  LabeledDigests AS (
    SELECT NonIgnoredCLDigests.grouping_id, NonIgnoredCLDigests.digest, COALESCE(CLExpectations.label, COALESCE(Expectations.label, 'u')) as label
    FROM NonIgnoredCLDigests
    LEFT JOIN Expectations
    ON NonIgnoredCLDigests.grouping_id = Expectations.grouping_id AND
      NonIgnoredCLDigests.digest = Expectations.digest
    LEFT JOIN CLExpectations
    ON NonIgnoredCLDigests.grouping_id = CLExpectations.grouping_id AND
      NonIgnoredCLDigests.digest = CLExpectations.digest
  )
SELECT * FROM LabeledDigests;`

	rows, err := s.db.Query(ctx, statement, clid, psID)
	if err != nil {
		return PatchsetNewAndUntriagedSummary{}, skerr.Wrapf(err, "getting summary for ps %q in cl %q", psID, clid)
	}
	defer rows.Close()
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	var digest schema.DigestBytes
	var grouping schema.GroupingID
	var label schema.ExpectationLabel
	var key groupingDigestKey
	keyGrouping := key.groupingID[:]
	keyDigest := key.digest[:]
	var rv PatchsetNewAndUntriagedSummary

	for rows.Next() {
		if err := rows.Scan(&grouping, &digest, &label); err != nil {
			return PatchsetNewAndUntriagedSummary{}, skerr.Wrap(err)
		}
		copy(keyGrouping, grouping)
		copy(keyDigest, digest)
		_, isExisting := s.digestsOnPrimary[key]
		if !isExisting {
			rv.NewImages++
		}
		if label == schema.LabelUntriaged {
			rv.TotalUntriagedImages++
			if !isExisting {
				rv.NewUntriagedImages++
			}
		}
	}
	return rv, nil
}

// ChangelistLastUpdated implements the API interface.
func (s *Impl) ChangelistLastUpdated(ctx context.Context, qCLID string) (time.Time, error) {
	ctx, span := trace.StartSpan(ctx, "search2_ChangelistLastUpdated")
	defer span.End()
	var updatedTS time.Time
	row := s.db.QueryRow(ctx, `WITH
LastSeenData AS (
	SELECT last_ingested_data as ts FROM Changelists
	WHERE changelist_id = $1
),
LatestTriageAction AS (
	SELECT triage_time as ts FROM ExpectationRecords
	WHERE branch_name = $1
    ORDER BY triage_time DESC LIMIT 1
)
SELECT ts FROM LastSeenData
UNION
SELECT ts FROM LatestTriageAction
ORDER BY ts DESC LIMIT 1
`, qCLID)
	if err := row.Scan(&updatedTS); err != nil {
		return time.Time{}, skerr.Wrapf(err, "Getting last updated ts for cl %q", qCLID)
	}
	return updatedTS.UTC(), nil
}

// Search implements the SearchAPI interface.
func (s *Impl) Search(ctx context.Context, q *query.Search) (*frontend.SearchResponse, error) {
	ctx, span := trace.StartSpan(ctx, "search2_Search")
	defer span.End()

	ctx = context.WithValue(ctx, queryKey, *q)
	ctx, err := s.addCommitsData(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	commits, err := s.getCommits(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Find all digests and traces that match the given search criteria.
	traceDigests, err := s.getMatchingDigestsAndTraces(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Lookup the closest diffs to the given digests. This returns a subset according to the
	// limit and offset in the query.
	closestDiffs, allClosestLabels, err := s.getClosestDiffs(ctx, traceDigests)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Go fetch history and paramset (within this grouping)
	paramsetsByDigest, err := s.getParamsetsForDigests(ctx, closestDiffs)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Flesh out the trace history with enough data to draw the dots diagram on the frontend.
	results, err := s.fillOutTraceHistory(ctx, closestDiffs)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Create the mapping that allows us to bulk triage all results (not for just the ones shown).
	bulkTriageData, err := s.convertBulkTriageData(ctx, allClosestLabels)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Fill in the paramsets of the reference images.
	for _, sr := range results {
		for _, srdd := range sr.RefDiffs {
			if srdd != nil {
				srdd.ParamSet = paramsetsByDigest[srdd.Digest]
			}
		}
	}

	return &frontend.SearchResponse{
		Results:        results,
		Offset:         q.Offset,
		Size:           len(allClosestLabels),
		BulkTriageData: bulkTriageData,
		Commits:        commits,
	}, nil
}

// To avoid piping a lot of info about the commits in the most recent window through all the
// functions in the search pipeline, we attach them as values to the context.
type searchContextKey string

const (
	actualWindowLengthKey = searchContextKey("actualWindowLengthKey")
	commitToIdxKey        = searchContextKey("commitToIdxKey")
	firstCommitIDKey      = searchContextKey("firstCommitIDKey")
	firstTileIDKey        = searchContextKey("firstTileIDKey")
	queryKey              = searchContextKey("query")
)

func getFirstCommitID(ctx context.Context) schema.CommitID {
	return ctx.Value(firstCommitIDKey).(schema.CommitID)
}

func getFirstTileID(ctx context.Context) schema.TileID {
	return ctx.Value(firstTileIDKey).(schema.TileID)
}

func getCommitToIdxMap(ctx context.Context) map[schema.CommitID]int {
	return ctx.Value(commitToIdxKey).(map[schema.CommitID]int)
}

func getActualWindowLength(ctx context.Context) int {
	return ctx.Value(actualWindowLengthKey).(int)
}

func getQuery(ctx context.Context) query.Search {
	return ctx.Value(queryKey).(query.Search)
}

// addCommitsData finds the current sliding window of data (The last N commits) and adds the
// derived data to the given context and returns it.
func (s *Impl) addCommitsData(ctx context.Context) (context.Context, error) {
	// Note: need to rename the context here to avoid adding the span data to all other contexts.
	sCtx, span := trace.StartSpan(ctx, "addCommitsData")
	defer span.End()
	const statement = `SELECT commit_id, tile_id FROM
CommitsWithData ORDER BY commit_id DESC LIMIT $1`
	rows, err := s.db.Query(sCtx, statement, s.windowLength)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	ids := make([]schema.CommitID, 0, s.windowLength)
	var firstObservedTile schema.TileID
	for rows.Next() {
		var id schema.CommitID
		if err := rows.Scan(&id, &firstObservedTile); err != nil {
			return nil, skerr.Wrap(err)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, skerr.Fmt("No commits with data")
	}
	// ids is ordered most recent commit to last commit at this point
	ctx = context.WithValue(ctx, actualWindowLengthKey, len(ids))
	ctx = context.WithValue(ctx, firstCommitIDKey, ids[len(ids)-1])
	ctx = context.WithValue(ctx, firstTileIDKey, firstObservedTile)
	idToIndex := map[schema.CommitID]int{}
	idx := 0
	for i := len(ids) - 1; i >= 0; i-- {
		idToIndex[ids[i]] = idx
		idx++
	}
	ctx = context.WithValue(ctx, commitToIdxKey, idToIndex)
	return ctx, nil
}

type stageOneResult struct {
	traceID    schema.TraceID
	groupingID schema.GroupingID
	digest     schema.DigestBytes
}

// getMatchingDigestsAndTraces returns the tuples of digest+traceID that match the given query.
// The order of the result is abitrary.
func (s *Impl) getMatchingDigestsAndTraces(ctx context.Context) ([]stageOneResult, error) {
	ctx, span := trace.StartSpan(ctx, "getMatchingDigestsAndTraces")
	defer span.End()
	statement := `WITH
MatchingDigests AS (
    SELECT grouping_id, digest FROM Expectations
    WHERE label = ANY($1)
),`
	tracesBlock, args := matchingTracesStatement(ctx)
	statement += tracesBlock
	statement += `
SELECT trace_id, MatchingDigests.grouping_id, MatchingTraces.digest FROM
MatchingDigests
JOIN
MatchingTraces ON MatchingDigests.grouping_id = MatchingTraces.grouping_id AND
  MatchingDigests.digest = MatchingTraces.digest`

	q := getQuery(ctx)
	var triageStatuses []schema.ExpectationLabel
	if q.IncludeUntriagedDigests {
		triageStatuses = append(triageStatuses, schema.LabelUntriaged)
	}
	if q.IncludeNegativeDigests {
		triageStatuses = append(triageStatuses, schema.LabelNegative)
	}
	if q.IncludePositiveDigests {
		triageStatuses = append(triageStatuses, schema.LabelPositive)
	}
	arguments := append([]interface{}{triageStatuses}, args...)

	rows, err := s.db.Query(ctx, statement, arguments...)
	if err != nil {
		return nil, skerr.Wrapf(err, "searching for query %v with args %v", q, arguments)
	}
	defer rows.Close()
	var rv []stageOneResult
	for rows.Next() {
		var row stageOneResult
		if err := rows.Scan(&row.traceID, &row.groupingID, &row.digest); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, row)
	}
	return rv, nil
}

type filterSets struct {
	key    string
	values []string
}

// matchingTracesStatement returns a SQL snippet that includes a WITH table called MatchingTraces.
// This table will have rows containing trace_id, grouping_id, and digest of traces that match
// the given search criteria. The second parameter is the arguments that need to be included
// in the query. This code knows to start using numbered parameters at 2.
func matchingTracesStatement(ctx context.Context) (string, []interface{}) {
	var keyFilters []filterSets
	q := getQuery(ctx)
	for key, values := range q.TraceValues {
		if key == types.CorpusField {
			continue
		}
		if key != sql.Sanitize(key) {
			sklog.Infof("key %q did not pass sanitization", key)
			continue
		}
		keyFilters = append(keyFilters, filterSets{key: key, values: values})
	}
	ignoreStatuses := []bool{false}
	if q.IncludeIgnoredTraces {
		ignoreStatuses = append(ignoreStatuses, true)
	}
	args := []interface{}{getFirstCommitID(ctx), ignoreStatuses}

	if len(keyFilters) == 0 {
		// Corpus is being used as a string
		args = append(args, q.TraceValues[types.CorpusField][0])
		return `
MatchingTraces AS (
    SELECT trace_id, grouping_id, digest FROM ValuesAtHead
	WHERE most_recent_commit_id >= $2 AND
    	matches_any_ignore_rule = ANY($3) AND
    	corpus = $4
)`, args
	}
	// Corpus is being used as a JSONB value here
	args = append(args, `"`+q.TraceValues[types.CorpusField][0]+`"`)
	return joinedTracesStatement(keyFilters) + `
MatchingTraces AS (
    SELECT ValuesAtHead.trace_id, grouping_id, digest FROM ValuesAtHead
	JOIN JoinedTraces ON ValuesAtHead.trace_id = JoinedTraces.trace_id
	WHERE most_recent_commit_id >= $2 AND
    	matches_any_ignore_rule = ANY($3)
)`, args
}

// joinedTracesStatement returns a SQL snippet that includes a WITH table called JoinedTraces.
// This table contains just the trace_ids that match the given filters. filters is expected to
// have keys which passed sanitization (it will sanitize the values). The snippet will include
// other tables that will be unioned and intersected to create the appropriate rows. This is
// similar to the technique we use for ingore rules, chosen to maximize consistent performance
// by using the inverted key index. The keys and values are hardcoded into the string instead
// of being passed in as arguments because kjlubick@ was not able to use the placeholder values
//to compare JSONB types removed from a JSONB object to a string while still using the indexes.
func joinedTracesStatement(filters []filterSets) string {
	statement := ""
	for i, filter := range filters {
		statement += fmt.Sprintf("U%d AS (\n", i)
		for j, value := range filter.values {
			if j != 0 {
				statement += "\tUNION\n"
			}
			statement += fmt.Sprintf("\tSELECT trace_id FROM Traces WHERE keys -> '%s' = '%q'\n", filter.key, sql.Sanitize(value))
		}
		statement += "),\n"
	}
	statement += "JoinedTraces AS (\n"
	for i := range filters {
		statement += fmt.Sprintf("\tSELECT trace_id FROM U%d\n\tINTERSECT\n", i)
	}
	// Include a final intersect for the corpus. The calling logic will make sure a JSONB value
	// (i.e. a quoted string) is in the arguments slice.
	statement += "\tSELECT trace_id FROM Traces where keys -> 'source_type' = $4\n),\n"
	return statement
}

type stageTwoResult struct {
	leftDigest      schema.DigestBytes
	groupingID      schema.GroupingID
	rightDigests    []schema.DigestBytes
	traceIDs        []schema.TraceID
	closestDigest   *frontend.SRDiffDigest // These won't have ParamSets yet
	closestPositive *frontend.SRDiffDigest
	closestNegative *frontend.SRDiffDigest
}

// getClosestDiffs returns information about the closest triaged digests for each result in the
// input. We are able to batch the queries by grouping and do so for better performance.
// While this returns a subset of data as defined by the query, it also returns sufficient
// information to bulk-triage all of the inputs.
func (s *Impl) getClosestDiffs(ctx context.Context, inputs []stageOneResult) ([]stageTwoResult, map[groupingDigestKey]expectations.Label, error) {
	ctx, span := trace.StartSpan(ctx, "getClosestDiffs")
	defer span.End()
	byGrouping := map[schema.MD5Hash][]stageOneResult{}
	byDigest := map[schema.MD5Hash]stageTwoResult{}
	for _, input := range inputs {
		gID := sql.AsMD5Hash(input.groupingID)
		byGrouping[gID] = append(byGrouping[gID], input)
		dID := sql.AsMD5Hash(input.digest)
		bd := byDigest[dID]
		bd.leftDigest = input.digest
		bd.groupingID = input.groupingID
		bd.traceIDs = append(bd.traceIDs, input.traceID)
		byDigest[dID] = bd
	}

	for groupingID, inputs := range byGrouping {
		const statement = `WITH
ObservedDigestsInTile AS (
	SELECT digest FROM TiledTraceDigests
    WHERE grouping_id = $2 and tile_id >= $3
),
PositiveOrNegativeDigests AS (
    SELECT digest, label FROM Expectations
    WHERE grouping_id = $2 AND (label = 'n' OR label = 'p')
),
ComparisonBetweenUntriagedAndObserved AS (
    SELECT DiffMetrics.* FROM DiffMetrics
    JOIN ObservedDigestsInTile ON DiffMetrics.right_digest = ObservedDigestsInTile.digest
    WHERE left_digest = ANY($1)
)
-- This will return the right_digest with the smallest combined_metric for each left_digest + label
SELECT DISTINCT ON (left_digest, label)
  label, left_digest, right_digest, num_pixels_diff, percent_pixels_diff, max_rgba_diffs,
  combined_metric, dimensions_differ
FROM
  ComparisonBetweenUntriagedAndObserved
JOIN PositiveOrNegativeDigests
  ON ComparisonBetweenUntriagedAndObserved.right_digest = PositiveOrNegativeDigests.digest
ORDER BY left_digest, label, combined_metric ASC, max_channel_diff ASC, right_digest ASC
`
		digests := make([][]byte, 0, len(inputs))
		for _, input := range inputs {
			digests = append(digests, input.digest)
		}
		rows, err := s.db.Query(ctx, statement, digests, groupingID[:], getFirstTileID(ctx))
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		var label schema.ExpectationLabel
		var row schema.DiffMetricRow
		for rows.Next() {
			if err := rows.Scan(&label, &row.LeftDigest, &row.RightDigest, &row.NumPixelsDiff,
				&row.PercentPixelsDiff, &row.MaxRGBADiffs, &row.CombinedMetric,
				&row.DimensionsDiffer); err != nil {
				rows.Close()
				return nil, nil, skerr.Wrap(err)
			}
			srdd := &frontend.SRDiffDigest{
				Digest:           types.Digest(hex.EncodeToString(row.RightDigest)),
				Status:           label.ToExpectation(),
				CombinedMetric:   row.CombinedMetric,
				DimDiffer:        row.DimensionsDiffer,
				MaxRGBADiffs:     row.MaxRGBADiffs,
				NumDiffPixels:    row.NumPixelsDiff,
				PixelDiffPercent: row.PercentPixelsDiff,
				QueryMetric:      row.CombinedMetric,
			}
			leftDigest := sql.AsMD5Hash(row.LeftDigest)
			stageTwo := byDigest[leftDigest]
			stageTwo.rightDigests = append(stageTwo.rightDigests, row.RightDigest)
			if label == schema.LabelPositive {
				stageTwo.closestPositive = srdd
			} else {
				stageTwo.closestNegative = srdd
			}
			if stageTwo.closestNegative != nil && stageTwo.closestPositive != nil {
				if stageTwo.closestPositive.CombinedMetric < stageTwo.closestNegative.CombinedMetric {
					stageTwo.closestDigest = stageTwo.closestPositive
				} else {
					stageTwo.closestDigest = stageTwo.closestNegative
				}
			} else {
				// there is only one type of diff, so it defaults to the closest.
				stageTwo.closestDigest = srdd
			}
			byDigest[leftDigest] = stageTwo
		}
		rows.Close()
	}

	q := getQuery(ctx)
	bulkTriageData := map[groupingDigestKey]expectations.Label{}
	results := make([]stageTwoResult, 0, len(byDigest))
	for _, s2 := range byDigest {
		// Filter out any results without a closest triaged digest (if that option is selected).
		if q.MustIncludeReferenceFilter && s2.closestDigest == nil {
			continue
		}
		if s2.closestDigest != nil {
			// Apply RGBA Filter here - if the closest digest isn't within range, we remove it.
			maxDiff := util.MaxInt(s2.closestDigest.MaxRGBADiffs[:]...)
			if maxDiff < q.RGBAMinFilter || maxDiff > q.RGBAMaxFilter {
				continue
			}
			closestLabel := s2.closestDigest.Status
			key := groupingDigestKey{groupingID: sql.AsMD5Hash(s2.groupingID), digest: sql.AsMD5Hash(s2.leftDigest)}
			bulkTriageData[key] = closestLabel
		}
		results = append(results, s2)

	}
	if q.Offset >= len(results) {
		return nil, bulkTriageData, nil
	}
	sortAsc := q.Sort == query.SortAscending
	sort.Slice(results, func(i, j int) bool {
		if results[i].closestDigest == nil {
			return true // sort results with no reference image to the top
		}
		if results[j].closestDigest == nil {
			return false
		}
		if results[i].closestDigest.CombinedMetric == results[j].closestDigest.CombinedMetric {
			// Tiebreak using digest in ascending order.
			return bytes.Compare(results[i].leftDigest, results[j].leftDigest) < 0
		}
		if sortAsc {
			return results[i].closestDigest.CombinedMetric < results[j].closestDigest.CombinedMetric
		}
		return results[i].closestDigest.CombinedMetric > results[j].closestDigest.CombinedMetric
	})

	if q.Limit <= 0 {
		return results, bulkTriageData, nil
	}
	end := util.MinInt(len(results), q.Offset+q.Limit)
	return results[q.Offset:end], bulkTriageData, nil
}

// getParamsetsForDigests fetches all the traces that produced the digests in the data and
// the keys for these traces as ParamSets. The ParamSets are mapped according to the digest
// which produced them in the given window (or the tiles that approximate this).
func (s *Impl) getParamsetsForDigests(ctx context.Context, inputs []stageTwoResult) (map[types.Digest]paramtools.ParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "getParamsetsForDigests")
	defer span.End()
	var rightDigests []schema.DigestBytes
	for _, input := range inputs {
		rightDigests = append(rightDigests, input.rightDigests...)
	}
	span.AddAttributes(trace.Int64Attribute("num_digests", int64(len(rightDigests))))
	digestToTraces, err := s.addRightTraces(ctx, rightDigests)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	traceToOptions, err := s.getOptionsForTraces(ctx, digestToTraces)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv, err := s.expandTracesIntoParamsets(ctx, digestToTraces, traceToOptions)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

// addRightTraces finds the traces that draw the given digests. We do not need to consider the
// ignore rules or other search constraints because those only apply to the search results
// (that is, the left side).
func (s *Impl) addRightTraces(ctx context.Context, digests []schema.DigestBytes) (map[types.Digest][]schema.TraceID, error) {
	ctx, span := trace.StartSpan(ctx, "addRightTraces")
	defer span.End()
	span.AddAttributes(trace.Int64Attribute("num_right_digests", int64(len(digests))))

	const statement = `SELECT DISTINCT encode(digest, 'hex') as digest, trace_id
FROM TiledTraceDigests WHERE digest = ANY($1) AND tile_id >= $2
`
	rows, err := s.db.Query(ctx, statement, digests, getFirstTileID(ctx))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	digestToTraces := map[types.Digest][]schema.TraceID{}
	defer rows.Close()
	for rows.Next() {
		var digest types.Digest
		var traceID schema.TraceID
		if err := rows.Scan(&digest, &traceID); err != nil {
			return nil, skerr.Wrap(err)
		}
		digestToTraces[digest] = append(digestToTraces[digest], traceID)
	}
	return digestToTraces, nil
}

// getOptionsForTraces returns the most recent option map for each given trace.
func (s *Impl) getOptionsForTraces(ctx context.Context, digestToTraces map[types.Digest][]schema.TraceID) (map[schema.MD5Hash]paramtools.Params, error) {
	ctx, span := trace.StartSpan(ctx, "getOptionsForTraces")
	defer span.End()
	byTrace := map[schema.MD5Hash]paramtools.Params{}
	placeHolder := paramtools.Params{}
	var traceKey schema.MD5Hash
	for _, traces := range digestToTraces {
		for _, traceID := range traces {
			copy(traceKey[:], traceID)
			byTrace[traceKey] = placeHolder
		}
	}
	// we now have a set of the traces we need to lookup
	traceIDs := make([]schema.TraceID, 0, len(byTrace))
	for trID := range byTrace {
		traceIDs = append(traceIDs, sql.FromMD5Hash(trID))
	}
	const statement = `SELECT trace_id, options_id FROM ValuesAtHead
WHERE trace_id = ANY($1)`
	rows, err := s.db.Query(ctx, statement, traceIDs)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var traceID schema.TraceID
	var optionsID schema.OptionsID
	for rows.Next() {
		if err := rows.Scan(&traceID, &optionsID); err != nil {
			return nil, skerr.Wrap(err)
		}
		ps, err := s.expandOptionsToParams(ctx, optionsID)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		copy(traceKey[:], traceID)
		byTrace[traceKey] = ps
	}
	return byTrace, nil
}

// expandOptionsToParams returns the params that correspond to a given optionsID. The returned
// params should not be mutated, as it is not copied (for performance reasons).
func (s *Impl) expandOptionsToParams(ctx context.Context, optionsID schema.OptionsID) (paramtools.Params, error) {
	ctx, span := trace.StartSpan(ctx, "expandOptionsToParams")
	defer span.End()
	if keys, ok := s.optionsGroupingCache.Get(string(optionsID)); ok {
		return keys.(paramtools.Params), nil
	}
	// cache miss
	const statement = `SELECT keys FROM Options WHERE options_id = $1`
	row := s.db.QueryRow(ctx, statement, optionsID)
	var keys paramtools.Params
	if err := row.Scan(&keys); err != nil {
		return nil, skerr.Wrap(err)
	}
	s.optionsGroupingCache.Add(string(optionsID), keys)
	return keys, nil
}

// expandTracesIntoParamsets effectively returns a map detailing "who drew a given digest?". This
// is done by looking up the keys associated with each trace and combining them.
func (s *Impl) expandTracesIntoParamsets(ctx context.Context, toLookUp map[types.Digest][]schema.TraceID, traceToOptions map[schema.MD5Hash]paramtools.Params) (map[types.Digest]paramtools.ParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "expandTracesIntoParamsets")
	defer span.End()
	span.AddAttributes(trace.Int64Attribute("num_trace_sets", int64(len(toLookUp))))
	rv := map[types.Digest]paramtools.ParamSet{}
	for digest, traces := range toLookUp {
		paramset, err := s.lookupOrLoadParamSetFromCache(ctx, traces)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		var traceKey schema.MD5Hash
		for _, traceID := range traces {
			copy(traceKey[:], traceID)
			ps := traceToOptions[traceKey]
			paramset.AddParams(ps)
		}
		paramset.Normalize()
		rv[digest] = paramset
	}
	return rv, nil
}

// lookupOrLoadParamSetFromCache takes a slice of traces and returns a ParamSet combining all their
// keys. It will use the traceCache or query the DB and fill the cache on a cache miss.
func (s *Impl) lookupOrLoadParamSetFromCache(ctx context.Context, traces []schema.TraceID) (paramtools.ParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "lookupOrLoadParamSetFromCache")
	defer span.End()
	paramset := paramtools.ParamSet{}
	var cacheMisses []schema.TraceID
	for _, traceID := range traces {
		if keys, ok := s.traceCache.Get(string(traceID)); ok {
			paramset.AddParams(keys.(paramtools.Params))
		} else {
			cacheMisses = append(cacheMisses, traceID)
		}
	}
	if len(cacheMisses) > 0 {
		const statement = `SELECT trace_id, keys FROM Traces WHERE trace_id = ANY($1)`
		rows, err := s.db.Query(ctx, statement, cacheMisses)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		defer rows.Close()
		var traceID schema.TraceID
		for rows.Next() {
			var keys paramtools.Params
			if err := rows.Scan(&traceID, &keys); err != nil {
				return nil, skerr.Wrap(err)
			}
			s.traceCache.Add(string(traceID), keys)
			paramset.AddParams(keys)
		}
	}
	return paramset, nil
}

// expandTraceToParams will return a traces keys from the cache. On a cache miss, it will look
// up the trace from the DB and add it to the cache.
func (s *Impl) expandTraceToParams(ctx context.Context, traceID schema.TraceID) (paramtools.Params, error) {
	ctx, span := trace.StartSpan(ctx, "expandTraceToParams")
	defer span.End()
	if keys, ok := s.traceCache.Get(string(traceID)); ok {
		return keys.(paramtools.Params).Copy(), nil // Return a copy to protect cached value
	}
	// cache miss
	const statement = `SELECT keys FROM Traces WHERE trace_id = $1`
	row := s.db.QueryRow(ctx, statement, traceID)
	var keys paramtools.Params
	if err := row.Scan(&keys); err != nil {
		return nil, skerr.Wrap(err)
	}
	s.traceCache.Add(string(traceID), keys)
	return keys.Copy(), nil // Return a copy to protect cached value
}

// fillOutTraceHistory returns a slice of SearchResults that are mostly filled in, particularly
// including the history of the traces for each result.
func (s *Impl) fillOutTraceHistory(ctx context.Context, inputs []stageTwoResult) ([]*frontend.SearchResult, error) {
	ctx, span := trace.StartSpan(ctx, "fillOutTraceHistory")
	span.AddAttributes(trace.Int64Attribute("results", int64(len(inputs))))
	defer span.End()
	rv := make([]*frontend.SearchResult, len(inputs))
	for i, input := range inputs {
		sr := &frontend.SearchResult{
			Digest: types.Digest(hex.EncodeToString(input.leftDigest)),
			RefDiffs: map[common.RefClosest]*frontend.SRDiffDigest{
				common.PositiveRef: input.closestPositive,
				common.NegativeRef: input.closestNegative,
			},
		}
		if input.closestDigest != nil && input.closestDigest.Status == expectations.Positive {
			sr.ClosestRef = common.PositiveRef
		} else if input.closestDigest != nil && input.closestDigest.Status == expectations.Negative {
			sr.ClosestRef = common.NegativeRef
		}
		tg, err := s.traceGroupForTraces(ctx, input.traceIDs, sr.Digest)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if err := s.fillInExpectations(ctx, &tg, input.groupingID); err != nil {
			return nil, skerr.Wrap(err)
		}
		if err := s.fillInTraceParams(ctx, &tg); err != nil {
			return nil, skerr.Wrap(err)
		}
		sr.TraceGroup = tg
		if len(tg.Digests) > 0 {
			// The first digest in the trace group is this digest.
			sr.Status = tg.Digests[0].Status
		} else {
			// We assume untriaged if digest is not in the window.
			sr.Status = expectations.Untriaged
		}
		if len(tg.Traces) > 0 {
			// Grab the test name from the first trace, since we know all the traces are of
			// the same grouping, which includes test name.
			sr.Test = types.TestName(tg.Traces[0].Params[types.PrimaryKeyField])
		}
		leftPS := paramtools.ParamSet{}
		for _, tr := range tg.Traces {
			leftPS.AddParams(tr.Params)
		}
		leftPS.Normalize()
		sr.ParamSet = leftPS
		rv[i] = sr
	}
	return rv, nil
}

type traceDigestCommit struct {
	traceID   schema.TraceID
	commitID  schema.CommitID
	digest    types.Digest
	optionsID schema.OptionsID
}

// traceGroupForTraces gets all the history for a slice of traces within the given window and
// turns it into a format that the frontend can render.
func (s *Impl) traceGroupForTraces(ctx context.Context, traceIDs []schema.TraceID, primary types.Digest) (frontend.TraceGroup, error) {
	ctx, span := trace.StartSpan(ctx, "traceGroupForTraces")
	span.AddAttributes(trace.Int64Attribute("num_traces", int64(len(traceIDs))))
	defer span.End()
	const statement = `SELECT trace_id, commit_id, encode(digest, 'hex'), options_id
FROM TraceValues WHERE trace_id = ANY($1) AND commit_id >= $2
ORDER BY trace_id, commit_id`
	rows, err := s.db.Query(ctx, statement, traceIDs, getFirstCommitID(ctx))
	if err != nil {
		return frontend.TraceGroup{}, skerr.Wrap(err)
	}
	defer rows.Close()
	var dataPoints []traceDigestCommit
	for rows.Next() {
		var row traceDigestCommit
		if err := rows.Scan(&row.traceID, &row.commitID, &row.digest, &row.optionsID); err != nil {
			return frontend.TraceGroup{}, skerr.Wrap(err)
		}
		dataPoints = append(dataPoints, row)
	}
	return makeTraceGroup(ctx, dataPoints, primary)
}

// fillInExpectations looks up all the expectations for the digests included in the given
// TraceGroup and updates the passed in TraceGroup directly.
func (s *Impl) fillInExpectations(ctx context.Context, tg *frontend.TraceGroup, groupingID schema.GroupingID) error {
	ctx, span := trace.StartSpan(ctx, "fillInExpectations")
	defer span.End()
	arguments := make([]interface{}, 0, len(tg.Digests))
	for _, digestStatus := range tg.Digests {
		dBytes, err := sql.DigestToBytes(digestStatus.Digest)
		if err != nil {
			sklog.Warningf("invalid digest: %s", digestStatus.Digest)
			continue
		}
		arguments = append(arguments, dBytes)
	}
	const statement = `SELECT encode(digest, 'hex'), label FROM Expectations
WHERE grouping_id = $1 and digest = ANY($2)`
	rows, err := s.db.Query(ctx, statement, groupingID, arguments)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var digest types.Digest
		var label schema.ExpectationLabel
		if err := rows.Scan(&digest, &label); err != nil {
			return skerr.Wrap(err)
		}
		for i, ds := range tg.Digests {
			if ds.Digest == digest {
				tg.Digests[i].Status = label.ToExpectation()
			}
		}
	}
	return nil
}

// fillInTraceParams looks up the keys (params) for each trace and fills them in on the passed in
// TraceGroup.
func (s *Impl) fillInTraceParams(ctx context.Context, tg *frontend.TraceGroup) error {
	ctx, span := trace.StartSpan(ctx, "fillInTraceParams")
	defer span.End()
	for i, tr := range tg.Traces {
		traceID, err := hex.DecodeString(string(tr.ID))
		if err != nil {
			return skerr.Wrapf(err, "invalid trace id %q", tr.ID)
		}
		ps, err := s.expandTraceToParams(ctx, traceID)
		if err != nil {
			return skerr.Wrap(err)
		}
		if tr.RawTrace.OptionsID != nil {
			opts, err := s.expandOptionsToParams(ctx, tr.RawTrace.OptionsID)
			if err != nil {
				return skerr.Wrap(err)
			}
			ps.Add(opts)
		}
		tg.Traces[i].Params = ps
		tg.Traces[i].RawTrace = nil // Done with this temporary data.
	}
	return nil
}

// convertBulkTriageData converts the passed in map into the version usable by the frontend.
func (s *Impl) convertBulkTriageData(ctx context.Context, data map[groupingDigestKey]expectations.Label) (web_frontend.TriageRequestData, error) {
	ctx, span := trace.StartSpan(ctx, "convertBulkTriageData")
	defer span.End()
	rv := map[types.TestName]map[types.Digest]expectations.Label{}
	for key, label := range data {
		var groupingKeys paramtools.Params
		if gk, ok := s.optionsGroupingCache.Get(key.groupingID); ok {
			groupingKeys = gk.(paramtools.Params)
		} else {
			const statement = `SELECT keys FROM Groupings WHERE grouping_id = $1`
			row := s.db.QueryRow(ctx, statement, key.groupingID[:])
			if err := row.Scan(&groupingKeys); err != nil {
				return nil, skerr.Wrap(err)
			}
			s.optionsGroupingCache.Add(key.groupingID, groupingKeys)
		}
		testName := types.TestName(groupingKeys[types.PrimaryKeyField])
		digest := types.Digest(hex.EncodeToString(key.digest[:]))
		if byTest, ok := rv[testName]; ok {
			byTest[digest] = label
		} else {
			rv[testName] = map[types.Digest]expectations.Label{digest: label}
		}
	}
	return rv, nil
}

// getCommits returns the front-end friendly version of the commits within the searched window.
func (s *Impl) getCommits(ctx context.Context) ([]web_frontend.Commit, error) {
	ctx, span := trace.StartSpan(ctx, "getCommits")
	defer span.End()
	rv := make([]web_frontend.Commit, getActualWindowLength(ctx))
	commitIDs := getCommitToIdxMap(ctx)
	for commitID, idx := range commitIDs {
		var commit web_frontend.Commit
		if c, ok := s.commitCache.Get(commitID); ok {
			commit = c.(web_frontend.Commit)
		} else {
			// TODO(kjlubick) will need to handle non-git repos too
			const statement = `SELECT git_hash, commit_time, author_email, subject
FROM GitCommits WHERE commit_id = $1`
			row := s.db.QueryRow(ctx, statement, commitID)
			var dbRow schema.GitCommitRow
			if err := row.Scan(&dbRow.GitHash, &dbRow.CommitTime, &dbRow.AuthorEmail, &dbRow.Subject); err != nil {
				return nil, skerr.Wrap(err)
			}
			commit = web_frontend.Commit{
				CommitTime: dbRow.CommitTime.UTC().Unix(),
				Hash:       dbRow.GitHash,
				Author:     dbRow.AuthorEmail,
				Subject:    dbRow.Subject,
			}
			s.commitCache.Add(commitID, commit)
		}
		rv[idx] = commit
	}
	return rv, nil
}

// makeTraceGroup converts all the trace+digest+commit triples into a TraceGroup. On the frontend,
// we only show the top 9 digests before fading them to grey - this handles that logic.
func makeTraceGroup(ctx context.Context, data []traceDigestCommit, primary types.Digest) (frontend.TraceGroup, error) {
	ctx, span := trace.StartSpan(ctx, "makeTraceGroup")
	defer span.End()
	tg := frontend.TraceGroup{}
	if len(data) == 0 {
		return tg, nil
	}
	indexMap := getCommitToIdxMap(ctx)
	currentTrace := frontend.Trace{
		ID:            tiling.TraceID(hex.EncodeToString(data[0].traceID)),
		DigestIndices: emptyIndices(getActualWindowLength(ctx)),
		RawTrace:      tiling.NewEmptyTrace(getActualWindowLength(ctx), nil, nil),
	}
	tg.Traces = append(tg.Traces, currentTrace)
	for _, dp := range data {
		tID := tiling.TraceID(hex.EncodeToString(dp.traceID))
		if currentTrace.ID != tID {
			currentTrace = frontend.Trace{
				ID:            tID,
				DigestIndices: emptyIndices(getActualWindowLength(ctx)),
				RawTrace:      tiling.NewEmptyTrace(getActualWindowLength(ctx), nil, nil),
			}
			tg.Traces = append(tg.Traces, currentTrace)
		}
		idx, ok := indexMap[dp.commitID]
		if !ok {
			continue
		}
		currentTrace.RawTrace.Digests[idx] = dp.digest
		// We want to report the latest options, so always update this
		currentTrace.RawTrace.OptionsID = dp.optionsID
	}

	// Find the most recent / important digests and assign them an index. Everything else will
	// be given the sentinel value.
	digestIndices, totalDigests := search.ComputeDigestIndices(&tg, primary)
	tg.TotalDigests = totalDigests

	tg.Digests = make([]frontend.DigestStatus, len(digestIndices))
	for digest, idx := range digestIndices {
		tg.Digests[idx] = frontend.DigestStatus{
			Digest: digest,
		}
	}

	for _, tr := range tg.Traces {
		for j, digest := range tr.RawTrace.Digests {
			if digest == tiling.MissingDigest {
				continue // There is already the missing index there.
			}
			idx, ok := digestIndices[digest]
			if ok {
				tr.DigestIndices[j] = idx
			} else {
				// Fold everything else into the last digest index (grey on the frontend).
				tr.DigestIndices[j] = search.MaxDistinctDigestsToPresent - 1
			}
		}
	}
	return tg, nil
}

// emptyIndices returns an array of the given length with placeholder values for "missing data".
func emptyIndices(length int) []int {
	rv := make([]int, length)
	for i := range rv {
		rv[i] = search.MissingDigestIndex
	}
	return rv
}

// Make sure Impl implements the API interface.
var _ API = (*Impl)(nil)
