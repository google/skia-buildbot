// Package search2 encapsulates various queries we make against Gold's data. It is backed
// by the SQL database and aims to replace the current search package.
package search

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	ttlcache "github.com/patrickmn/go-cache"
	"go.opencensus.io/trace"
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/publicparams"
	"go.skia.org/infra/golden/go/search/caching"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/search/providers"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/web/frontend"
)

type API interface {
	// NewAndUntriagedSummaryForCL returns a summarized look at the new digests produced by a CL
	// (that is, digests not currently on the primary branch for this grouping at all) as well as
	// how many of the newly produced digests are currently untriaged.
	NewAndUntriagedSummaryForCL(ctx context.Context, qCLID string) (NewAndUntriagedSummary, error)

	// ChangelistLastUpdated returns the timestamp that the given CL was updated. It returns an
	// error if the CL does not exist.
	ChangelistLastUpdated(ctx context.Context, qCLID string) (time.Time, error)

	// Search queries the current tile based on the parameters specified in
	// the instance of the *query.Search.
	Search(context.Context, *query.Search) (*frontend.SearchResponse, error)

	// GetPrimaryBranchParamset returns all params that are on the most recent few tiles. If
	// this is public view, it will only return the params on the traces which match the publicly
	// visible rules.
	GetPrimaryBranchParamset(ctx context.Context) (paramtools.ReadOnlyParamSet, error)

	// GetChangelistParamset returns all params that were produced by the given CL. If
	// this is public view, it will only return the params on the traces which match the publicly
	// visible rules.
	GetChangelistParamset(ctx context.Context, crs, clID string) (paramtools.ReadOnlyParamSet, error)

	// GetBlamesForUntriagedDigests finds all untriaged digests at head and then tries to determine
	// which commits first introduced those untriaged digests. It returns a list of commits or
	// commit ranges that are believed to have caused those untriaged digests.
	GetBlamesForUntriagedDigests(ctx context.Context, corpus string) (BlameSummaryV1, error)

	// GetCluster returns all digests from the traces matching the various filters compared to
	// all other digests in that set, so they can be drawn as a 2d cluster. This helps visualize
	// patterns in the images, which can identify errors in triaging, among other things.
	GetCluster(ctx context.Context, opts ClusterOptions) (frontend.ClusterDiffResult, error)

	// GetCommitsInWindow returns the commits in the configured window.
	GetCommitsInWindow(ctx context.Context) ([]frontend.Commit, error)

	// GetDigestsForGrouping returns all digests that were produced in a given grouping in the most
	// recent window of data.
	GetDigestsForGrouping(ctx context.Context, grouping paramtools.Params) (frontend.DigestListResponse, error)

	// GetDigestDetails returns information about the given digest as produced on the given
	// grouping. If the CL and CRS are provided, it will include information specific to that CL.
	GetDigestDetails(ctx context.Context, grouping paramtools.Params, digest types.Digest, clID, crs string) (frontend.DigestDetails, error)

	// GetDigestsDiff returns comparison and triage information about the left and right digest.
	GetDigestsDiff(ctx context.Context, grouping paramtools.Params, left, right types.Digest, clID, crs string) (frontend.DigestComparison, error)

	// CountDigestsByTest summarizes the counts of digests according to some limited filtering
	// and breaks it down by test.
	CountDigestsByTest(ctx context.Context, q frontend.ListTestsQuery) (frontend.ListTestsResponse, error)

	// ComputeGUIStatus looks at all visible traces at head and returns a summary of how many are
	// untriaged for each corpus, as well as the most recent commit for which we have data.
	ComputeGUIStatus(ctx context.Context) (frontend.GUIStatus, error)
}

// NewAndUntriagedSummary is a summary of the results associated with a given CL. It focuses on
// the untriaged and new images produced.
type NewAndUntriagedSummary struct {
	// ChangelistID is the nonqualified id of the CL.
	ChangelistID string
	// PatchsetSummaries is a summary for all Patchsets for which we have data.
	PatchsetSummaries []providers.PatchsetNewAndUntriagedSummary
	// LastUpdated returns the timestamp of the CL, which corresponds to the last datapoint for
	// this CL.
	LastUpdated time.Time
	// Outdated is set to true if the value that was previously cached was out of date and is
	// currently being recalculated. We do this to return something quickly to the user (even if
	// something like the
	Outdated bool
}

type BlameSummaryV1 struct {
	Ranges []BlameEntry
}

// BlameEntry represents a commit or range of commits that is responsible for some amount of
// untriaged digests. It allows us to identify potentially problematic commits and coordinate with
// the authors as necessary.
type BlameEntry struct {
	// CommitRange is either a single commit id or two commit ids separated by a colon indicating
	// a range. This string can be used as the "blame id" in the search.
	CommitRange string
	// TotalUntriagedDigests is the number of digests that are believed to be first untriaged
	// in this commit range.
	TotalUntriagedDigests int
	// AffectedGroupings summarize the untriaged digests affected in the commit range.
	AffectedGroupings []*AffectedGrouping
	// Commits is one or two commits corresponding to the CommitRange.
	Commits []frontend.Commit
}

type AffectedGrouping struct {
	Grouping         paramtools.Params
	UntriagedDigests int
	SampleDigest     types.Digest

	// groupingID is used as an intermediate step in combineIntoRanges, and to search by blame ID.
	groupingID schema.MD5Hash

	// traceIDsAndDigests is used to search by blame ID.
	traceIDsAndDigests []traceIDAndDigest
}

type traceIDAndDigest struct {
	id     schema.TraceID
	digest schema.DigestBytes
}

type ClusterOptions struct {
	Grouping                paramtools.Params
	Filters                 paramtools.ParamSet
	IncludePositiveDigests  bool
	IncludeNegativeDigests  bool
	IncludeUntriagedDigests bool
	CodeReviewSystem        string
	ChangelistID            string
	PatchsetID              string
}

const (
	commitCacheSize          = 5_000
	optionsGroupingCacheSize = 50_000
	traceCacheSize           = 1_000_000
)

type Impl struct {
	db           *pgxpool.Pool
	windowLength int
	// Lets us create links from CL data to the Code Review System that produced it.
	reviewSystemMapping map[string]string

	// mutex protects the caches, e.g. digestsOnPrimary and publiclyVisibleTraces
	mutex sync.RWMutex
	// This caches the digests seen per grouping on the primary branch.
	digestsOnPrimary map[common.GroupingDigestKey]struct{}
	// This caches the trace ids that are publicly visible.
	publiclyVisibleTraces map[schema.MD5Hash]struct{}
	// This caches the corpora names that are publicly visible.
	publiclyVisibleCorpora map[string]struct{}
	isPublicView           bool

	optionsGroupingCache *lru.Cache
	traceCache           *lru.Cache
	paramsetCache        *ttlcache.Cache

	dbType config.DatabaseType

	statusProvider           *providers.StatusProvider
	changeDataProvider       *providers.ChangelistProvider
	materializedViewProvider *providers.MaterializedViewProvider
	commitsProvider          *providers.CommitsProvider
	traceDigestsProvider     *providers.TraceDigestsProvider
	cacheManager             *caching.SearchCacheManager
}

// New returns an implementation of API.
func New(sqlDB *pgxpool.Pool, windowLength int, cacheClient cache.Cache, cache_corpora []string) *Impl {

	gc, err := lru.New(optionsGroupingCacheSize)
	if err != nil {
		panic(err) // should only happen if optionsGroupingCacheSize is negative.
	}
	tc, err := lru.New(traceCacheSize)
	if err != nil {
		panic(err) // should only happen if traceCacheSize is negative.
	}
	pc := ttlcache.New(time.Minute, 10*time.Minute)
	materializedViewProvider := providers.NewMaterializedViewProvider(sqlDB, windowLength)
	cacheManager := caching.New(cacheClient, sqlDB, cache_corpora, windowLength)
	return &Impl{
		db:                       sqlDB,
		windowLength:             windowLength,
		digestsOnPrimary:         map[common.GroupingDigestKey]struct{}{},
		optionsGroupingCache:     gc,
		traceCache:               tc,
		paramsetCache:            pc,
		reviewSystemMapping:      map[string]string{},
		statusProvider:           providers.NewStatusProvider(sqlDB, windowLength),
		changeDataProvider:       providers.NewChangelistProvider(sqlDB),
		materializedViewProvider: materializedViewProvider,
		commitsProvider:          providers.NewCommitsProvider(sqlDB, cacheClient, windowLength),
		traceDigestsProvider:     providers.NewTraceDigestsProvider(sqlDB, windowLength, materializedViewProvider, cacheManager),
		cacheManager:             cacheManager,
	}
}

// SetDatabaseType sets the database type for the current configuration.
func (s *Impl) SetDatabaseType(dbType config.DatabaseType) {
	s.dbType = dbType
	s.traceDigestsProvider.SetDatabaseType(dbType)
}

// SetReviewSystemTemplates sets the URL templates that are used to link to the code review system.
// The Changelist ID will be substituted in using fmt.Sprintf and a %s placeholder.
func (s *Impl) SetReviewSystemTemplates(m map[string]string) {
	s.reviewSystemMapping = m
}

// StartCacheProcess loads the caches used for searching and starts a goroutine to keep those
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
	s.changeDataProvider.SetDigestsOnPrimary(onPrimary)
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
ORDER BY commit_id DESC
LIMIT 1 OFFSET $1`, commitsWithDataToSearch)
	var tileID int
	if s.dbType == config.Spanner {
		var lc pgtype.Int8
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
		tileID = int(lc.Int)
	} else {
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
		tileID = int(lc.Int)
	}
	return schema.TileID(tileID), nil
}

// getDigestsOnPrimary returns a map of all distinct digests on the primary branch.
func (s *Impl) getDigestsOnPrimary(ctx context.Context, tile schema.TileID) (map[common.GroupingDigestKey]struct{}, error) {
	ctx, span := trace.StartSpan(ctx, "getDigestsOnPrimary")
	defer span.End()
	rows, err := s.db.Query(ctx, `
SELECT DISTINCT grouping_id, digest FROM
TiledTraceDigests WHERE tile_id >= $1`, tile)
	if err != nil {
		if err == pgx.ErrNoRows {
			return map[common.GroupingDigestKey]struct{}{}, nil
		}
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	rv := map[common.GroupingDigestKey]struct{}{}
	var digest schema.DigestBytes
	var grouping schema.GroupingID
	var key common.GroupingDigestKey
	keyGrouping := key.GroupingID[:]
	keyDigest := key.Digest[:]
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

func (s *Impl) StartMaterializedViews(ctx context.Context, corpora []string, updateInterval time.Duration) error {
	return s.materializedViewProvider.StartMaterializedViews(ctx, corpora, updateInterval)
}

// StartApplyingPublicParams loads the cached set of traces which are publicly visible and then
// starts a goroutine to update this cache as per the provided interval.
func (s *Impl) StartApplyingPublicParams(ctx context.Context, matcher publicparams.Matcher, interval time.Duration) error {
	_, span := trace.StartSpan(ctx, "StartApplyingPublicParams", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	s.isPublicView = true

	cycle := func(ctx context.Context) error {
		rows, err := s.db.Query(ctx, `SELECT trace_id, keys FROM Traces AS OF SYSTEM TIME '-0.1s'`)
		if err != nil {
			return skerr.Wrap(err)
		}
		publiclyVisibleCorpora := map[string]struct{}{}
		publiclyVisibleTraces := map[schema.MD5Hash]struct{}{}
		var yes struct{}
		var traceKey schema.MD5Hash
		defer rows.Close()
		for rows.Next() {
			var traceID schema.TraceID
			var keys paramtools.Params
			if err := rows.Scan(&traceID, &keys); err != nil {
				return skerr.Wrap(err)
			}
			if matcher.Matches(keys) {
				copy(traceKey[:], traceID)
				publiclyVisibleTraces[traceKey] = yes
				publiclyVisibleCorpora[keys[types.CorpusField]] = yes
			}
		}
		s.mutex.Lock()
		defer s.mutex.Unlock()
		s.publiclyVisibleTraces = publiclyVisibleTraces
		s.statusProvider.SetPublicTraces(publiclyVisibleTraces)
		s.traceDigestsProvider.SetPublicTraces(publiclyVisibleTraces)

		s.publiclyVisibleCorpora = publiclyVisibleCorpora
		s.statusProvider.SetPublicCorpora(publiclyVisibleCorpora)
		return nil
	}
	if err := cycle(ctx); err != nil {
		return skerr.Wrapf(err, "initializing cache of visible traces")
	}
	sklog.Infof("Successfully initialized visible trace cache.")

	go util.RepeatCtx(ctx, interval, func(ctx context.Context) {
		err := cycle(ctx)
		if err != nil {
			sklog.Warningf("Could not update map of public traces: %s", err)
		}
	})
	return nil
}

// NewAndUntriagedSummaryForCL queries all the patchsets in parallel (to keep the query less
// complex). If there are no patchsets for the provided CL, it returns an error.
func (s *Impl) NewAndUntriagedSummaryForCL(ctx context.Context, qCLID string) (NewAndUntriagedSummary, error) {
	ctx, span := trace.StartSpan(ctx, "search2_NewAndUntriagedSummaryForCL")
	defer span.End()

	patchSummaries, err := s.changeDataProvider.GetNewAndUntriagedSummaryForCL(ctx, qCLID)
	if err != nil {
		return NewAndUntriagedSummary{}, skerr.Wrapf(err, "Getting Patchset summaries for CL %q", qCLID)
	}
	var updatedTS time.Time
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		updatedTS, err = s.ChangelistLastUpdated(ctx, qCLID)
		return skerr.Wrap(err)
	})
	if err := eg.Wait(); err != nil {
		return NewAndUntriagedSummary{}, skerr.Wrapf(err, "Getting counts for CL %q", qCLID)
	}
	return NewAndUntriagedSummary{
		ChangelistID:      sql.Unqualify(qCLID),
		PatchsetSummaries: patchSummaries,
		LastUpdated:       updatedTS.UTC(),
	}, nil
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
		if err == pgx.ErrNoRows {
			return time.Time{}, nil
		}
		return time.Time{}, skerr.Wrapf(err, "Getting last updated ts for cl %q", qCLID)
	}
	return updatedTS.UTC(), nil
}

// Search implements the SearchAPI interface.
func (s *Impl) Search(ctx context.Context, q *query.Search) (*frontend.SearchResponse, error) {
	ctx, span := trace.StartSpan(ctx, "search2_Search")
	defer span.End()

	sklog.Infof("Searching with query: %v", q)
	ctx = context.WithValue(ctx, common.QueryKey, *q)
	ctx, err := s.addCommitsData(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if q.ChangelistID != "" {
		if q.CodeReviewSystemID == "" {
			return nil, skerr.Fmt("Code Review System (crs) must be specified")
		}
		return s.searchCLData(ctx)
	}

	commits, err := s.commitsProvider.GetCommits(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Find all digests and traces that match the given search criteria.
	// This will be filtered according to the publiclyAllowedParams as well.
	var traceDigests []common.DigestWithTraceAndGrouping
	if q.BlameGroupID != "" {
		corpus := q.TraceValues[types.CorpusField][0]
		if corpus == "" {
			return nil, skerr.Fmt("must specify corpus in search of left side.")
		}
		traceDigests, err = s.getTracesForBlame(ctx, corpus, q.BlameGroupID)
		if err != nil {
			return nil, skerr.Wrapf(err, "searching for blame %q in corpus %q", q.BlameGroupID, corpus)
		}
	} else {
		traceDigests, err = s.traceDigestsProvider.GetMatchingDigestsAndTraces(ctx, q.IncludeUntriagedDigests, q.IncludeNegativeDigests, q.IncludePositiveDigests)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	if len(traceDigests) == 0 {
		return &frontend.SearchResponse{
			Commits: commits,
		}, nil
	}
	// Lookup the closest diffs to the given digests. This returns a subset according to the
	// limit and offset in the query.
	closestDiffs, extendedBulkTriageDeltaInfos, err := s.getClosestDiffs(ctx, traceDigests)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Go fetch history and paramset (within this grouping, and respecting publiclyAllowedParams).
	paramsetsByDigest, err := s.getParamsetsForRightSide(ctx, closestDiffs)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Flesh out the trace history with enough data to draw the dots diagram on the frontend.
	results, err := s.fillOutTraceHistory(ctx, closestDiffs)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Populate the LabelBefore fields of the extendedBulkTriageDeltaInfos with expectations from
	// the primary branch.
	if err := s.populateLabelBefore(ctx, extendedBulkTriageDeltaInfos); err != nil {
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

	// Populate the optionsIDs fields of each extendedBulkTriageDeltaInfo.
	if err := s.populateExtendedBulkTriageDeltaInfosOptionsIDs(ctx, extendedBulkTriageDeltaInfos); err != nil {
		return nil, skerr.Wrap(err)
	}

	bulkTriageDeltaInfos, err := s.prepareExtendedBulkTriageDeltaInfosForFrontend(ctx, extendedBulkTriageDeltaInfos)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &frontend.SearchResponse{
		Results:              results,
		Offset:               q.Offset,
		Size:                 len(extendedBulkTriageDeltaInfos),
		BulkTriageDeltaInfos: bulkTriageDeltaInfos,
		Commits:              commits,
	}, nil
}

// addCommitsData finds the current sliding window of data (The last N commits) and adds the
// derived data to the given context and returns it.
func (s *Impl) addCommitsData(ctx context.Context) (context.Context, error) {
	return common.AddCommitsData(ctx, s.db, s.windowLength)
}

type digestWithTraceAndGrouping struct {
	traceID    schema.TraceID
	groupingID schema.GroupingID
	digest     schema.DigestBytes
	// optionsID will be set for CL data only; for primary data we have to look it up from a
	// different table and the options could change over time.
	optionsID schema.OptionsID
}

// getTracesForBlame returns the traces that match the given blameID. It mirrors the behavior of
// method GetBlamesForUntriagedDigests. See function combineIntoRanges for details.
func (s *Impl) getTracesForBlame(ctx context.Context, corpus string, blameID string) ([]common.DigestWithTraceAndGrouping, error) {
	ctx, span := trace.StartSpan(ctx, "getTracesForBlame")
	defer span.End()
	// Find untriaged digests at head and the traces that produced them.
	tracesByDigest, err := s.traceDigestsProvider.GetTracesWithUntriagedDigestsAtHead(ctx, corpus)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if s.isPublicView {
		tracesByDigest = s.applyPublicFilter(ctx, tracesByDigest)
	}
	var traces []schema.TraceID
	for _, xt := range tracesByDigest {
		traces = append(traces, xt...)
	}
	if len(traces) == 0 {
		return nil, nil // No data, we can stop here
	}
	ctx, err = s.addCommitsData(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Return the trace histories for those traces, as well as a mapping of the unique
	// digest+grouping pairs in order to get expectations.
	histories, _, err := s.getHistoriesForTraces(ctx, tracesByDigest)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Expand grouping_ids into full params.
	groupings, err := s.expandGroupings(ctx, tracesByDigest)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	commits, err := s.commitsProvider.GetCommits(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Look at trace histories and identify ranges of commits that caused us to go from drawing
	// triaged digests to untriaged digests.
	ranges := combineIntoRanges(ctx, histories, groupings, commits)

	var rv []common.DigestWithTraceAndGrouping
	for _, r := range ranges {
		if r.CommitRange == blameID {
			for _, ag := range r.AffectedGroupings {
				for _, traceIDAndDigest := range ag.traceIDsAndDigests {
					rv = append(rv, common.DigestWithTraceAndGrouping{
						TraceID:    traceIDAndDigest.id,
						GroupingID: ag.groupingID[:],
						Digest:     traceIDAndDigest.digest,
					})
				}
			}
		}
	}
	return rv, nil
}

type digestAndClosestDiffs struct {
	leftDigest      schema.DigestBytes
	groupingID      schema.GroupingID
	rightDigests    []schema.DigestBytes
	traceIDs        []schema.TraceID
	optionsIDs      []schema.OptionsID     // will be set for CL data only
	closestDigest   *frontend.SRDiffDigest // These won't have ParamSets yet
	closestPositive *frontend.SRDiffDigest
	closestNegative *frontend.SRDiffDigest
}

// extendedBulkTriageDeltaInfo extends the frontend.BulkTriageDeltaInfo struct with the information
// needed to populate the LabelBefore field in a separate SQL query.
type extendedBulkTriageDeltaInfo struct {
	frontend.BulkTriageDeltaInfo

	traceIDs   []schema.TraceID
	groupingID schema.GroupingID
	digest     schema.DigestBytes
	optionsIDs []schema.OptionsID // Will be set for CL data only.
}

// getClosestDiffs returns information about the closest triaged digests for each result in the
// input. We are able to batch the queries by grouping and do so for better performance.
// While this returns a subset of data as defined by the query, it also returns sufficient
// information to bulk-triage all of the inputs. Note that this function does not populate the
// LabelBefore fields of the returned extendedBulkTriageDeltaInfo structs; these need to be
// populated by the caller.
func (s *Impl) getClosestDiffs(ctx context.Context, inputs []common.DigestWithTraceAndGrouping) ([]digestAndClosestDiffs, []extendedBulkTriageDeltaInfo, error) {
	ctx, span := trace.StartSpan(ctx, "getClosestDiffs")
	defer span.End()
	byGrouping := map[schema.MD5Hash][]common.DigestWithTraceAndGrouping{}
	// Even if two groupings draw the same digest, we want those as two different results because
	// they could be triaged differently.
	byDigestAndGrouping := map[common.GroupingDigestKey]digestAndClosestDiffs{}
	var mutex sync.Mutex
	for _, input := range inputs {
		gID := sql.AsMD5Hash(input.GroupingID)
		byGrouping[gID] = append(byGrouping[gID], input)
		key := common.GroupingDigestKey{
			Digest:     sql.AsMD5Hash(input.Digest),
			GroupingID: sql.AsMD5Hash(input.GroupingID),
		}
		bd := byDigestAndGrouping[key]
		bd.leftDigest = input.Digest
		bd.groupingID = input.GroupingID
		bd.traceIDs = append(bd.traceIDs, input.TraceID)
		if input.OptionsID != nil {
			bd.optionsIDs = append(bd.optionsIDs, input.OptionsID)
		}
		byDigestAndGrouping[key] = bd
	}

	// Look up the diffs in parallel by grouping, as we only want to compare the images to other
	// images produced by traces in the same grouping.
	eg, eCtx := errgroup.WithContext(ctx)
	for g, i := range byGrouping {
		groupingID, inputs := g, i
		eg.Go(func() error {
			// Aggregate and deduplicate digests from the stage one results.
			digests := make([]schema.DigestBytes, 0, len(inputs))
			duplicates := map[schema.MD5Hash]bool{}
			var key schema.MD5Hash
			for _, input := range inputs {
				copy(key[:], input.Digest)
				if duplicates[key] {
					continue
				}
				duplicates[key] = true
				digests = append(digests, input.Digest)
			}
			resultsByDigest, err := s.getDiffsForGrouping(eCtx, groupingID, digests)
			if err != nil {
				return skerr.Wrap(err)
			}
			// Combine those results into our search results.
			mutex.Lock()
			defer mutex.Unlock()
			for key, diffs := range resultsByDigest {
				// combine this map with the other
				digestAndClosestDiffs := byDigestAndGrouping[key]
				for _, srdd := range diffs {
					digestBytes, err := sql.DigestToBytes(srdd.Digest)
					if err != nil {
						return skerr.Wrap(err)
					}
					digestAndClosestDiffs.rightDigests = append(digestAndClosestDiffs.rightDigests, digestBytes)
					if srdd.Status == expectations.Positive {
						digestAndClosestDiffs.closestPositive = srdd
					} else {
						digestAndClosestDiffs.closestNegative = srdd
					}
					if digestAndClosestDiffs.closestNegative != nil && digestAndClosestDiffs.closestPositive != nil {
						if digestAndClosestDiffs.closestPositive.CombinedMetric < digestAndClosestDiffs.closestNegative.CombinedMetric {
							digestAndClosestDiffs.closestDigest = digestAndClosestDiffs.closestPositive
						} else {
							digestAndClosestDiffs.closestDigest = digestAndClosestDiffs.closestNegative
						}
					} else {
						// there is only one type of diff, so it defaults to the closest.
						digestAndClosestDiffs.closestDigest = srdd
					}
				}
				byDigestAndGrouping[key] = digestAndClosestDiffs
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	q := common.GetQuery(ctx)
	var extendedBulkTriageDeltaInfos []extendedBulkTriageDeltaInfo
	results := make([]digestAndClosestDiffs, 0, len(byDigestAndGrouping))
	for _, s2 := range byDigestAndGrouping {
		// Filter out any results without a closest triaged digest (if that option is selected).
		if q.MustIncludeReferenceFilter && s2.closestDigest == nil {
			continue
		}
		grouping, err := s.expandGrouping(ctx, sql.AsMD5Hash(s2.groupingID))
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		triageDeltaInfo := extendedBulkTriageDeltaInfo{
			// We do not populate the LabelBefore field, as that is the caller's responsibility.
			// However, we will populate the ClosestDiffLabel field if there is a closest digest.
			BulkTriageDeltaInfo: frontend.BulkTriageDeltaInfo{
				Grouping: grouping,
				Digest:   types.Digest(hex.EncodeToString(s2.leftDigest)),
			},
			traceIDs:   s2.traceIDs,
			groupingID: s2.groupingID,
			digest:     s2.leftDigest,
			optionsIDs: s2.optionsIDs,
		}
		if s2.closestDigest != nil {
			// Apply RGBA Filter here - if the closest digest isn't within range, we remove it.
			maxDiff := util.MaxInt(s2.closestDigest.MaxRGBADiffs[:]...)
			if maxDiff < q.RGBAMinFilter || maxDiff > q.RGBAMaxFilter {
				continue
			}
			closestLabel := s2.closestDigest.Status
			triageDeltaInfo.ClosestDiffLabel = frontend.ClosestDiffLabel(closestLabel)
		} else {
			triageDeltaInfo.ClosestDiffLabel = frontend.ClosestDiffLabelNone
		}
		results = append(results, s2)
		extendedBulkTriageDeltaInfos = append(extendedBulkTriageDeltaInfos, triageDeltaInfo)
	}
	// Sort for determinism.
	sort.Slice(extendedBulkTriageDeltaInfos, func(i, j int) bool {
		groupIDComparison := bytes.Compare(extendedBulkTriageDeltaInfos[i].groupingID, extendedBulkTriageDeltaInfos[j].groupingID)
		return groupIDComparison < 0 || (groupIDComparison == 0 && extendedBulkTriageDeltaInfos[i].Digest < extendedBulkTriageDeltaInfos[j].Digest)
	})
	if q.Offset >= len(results) {
		return nil, extendedBulkTriageDeltaInfos, nil
	}
	sortAsc := q.Sort == query.SortAscending
	sort.Slice(results, func(i, j int) bool {
		if results[i].closestDigest == nil && results[j].closestDigest != nil {
			return true // sort results with no reference image to the top
		}
		if results[i].closestDigest != nil && results[j].closestDigest == nil {
			return false
		}
		if (results[i].closestDigest == nil && results[j].closestDigest == nil) ||
			results[i].closestDigest.CombinedMetric == results[j].closestDigest.CombinedMetric {
			// Tiebreak using digest in ascending order, followed by groupingID.
			c := bytes.Compare(results[i].leftDigest, results[j].leftDigest)
			if c != 0 {
				return c < 0
			}
			return bytes.Compare(results[i].groupingID, results[j].groupingID) < 0
		}
		if sortAsc {
			return results[i].closestDigest.CombinedMetric < results[j].closestDigest.CombinedMetric
		}
		return results[i].closestDigest.CombinedMetric > results[j].closestDigest.CombinedMetric
	})

	if q.Limit <= 0 {
		for i := range extendedBulkTriageDeltaInfos {
			extendedBulkTriageDeltaInfos[i].InCurrentSearchResultsPage = true
		}
		return results, extendedBulkTriageDeltaInfos, nil
	}
	end := util.MinInt(len(results), q.Offset+q.Limit)
	for i := q.Offset; i < end; i++ {
		extendedBulkTriageDeltaInfos[i].InCurrentSearchResultsPage = true
	}
	return results[q.Offset:end], extendedBulkTriageDeltaInfos, nil
}

// getDiffsForGrouping returns the closest positive and negative diffs for the provided digests
// in the given grouping.
func (s *Impl) getDiffsForGrouping(ctx context.Context, groupingID schema.MD5Hash, leftDigests []schema.DigestBytes) (map[common.GroupingDigestKey][]*frontend.SRDiffDigest, error) {
	ctx, span := trace.StartSpan(ctx, "getDiffsForGrouping")
	defer span.End()
	rtv := common.GetQuery(ctx).RightTraceValues
	digestsInGrouping, err := s.getDigestsForGrouping(ctx, groupingID[:], rtv)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	statement := `
WITH
PositiveOrNegativeDigests AS (
	SELECT digest, label FROM Expectations
	WHERE grouping_id = $1 AND (label = 'n' OR label = 'p')
),
ComparisonBetweenUntriagedAndObserved AS (
	SELECT DiffMetrics.* FROM DiffMetrics
	WHERE left_digest = ANY($2) AND right_digest = ANY($3)
)
`
	if s.dbType == config.Spanner {
		statement += `
		-- This will return the right_digest with the smallest combined_metric for each left_digest + label
		SELECT label, left_digest, right_digest, num_pixels_diff, percent_pixels_diff, max_rgba_diffs,
  combined_metric, dimensions_differ
FROM
  ComparisonBetweenUntriagedAndObserved
JOIN PositiveOrNegativeDigests
  ON ComparisonBetweenUntriagedAndObserved.right_digest = PositiveOrNegativeDigests.digest
ORDER BY left_digest, label, combined_metric ASC, max_channel_diff ASC, right_digest ASC
		`
	} else {
		statement += `
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
	}

	rows, err := s.db.Query(ctx, statement, groupingID[:], leftDigests, digestsInGrouping)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	results := map[common.GroupingDigestKey][]*frontend.SRDiffDigest{}
	var label schema.ExpectationLabel
	var row schema.DiffMetricRow
	for rows.Next() {
		if err := rows.Scan(&label, &row.LeftDigest, &row.RightDigest, &row.NumPixelsDiff,
			&row.PercentPixelsDiff, &row.MaxRGBADiffs, &row.CombinedMetric,
			&row.DimensionsDiffer); err != nil {
			rows.Close()
			return nil, skerr.Wrap(err)
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
		key := common.GroupingDigestKey{
			Digest:     sql.AsMD5Hash(row.LeftDigest),
			GroupingID: groupingID,
		}
		results[key] = append(results[key], srdd)
	}
	return results, nil
}

// getDigestsForGrouping returns the digests that were produced in the given range by any traces
// which belong to the grouping and match the provided paramset (if provided). It returns digests
// from traces regardless of the traces' ignore statuses. As per usual with a ParamSet, we use
// a union on values associated with a given key and an intersect across multiple keys.
func (s *Impl) getDigestsForGrouping(ctx context.Context, groupingID schema.GroupingID, traceKeys paramtools.ParamSet) ([]schema.DigestBytes, error) {
	ctx, span := trace.StartSpan(ctx, "getDigestsForGrouping")
	defer span.End()
	// First let's try to get it from the cache.
	digestsFromCache, err := s.cacheManager.GetDigestsForGrouping(ctx, groupingID, traceKeys)
	if err != nil {
		sklog.Errorf("Error while retrieving digests for grouping from the cache: %v", err)
	}

	// Either encountered error from cache or no data was found in cache.
	if digestsFromCache == nil || err != nil {
		tracesForGroup, err := s.getTracesForGroup(ctx, groupingID, traceKeys)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		beginTile, endTile := common.GetFirstTileID(ctx), common.GetLastTileID(ctx)
		tilesInRange := make([]schema.TileID, 0, endTile-beginTile+1)
		for i := beginTile; i <= endTile; i++ {
			tilesInRange = append(tilesInRange, i)
		}
		// See diff/worker for explanation of this faster query.
		const statement = `
	SELECT DISTINCT digest FROM TiledTraceDigests
	WHERE tile_id = ANY($1) AND trace_id = ANY($2)`

		rows, err := s.db.Query(ctx, statement, tilesInRange, tracesForGroup)
		if err != nil {
			return nil, skerr.Wrapf(err, "fetching digests")
		}
		defer rows.Close()
		var rv []schema.DigestBytes
		for rows.Next() {
			var d schema.DigestBytes
			if err := rows.Scan(&d); err != nil {
				return nil, skerr.Wrap(err)
			}
			rv = append(rv, d)
		}

		// Now that we have the data, let's add it to the cache.
		err = s.cacheManager.SetDigestsForGrouping(ctx, groupingID, traceKeys, rv)
		if err != nil {
			sklog.Errorf("Error encountered when trying to set the digest for grouping data into cache: %v", err)
		}
		return rv, nil
	} else {
		return digestsFromCache, nil
	}
}

// getTracesForGroup returns all traces that match the given groupingID and the provided key/values
// in traceKeys, if any. It may return an error if traceKeys contains invalid characters that we
// cannot safely turn into a SQL query.
func (s *Impl) getTracesForGroup(ctx context.Context, groupingID schema.GroupingID, traceKeys paramtools.ParamSet) ([]schema.TraceID, error) {
	ctx, span := trace.StartSpan(ctx, "getTracesForGroup")
	span.AddAttributes(trace.Int64Attribute("num trace keys", int64(len(traceKeys))))
	defer span.End()

	statement, err := observedDigestsStatement(traceKeys)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not make valid query for %#v", traceKeys)
	}

	rows, err := s.db.Query(ctx, statement, groupingID)
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching trace ids")
	}
	defer rows.Close()
	var rv []schema.TraceID
	for rows.Next() {
		var t schema.TraceID
		if err := rows.Scan(&t); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, t)
	}
	return rv, nil
}

// observedDigestsStatement returns a SQL query that returns all trace ids, regardless of ignore
// status, that matches the given paramset and belongs to the given grouping. We put the keys and
// values directly into the query because the pgx driver does not handle the placeholders well
// when used like key -> $2; it has a hard time deducing the appropriate types.
func observedDigestsStatement(ps paramtools.ParamSet) (string, error) {
	if len(ps) == 0 {
		return `SELECT trace_id FROM Traces
WHERE grouping_id = $1`, nil
	}
	statement := "WITH\n"
	unionIndex := 0
	keys := make([]string, 0, len(ps))
	for key := range ps {
		keys = append(keys, key)
	}
	sort.Strings(keys) // sort for determinism
	for _, key := range keys {
		if key != sql.Sanitize(key) {
			return "", skerr.Fmt("Invalid query key %q", key)
		}
		if unionIndex > 0 {
			statement += ",\n"
		}
		statement += fmt.Sprintf("U%d AS (\n", unionIndex)
		for j, value := range ps[key] {
			if value != sql.Sanitize(value) {
				return "", skerr.Fmt("Invalid query value %q", value)
			}
			if j != 0 {
				statement += "\tUNION\n"
			}
			// It is important to use -> and not --> to correctly make use of the keys index.
			statement += fmt.Sprintf("\tSELECT trace_id FROM Traces WHERE keys -> '%s' = '%q'\n", key, value)
		}
		statement += ")"
		unionIndex++
	}
	statement += "\n"
	for i := 0; i < unionIndex; i++ {
		statement += fmt.Sprintf("SELECT trace_id FROM U%d\nINTERSECT\n", i)
	}
	statement += `SELECT trace_id FROM Traces WHERE grouping_id = $1`
	return statement, nil
}

// getParamsetsForRightSide fetches all the traces that produced the digests in the data and
// the keys for these traces as ParamSets. The ParamSets are mapped according to the digest
// which produced them in the given window (or the tiles that approximate this).
func (s *Impl) getParamsetsForRightSide(ctx context.Context, inputs []digestAndClosestDiffs) (map[types.Digest]paramtools.ParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "getParamsetsForRightSide")
	defer span.End()
	digestToTraces, err := s.addRightTraces(ctx, inputs)
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
func (s *Impl) addRightTraces(ctx context.Context, inputs []digestAndClosestDiffs) (map[types.Digest][]schema.TraceID, error) {
	ctx, span := trace.StartSpan(ctx, "addRightTraces")
	defer span.End()

	digestToTraces := map[types.Digest][]schema.TraceID{}
	var mutex sync.Mutex
	eg, eCtx := errgroup.WithContext(ctx)
	totalDigests := 0
	for _, input := range inputs {
		groupingID := input.groupingID
		rightDigests := input.rightDigests
		totalDigests += len(input.rightDigests)
		eg.Go(func() error {
			statement := `SELECT DISTINCT encode(digest, 'hex') AS digest, trace_id
FROM TiledTraceDigests@grouping_digest_idx
WHERE digest = ANY($1) AND grouping_id = $2 AND tile_id >= $3
`
			if s.dbType == config.Spanner {
				statement = `SELECT DISTINCT digest, trace_id
FROM TiledTraceDigests
WHERE digest = ANY($1) AND grouping_id = $2 AND tile_id >= $3
`
			}
			rows, err := s.db.Query(eCtx, statement, rightDigests, groupingID, common.GetFirstTileID(ctx))
			if err != nil {
				return skerr.Wrap(err)
			}
			defer rows.Close()
			mutex.Lock()
			defer mutex.Unlock()
			s.mutex.RLock()
			defer s.mutex.RUnlock()
			var traceKey schema.MD5Hash
			for rows.Next() {
				var digest types.Digest
				var traceID schema.TraceID
				if s.dbType == config.Spanner {
					var digestBytes []byte
					if err := rows.Scan(&digestBytes, &traceID); err != nil {
						return skerr.Wrap(err)
					}
					digest = types.Digest(hex.EncodeToString(digestBytes))
				} else {
					if err := rows.Scan(&digest, &traceID); err != nil {
						return skerr.Wrap(err)
					}
				}

				if s.publiclyVisibleTraces != nil {
					copy(traceKey[:], traceID)
					if _, ok := s.publiclyVisibleTraces[traceKey]; !ok {
						continue
					}
				}
				digestToTraces[digest] = append(digestToTraces[digest], traceID)
			}
			return nil
		})
	}
	span.AddAttributes(trace.Int64Attribute("num_right_digests", int64(totalDigests)))
	if err := eg.Wait(); err != nil {
		return nil, skerr.Wrap(err)
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
		opts, err := s.expandOptionsToParams(ctx, optionsID)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		copy(traceKey[:], traceID)
		byTrace[traceKey] = opts
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
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, skerr.Wrap(err)
	}
	s.traceCache.Add(string(traceID), keys)
	return keys.Copy(), nil // Return a copy to protect cached value
}

// fillOutTraceHistory returns a slice of SearchResults that are mostly filled in, particularly
// including the history of the traces for each result.
func (s *Impl) fillOutTraceHistory(ctx context.Context, inputs []digestAndClosestDiffs) ([]*frontend.SearchResult, error) {
	ctx, span := trace.StartSpan(ctx, "fillOutTraceHistory")
	span.AddAttributes(trace.Int64Attribute("results", int64(len(inputs))))
	defer span.End()
	// Fill out these histories in parallel. We avoid race conditions by writing to a prescribed
	// index in the results slice.
	results := make([]*frontend.SearchResult, len(inputs))
	eg, eCtx := errgroup.WithContext(ctx)
	for i, j := range inputs {
		idx, input := i, j
		eg.Go(func() error {
			sr := &frontend.SearchResult{
				Digest: types.Digest(hex.EncodeToString(input.leftDigest)),
				RefDiffs: map[frontend.RefClosest]*frontend.SRDiffDigest{
					frontend.PositiveRef: input.closestPositive,
					frontend.NegativeRef: input.closestNegative,
				},
			}
			if input.closestDigest != nil && input.closestDigest.Status == expectations.Positive {
				sr.ClosestRef = frontend.PositiveRef
			} else if input.closestDigest != nil && input.closestDigest.Status == expectations.Negative {
				sr.ClosestRef = frontend.NegativeRef
			}
			tg, err := s.traceGroupForTraces(eCtx, input.traceIDs, input.optionsIDs, sr.Digest)
			if err != nil {
				return skerr.Wrap(err)
			}
			if err := s.fillInExpectations(eCtx, &tg, input.groupingID); err != nil {
				return skerr.Wrap(err)
			}
			th, err := s.getTriageHistory(eCtx, input.groupingID, input.leftDigest)
			if err != nil {
				return skerr.Wrap(err)
			}
			sr.TriageHistory = th
			if err := s.fillInTraceParams(eCtx, &tg); err != nil {
				return skerr.Wrap(err)
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
			results[idx] = sr
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, skerr.Wrap(err)
	}
	return results, nil
}

type traceDigestCommit struct {
	commitID  schema.CommitID
	digest    types.Digest
	optionsID schema.OptionsID
}

// traceGroupForTraces gets all the history for a slice of traces within the given window and
// turns it into a format that the frontend can render. If latestOptions is provided, it is assumed
// to be a parallel slice to traceIDs - those options will be used as the options for each trace
// instead of the ones that were searched.
func (s *Impl) traceGroupForTraces(ctx context.Context, traceIDs []schema.TraceID, latestOptions []schema.OptionsID, primary types.Digest) (frontend.TraceGroup, error) {
	ctx, span := trace.StartSpan(ctx, "traceGroupForTraces")
	span.AddAttributes(trace.Int64Attribute("num_traces", int64(len(traceIDs))))
	defer span.End()

	traceData := make(map[schema.MD5Hash][]traceDigestCommit, len(traceIDs))
	// Make sure there's an entry for each trace. That way, even if the trace is not seen on
	// the primary branch (e.g. newly added in a CL), we can show something for it.
	for i := range traceIDs {
		key := sql.AsMD5Hash(traceIDs[i])
		traceData[key] = nil
		if latestOptions != nil {
			traceData[key] = append(traceData[key], traceDigestCommit{optionsID: latestOptions[i]})
		}
	}
	statement := `SELECT trace_id, commit_id, encode(digest, 'hex'), options_id
FROM TraceValues WHERE trace_id = ANY($1) AND commit_id >= $2
ORDER BY trace_id, commit_id`
	if s.dbType == config.Spanner {
		statement = `SELECT trace_id, commit_id, digest, options_id
FROM TraceValues WHERE trace_id = ANY($1) AND commit_id >= $2
ORDER BY trace_id, commit_id`
	}
	rows, err := s.db.Query(ctx, statement, traceIDs, common.GetFirstCommitID(ctx))
	if err != nil {
		return frontend.TraceGroup{}, skerr.Wrap(err)
	}
	defer rows.Close()
	var key schema.MD5Hash
	var traceID schema.TraceID
	for rows.Next() {
		var row traceDigestCommit
		if s.dbType == config.Spanner {
			var digest []byte
			if err := rows.Scan(&traceID, &row.commitID, &digest, &row.optionsID); err != nil {
				return frontend.TraceGroup{}, skerr.Wrap(err)
			}
			row.digest = types.Digest(hex.EncodeToString(digest))
		} else {
			if err := rows.Scan(&traceID, &row.commitID, &row.digest, &row.optionsID); err != nil {
				return frontend.TraceGroup{}, skerr.Wrap(err)
			}
		}
		copy(key[:], traceID)
		traceData[key] = append(traceData[key], row)
	}
	return makeTraceGroup(ctx, traceData, primary)
}

// fillInExpectations looks up all the expectations for the digests included in the given
// TraceGroup and updates the passed in TraceGroup directly.
func (s *Impl) fillInExpectations(ctx context.Context, tg *frontend.TraceGroup, groupingID schema.GroupingID) error {
	ctx, span := trace.StartSpan(ctx, "fillInExpectations")
	defer span.End()
	digests := make([]interface{}, 0, len(tg.Digests))
	for _, digestStatus := range tg.Digests {
		dBytes, err := sql.DigestToBytes(digestStatus.Digest)
		if err != nil {
			sklog.Warningf("invalid digest: %s", digestStatus.Digest)
			continue
		}
		digests = append(digests, dBytes)
	}
	arguments := []interface{}{groupingID, digests}
	statement := `
SELECT encode(digest, 'hex'), label FROM Expectations
WHERE grouping_id = $1 and digest = ANY($2)`
	if s.dbType == config.Spanner {
		statement = `
SELECT digest, label FROM Expectations
WHERE grouping_id = $1 and digest = ANY($2)`
	}
	if qCLID := common.GetQualifiedCL(ctx); qCLID != "" {
		// We use a full outer join to make sure we have the triage status from both tables
		// (with the CL expectations winning if triaged in both places)
		statement = `WITH
CLExpectations AS (
	SELECT digest, label
	FROM SecondaryBranchExpectations
	WHERE branch_name = $3 AND grouping_id = $1 AND digest = ANY($2)
),
PrimaryExpectations AS (
	SELECT digest, label FROM Expectations
	WHERE grouping_id = $1 AND digest = ANY($2)
)`
		if s.dbType == config.Spanner {
			statement += `SELECT COALESCE(CLExpectations.digest, PrimaryExpectations.digest),
			COALESCE(CLExpectations.label, COALESCE(PrimaryExpectations.label, 'u')) FROM
		CLExpectations FULL OUTER JOIN PrimaryExpectations ON
			CLExpectations.digest = PrimaryExpectations.digest`
		} else {
			statement += `SELECT encode(COALESCE(CLExpectations.digest, PrimaryExpectations.digest), 'hex'),
	COALESCE(CLExpectations.label, COALESCE(PrimaryExpectations.label, 'u')) FROM
CLExpectations FULL OUTER JOIN PrimaryExpectations ON
	CLExpectations.digest = PrimaryExpectations.digest`
		}

		arguments = append(arguments, qCLID)
	}
	rows, err := s.db.Query(ctx, statement, arguments...)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var digest types.Digest
		var label schema.ExpectationLabel
		if s.dbType == config.Spanner {
			var digestBytes []byte
			if err := rows.Scan(&digestBytes, &label); err != nil {
				return skerr.Wrap(err)
			}
			digest = types.Digest(hex.EncodeToString(digestBytes))
		} else {
			if err := rows.Scan(&digest, &label); err != nil {
				return skerr.Wrap(err)
			}
		}

		for i, ds := range tg.Digests {
			if ds.Digest == digest {
				tg.Digests[i].Status = label.ToExpectation()
			}
		}
	}
	return nil
}

func (s *Impl) getTriageHistory(ctx context.Context, groupingID schema.GroupingID, digest schema.DigestBytes) ([]frontend.TriageHistory, error) {
	ctx, span := trace.StartSpan(ctx, "getTriageHistory")
	defer span.End()

	qCLID := common.GetQualifiedCL(ctx)
	const statement = `WITH
PrimaryRecord AS (
	SELECT expectation_record_id FROM Expectations
	WHERE grouping_id = $1 AND digest = $2
),
SecondaryRecord AS (
	SELECT expectation_record_id FROM SecondaryBranchExpectations
	WHERE grouping_id = $1 AND digest = $2 AND branch_name = $3
),
CombinedRecords AS (
	SELECT expectation_record_id FROM PrimaryRecord
	UNION
	SELECT expectation_record_id FROM SecondaryRecord
)
SELECT user_name, triage_time FROM ExpectationRecords
JOIN CombinedRecords ON ExpectationRecords.expectation_record_id = CombinedRecords.expectation_record_id
ORDER BY triage_time DESC LIMIT 1`

	row := s.db.QueryRow(ctx, statement, groupingID, digest, qCLID)
	var user string
	var ts time.Time
	if err := row.Scan(&user, &ts); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, skerr.Wrap(err)
	}
	return []frontend.TriageHistory{{
		User: user,
		TS:   ts.UTC(),
	}}, nil
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

// makeGroupingAndDigestWhereClause builds the part of a "WHERE" clause that filters by grouping ID
// and digest. It returns the SQL clause and a list of parameter values.
func makeGroupingAndDigestWhereClause(triageDeltaInfos []extendedBulkTriageDeltaInfo, startingPlaceholderNum int) (string, []interface{}) {
	var parts []string
	args := make([]interface{}, 0, 2*len(triageDeltaInfos))
	placeholderNum := startingPlaceholderNum
	for _, bulkTriageDeltaInfo := range triageDeltaInfos {
		parts = append(parts, fmt.Sprintf("(grouping_id = $%d AND digest = $%d)", placeholderNum, placeholderNum+1))
		args = append(args, bulkTriageDeltaInfo.groupingID, bulkTriageDeltaInfo.digest)
		placeholderNum += 2
	}
	sort.Strings(parts) // Make the query string deterministic for easier debugging.
	return strings.Join(parts, " OR "), args
}

// findPrimaryBranchLabels returns the primary branch labels for the digests corresponding to the
// passed in extendedBulkTriageDeltaInfo structs.
func (s *Impl) findPrimaryBranchLabels(ctx context.Context, triageDeltaInfos []extendedBulkTriageDeltaInfo) (map[common.GroupingDigestKey]schema.ExpectationLabel, error) {
	ctx, span := trace.StartSpan(ctx, "findPrimaryBranchLabels")
	defer span.End()

	labels := map[common.GroupingDigestKey]schema.ExpectationLabel{}
	if len(triageDeltaInfos) == 0 {
		return labels, nil
	}
	whereClause, whereArgs := makeGroupingAndDigestWhereClause(triageDeltaInfos, 1)
	statement := "SELECT grouping_id, digest, label FROM Expectations WHERE " + whereClause
	rows, err := s.db.Query(ctx, statement, whereArgs...)
	if err != nil {
		sklog.Warningf("Error for triage delta infos %+v", triageDeltaInfos)
		return nil, skerr.Wrapf(err, "with statement %s", statement)
	}
	defer rows.Close()
	for rows.Next() {
		var groupingID schema.GroupingID
		var digest schema.DigestBytes
		var label schema.ExpectationLabel
		if err := rows.Scan(&groupingID, &digest, &label); err != nil {
			return nil, skerr.Wrap(err)
		}
		labels[common.GroupingDigestKey{
			GroupingID: sql.AsMD5Hash(groupingID),
			Digest:     sql.AsMD5Hash(digest),
		}] = label
	}
	return labels, nil
}

// findSecondaryBranchLabels returns the primary branch labels for the digests corresponding to the
// passed in extendedBulkTriageDeltaInfo structs.
func (s *Impl) findSecondaryBranchLabels(ctx context.Context, triageDeltaInfos []extendedBulkTriageDeltaInfo) (map[common.GroupingDigestKey]schema.ExpectationLabel, error) {
	ctx, span := trace.StartSpan(ctx, "findSecondaryBranchLabels")
	defer span.End()

	labels := map[common.GroupingDigestKey]schema.ExpectationLabel{}
	if len(triageDeltaInfos) == 0 {
		return labels, nil
	}
	whereClause, whereArgs := makeGroupingAndDigestWhereClause(triageDeltaInfos, 2)
	statement := `
		SELECT grouping_id,
		       digest,
			   label
		  FROM SecondaryBranchExpectations
		 WHERE branch_name = $1 AND (` + whereClause + ")"
	rows, err := s.db.Query(ctx, statement, append([]interface{}{common.GetQualifiedCL(ctx)}, whereArgs...)...)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var groupingID schema.GroupingID
		var digest schema.DigestBytes
		var label schema.ExpectationLabel
		if err := rows.Scan(&groupingID, &digest, &label); err != nil {
			return nil, skerr.Wrap(err)
		}
		labels[common.GroupingDigestKey{
			GroupingID: sql.AsMD5Hash(groupingID),
			Digest:     sql.AsMD5Hash(digest),
		}] = label
	}
	return labels, nil
}

// populateLabelBefore populates the LabelBefore field of each passed in
// extendedBulkTriageDeltaInfo with expectations for the primary branch.
//
// It mirrors the verifyPrimaryBranchLabelBefore function in web.go
// (https://skia.googlesource.com/buildbot/+/refs/heads/main/golden/go/web/web.go#1178).
func (s *Impl) populateLabelBefore(ctx context.Context, triageDeltaInfos []extendedBulkTriageDeltaInfo) error {
	ctx, span := trace.StartSpan(ctx, "populateLabelBefore")
	defer span.End()

	// Gather the relevant labels from the Expectations table.
	primaryBranchLabels, err := s.findPrimaryBranchLabels(ctx, triageDeltaInfos)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Place extendedBulkTriageDeltaInfos in a map for faster querying.
	byKey := map[common.GroupingDigestKey]*extendedBulkTriageDeltaInfo{}
	for i := range triageDeltaInfos {
		byKey[common.GroupingDigestKey{
			GroupingID: sql.AsMD5Hash(triageDeltaInfos[i].groupingID),
			Digest:     sql.AsMD5Hash(triageDeltaInfos[i].digest),
		}] = &triageDeltaInfos[i]
	}

	for key, triageDeltaInfo := range byKey {
		label, ok := primaryBranchLabels[key]
		if !ok {
			label = schema.LabelUntriaged
		}
		triageDeltaInfo.LabelBefore = label.ToExpectation()
	}

	return nil
}

// populateLabelBeforeForCL populates the LabelBefore field of each passed in
// extendedBulkTriageDeltaInfo with expectations for a CL.
//
// It mirrors the verifySecondaryBranchLabelBefore function in web.go
// (https://skia.googlesource.com/buildbot/+/refs/heads/main/golden/go/web/web.go#1231).
func (s *Impl) populateLabelBeforeForCL(ctx context.Context, triageDeltaInfos []extendedBulkTriageDeltaInfo) error {
	ctx, span := trace.StartSpan(ctx, "populateLabelBeforeCL")
	defer span.End()

	// Gather the relevant labels from the Expectations table.
	primaryBranchLabels, err := s.findPrimaryBranchLabels(ctx, triageDeltaInfos)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Gather the relevant labels from the SecondaryBranchExpectations table.
	secondaryBranchLabels, err := s.findSecondaryBranchLabels(ctx, triageDeltaInfos)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Place extendedBulkTriageDeltaInfos in a map for faster querying.
	byKey := map[common.GroupingDigestKey]*extendedBulkTriageDeltaInfo{}
	for i := range triageDeltaInfos {
		byKey[common.GroupingDigestKey{
			GroupingID: sql.AsMD5Hash(triageDeltaInfos[i].groupingID),
			Digest:     sql.AsMD5Hash(triageDeltaInfos[i].digest),
		}] = &triageDeltaInfos[i]
	}

	for key, triageDeltaInfo := range byKey {
		label, ok := secondaryBranchLabels[key]
		if !ok {
			label, ok = primaryBranchLabels[key]
		}
		if !ok {
			label = schema.LabelUntriaged
		}
		triageDeltaInfo.LabelBefore = label.ToExpectation()
	}

	return nil
}

type traceDigestKey struct {
	traceID schema.MD5Hash
	digest  schema.MD5Hash
}

// populateExtendedBulkTriageDeltaInfosOptionsIDs populates the optionsIDs field of the given
// extendedBulkTriageDeltaInfos.
func (s *Impl) populateExtendedBulkTriageDeltaInfosOptionsIDs(ctx context.Context, triageDeltaInfos []extendedBulkTriageDeltaInfo) error {
	ctx, span := trace.StartSpan(ctx, "populateExtendedBulkTriageDeltaInforsOptionsIDs")
	defer span.End()

	// Map triageDeltaInfos by trace ID and digest for faster querying, and gather all trace IDs.
	var allTraceIDs []schema.TraceID
	triageDeltaInfosByTraceAndDigest := map[traceDigestKey]*extendedBulkTriageDeltaInfo{}
	for i := range triageDeltaInfos {
		allTraceIDs = append(allTraceIDs, triageDeltaInfos[i].traceIDs...)
		for _, traceID := range triageDeltaInfos[i].traceIDs {
			key := traceDigestKey{
				traceID: sql.AsMD5Hash(traceID),
				digest:  sql.AsMD5Hash(triageDeltaInfos[i].digest),
			}
			triageDeltaInfosByTraceAndDigest[key] = &triageDeltaInfos[i]
		}
	}

	const statement = `SELECT trace_id, digest, options_id FROM TraceValues
	WHERE trace_id = ANY($1) AND commit_id >= $2`
	rows, err := s.db.Query(ctx, statement, allTraceIDs, common.GetFirstCommitID(ctx))
	if err != nil {
		return skerr.Wrap(err)
	}
	defer rows.Close()

	var traceID schema.TraceID
	var digest schema.DigestBytes
	var optionsID schema.OptionsID
	for rows.Next() {
		if err := rows.Scan(&traceID, &digest, &optionsID); err != nil {
			return skerr.Wrap(err)
		}
		key := traceDigestKey{
			traceID: sql.AsMD5Hash(traceID),
			digest:  sql.AsMD5Hash(digest),
		}
		if triageDeltaInfo, ok := triageDeltaInfosByTraceAndDigest[key]; ok {
			triageDeltaInfo.optionsIDs = append(triageDeltaInfo.optionsIDs, optionsID)
		}
	}
	return nil
}

// prepareExtendedBulkTriageDeltaInfosForFrontend turns extendedBulkTriageDeltaInfo structs into
// frontend.BulkTriageDeltaInfo structs, and filters out those with disallow_triaging=true.
func (s *Impl) prepareExtendedBulkTriageDeltaInfosForFrontend(ctx context.Context, extendedBulkTriageDeltaInfos []extendedBulkTriageDeltaInfo) ([]frontend.BulkTriageDeltaInfo, error) {
	ctx, span := trace.StartSpan(ctx, "prepareExtendedBulkTriageDeltaInfosForFrontend")
	defer span.End()

	// The frontend expects a non-null array.
	bulkTriageDeltaInfos := []frontend.BulkTriageDeltaInfo{}
	for _, triageDeltaInfo := range extendedBulkTriageDeltaInfos {
		disallowTriaging := false
		for _, optionsID := range triageDeltaInfo.optionsIDs {
			options, err := s.expandOptionsToParams(ctx, optionsID)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			if options["disallow_triaging"] == "true" {
				disallowTriaging = true
				break
			}
		}
		if !disallowTriaging {
			bulkTriageDeltaInfos = append(bulkTriageDeltaInfos, triageDeltaInfo.BulkTriageDeltaInfo)
		}
	}
	return bulkTriageDeltaInfos, nil
}

// expandGrouping returns the params associated with the grouping id. It will use the cache - if
// there is a cache miss, it will look it up, add it to the cache and return it.
func (s *Impl) expandGrouping(ctx context.Context, groupingID schema.MD5Hash) (paramtools.Params, error) {
	ctx, span := trace.StartSpan(ctx, "expandGrouping")
	defer span.End()

	var groupingKeys paramtools.Params
	if gk, ok := s.optionsGroupingCache.Get(groupingID); ok {
		return gk.(paramtools.Params), nil
	} else {
		const statement = `SELECT keys FROM Groupings WHERE grouping_id = $1`
		row := s.db.QueryRow(ctx, statement, groupingID[:])
		if err := row.Scan(&groupingKeys); err != nil {
			return nil, skerr.Wrap(err)
		}
		s.optionsGroupingCache.Add(groupingID, groupingKeys)
	}
	return groupingKeys, nil
}

// searchCLData returns the search response for the given CL's data (or an error if no such data
// exists). It reuses much of the same pipeline structure as the normal search, with a few key
// differences. It prepends the data to all traces, pretending as if the CL were to land and the
// new data would be drawn at ToT (this can be confusing for CLs which already landed).
func (s *Impl) searchCLData(ctx context.Context) (*frontend.SearchResponse, error) {
	ctx, span := trace.StartSpan(ctx, "searchCLData")
	defer span.End()
	var err error
	ctx, err = s.addCLData(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	commits, err := s.commitsProvider.GetCommits(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if commits, err = s.addCLCommit(ctx, commits); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Find all digests and traces that match the given search criteria.
	// This will be filtered according to the publiclyAllowedParams as well.
	traceDigests, err := s.getMatchingDigestsAndTracesForCL(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Lookup the closest diffs on the primary branch to the given digests. This returns a subset
	// according to the limit and offset in the query.
	// TODO(kjlubick) perhaps we want to include the digests produced by this CL/PS as well?
	closestDiffs, extendedBulkTriageDeltaInfos, err := s.getClosestDiffs(ctx, traceDigests)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Go fetch history and paramset (within this grouping, and respecting publiclyAllowedParams).
	paramsetsByDigest, err := s.getParamsetsForRightSide(ctx, closestDiffs)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Flesh out the trace history with enough data to draw the dots diagram on the frontend.
	results, err := s.fillOutTraceHistory(ctx, closestDiffs)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Populate the LabelBefore fields of the extendedBulkTriageDeltaInfos with expectations from
	// the CL.
	if err := s.populateLabelBeforeForCL(ctx, extendedBulkTriageDeltaInfos); err != nil {
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

	bulkTriageDeltaInfos, err := s.prepareExtendedBulkTriageDeltaInfosForFrontend(ctx, extendedBulkTriageDeltaInfos)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &frontend.SearchResponse{
		Results:              results,
		Offset:               common.GetQuery(ctx).Offset,
		Size:                 len(extendedBulkTriageDeltaInfos),
		BulkTriageDeltaInfos: bulkTriageDeltaInfos,
		Commits:              commits,
	}, nil
}

// addCLData returns a context with some CL-specific data added as values. If the data can not be
// verified, an error is returned.
func (s *Impl) addCLData(ctx context.Context) (context.Context, error) {
	ctx, span := trace.StartSpan(ctx, "addCLData")
	defer span.End()
	q := common.GetQuery(ctx)
	qCLID := sql.Qualify(q.CodeReviewSystemID, q.ChangelistID)
	const statement = `SELECT patchset_id FROM Patchsets WHERE
changelist_id = $1 AND system = $2 AND ps_order = $3`
	row := s.db.QueryRow(ctx, statement, qCLID, q.CodeReviewSystemID, q.Patchsets[0])
	var qPSID string
	if err := row.Scan(&qPSID); err != nil {
		if err == pgx.ErrNoRows {
			return nil, skerr.Fmt("CL %q has no PS with order %d: %#v", qCLID, q.Patchsets[0], q)
		}
		return nil, skerr.Wrap(err)
	}
	ctx = context.WithValue(ctx, common.QualifiedCLIDKey, qCLID)
	ctx = context.WithValue(ctx, common.QualifiedPSIDKey, qPSID)
	return ctx, nil
}

// addCLCommit adds a fake commit to the end of the trace data to represent the data for this CL.
func (s *Impl) addCLCommit(ctx context.Context, commits []frontend.Commit) ([]frontend.Commit, error) {
	ctx, span := trace.StartSpan(ctx, "addCLCommit")
	defer span.End()
	q := common.GetQuery(ctx)

	urlTemplate, ok := s.reviewSystemMapping[q.CodeReviewSystemID]
	if !ok {
		return nil, skerr.Fmt("unknown CRS %s", q.CodeReviewSystemID)
	}

	qCLID := sql.Qualify(q.CodeReviewSystemID, q.ChangelistID)
	const statement = `SELECT owner_email, subject, last_ingested_data FROM Changelists
WHERE changelist_id = $1`
	row := s.db.QueryRow(ctx, statement, qCLID)
	var cl schema.ChangelistRow
	if err := row.Scan(&cl.OwnerEmail, &cl.Subject, &cl.LastIngestedData); err != nil {
		return nil, skerr.Wrap(err)
	}
	return append(commits, frontend.Commit{
		CommitTime:    cl.LastIngestedData.UTC().Unix(),
		Hash:          q.ChangelistID,
		Author:        cl.OwnerEmail,
		Subject:       cl.Subject,
		ChangelistURL: fmt.Sprintf(urlTemplate, q.ChangelistID),
	}), nil
}

// getMatchingDigestsAndTracesForCL returns all data produced at the specified Changelist and
// Patchset that matches the provided query. One key difference from searching on the primary
// branch is the fact that the "AtHead" option is meaningless. Another is that we can filter out
// data seen on the primary branch. We have this as an in-memory cache because it is much much
// faster than querying it live (even with good indexing).
func (s *Impl) getMatchingDigestsAndTracesForCL(ctx context.Context) ([]common.DigestWithTraceAndGrouping, error) {
	ctx, span := trace.StartSpan(ctx, "getMatchingDigestsAndTracesForCL")
	defer span.End()
	q := common.GetQuery(ctx)
	statement := `WITH
CLDigests AS (
	SELECT secondary_branch_trace_id, digest, grouping_id, options_id
	FROM SecondaryBranchValues
	WHERE branch_name = $1 and version_name = $2
),`
	mt, err := matchingCLTracesStatement(q.TraceValues, q.IncludeIgnoredTraces)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	statement += mt
	statement += `
MatchingCLDigests AS (
	SELECT trace_id, digest, grouping_id, options_id
	FROM CLDigests JOIN MatchingTraces
		ON CLDigests.secondary_branch_trace_id = MatchingTraces.trace_id
),
CLExpectations AS (
	SELECT grouping_id, digest, label
	FROM SecondaryBranchExpectations
	WHERE branch_name = $1
)
SELECT trace_id, MatchingCLDigests.grouping_id, MatchingCLDigests.digest, options_id
FROM MatchingCLDigests
LEFT JOIN Expectations
	ON MatchingCLDigests.grouping_id = Expectations.grouping_id AND
	MatchingCLDigests.digest = Expectations.digest
LEFT JOIN CLExpectations
	ON MatchingCLDigests.grouping_id = CLExpectations.grouping_id AND
	MatchingCLDigests.digest = CLExpectations.digest
WHERE COALESCE(CLExpectations.label, COALESCE(Expectations.label, 'u')) = ANY($3)
`

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

	rows, err := s.db.Query(ctx, statement, common.GetQualifiedCL(ctx), common.GetQualifiedPS(ctx), triageStatuses)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var rv []common.DigestWithTraceAndGrouping
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	var traceKey schema.MD5Hash
	var key common.GroupingDigestKey
	keyGrouping := key.GroupingID[:]
	keyDigest := key.Digest[:]
	for rows.Next() {
		var row common.DigestWithTraceAndGrouping
		if err := rows.Scan(&row.TraceID, &row.GroupingID, &row.Digest, &row.OptionsID); err != nil {
			return nil, skerr.Wrap(err)
		}
		if s.publiclyVisibleTraces != nil {
			copy(traceKey[:], row.TraceID)
			if _, ok := s.publiclyVisibleTraces[traceKey]; !ok {
				continue
			}
		}
		if !q.IncludeDigestsProducedOnMaster {
			copy(keyGrouping, row.GroupingID)
			copy(keyDigest, row.Digest)
			if _, existsOnPrimary := s.digestsOnPrimary[key]; existsOnPrimary {
				continue
			}
		}
		rv = append(rv, row)
	}
	return rv, nil
}

// matchingCLTracesStatement returns
func matchingCLTracesStatement(ps paramtools.ParamSet, includeIgnored bool) (string, error) {
	corpora := ps[types.CorpusField]
	if len(corpora) == 0 {
		return "", skerr.Fmt("Corpus must be specified: %v", ps)
	}
	corpus := corpora[0]
	if corpus != sql.Sanitize(corpus) {
		return "", skerr.Fmt("Invalid corpus: %q", corpus)
	}
	corpusStatement := `SELECT trace_id FROM Traces WHERE corpus = '` + corpus + `' AND (matches_any_ignore_rule `
	if includeIgnored {
		corpusStatement += "IS NOT NULL)"
	} else {
		corpusStatement += "= FALSE OR matches_any_ignore_rule is NULL)"
	}
	if len(ps) == 1 {
		return "MatchingTraces AS (\n\t" + corpusStatement + "\n),", nil
	}
	statement := ""
	unionIndex := 0
	keys := make([]string, 0, len(ps))
	for key := range ps {
		keys = append(keys, key)
	}
	sort.Strings(keys) // sort for determinism
	for _, key := range keys {
		if key == types.CorpusField {
			continue
		}
		if key != sql.Sanitize(key) {
			return "", skerr.Fmt("Invalid query key %q", key)
		}
		statement += fmt.Sprintf("U%d AS (\n", unionIndex)
		for j, value := range ps[key] {
			if value != sql.Sanitize(value) {
				return "", skerr.Fmt("Invalid query value %q", value)
			}
			if j != 0 {
				statement += "\tUNION\n"
			}
			statement += fmt.Sprintf("\tSELECT trace_id FROM Traces WHERE keys -> '%s' = '%q'\n", key, value)
		}
		statement += "),\n"
		unionIndex++
	}
	statement += "MatchingTraces AS (\n"
	for i := 0; i < unionIndex; i++ {
		statement += fmt.Sprintf("\tSELECT trace_id FROM U%d\n\tINTERSECT\n", i)
	}
	// Include a final intersect for the corpus
	statement += "\t" + corpusStatement + "\n),\n"
	return statement, nil
}

// makeTraceGroup converts all the trace+digest+commit triples into a TraceGroup. On the frontend,
// we only show the top 9 digests before fading them to grey - this handles that logic.
// It is assumed that the slices in the data map are in ascending order of commits.
func makeTraceGroup(ctx context.Context, data map[schema.MD5Hash][]traceDigestCommit, primary types.Digest) (frontend.TraceGroup, error) {
	ctx, span := trace.StartSpan(ctx, "makeTraceGroup")
	defer span.End()
	isCL := common.GetQualifiedCL(ctx) != ""
	tg := frontend.TraceGroup{}
	if len(data) == 0 {
		return tg, nil
	}
	traceLength := common.GetActualWindowLength(ctx)
	if isCL {
		traceLength++ // We will append the current data to the end.
	}
	indexMap := common.GetCommitToIdxMap(ctx)
	for trID, points := range data {
		currentTrace := frontend.Trace{
			ID:            tiling.TraceID(hex.EncodeToString(trID[:])),
			DigestIndices: emptyIndices(traceLength),
			RawTrace:      tiling.NewEmptyTrace(traceLength, nil, nil),
		}
		for _, dp := range points {
			if dp.optionsID != nil {
				// We want to report the latest options, so always update this if non-nil.
				currentTrace.RawTrace.OptionsID = dp.optionsID
			}
			idx, ok := indexMap[dp.commitID]
			if !ok {
				continue
			}
			currentTrace.RawTrace.Digests[idx] = dp.digest
		}
		tg.Traces = append(tg.Traces, currentTrace)
	}
	// Sort traces by ID for determinism
	sort.Slice(tg.Traces, func(i, j int) bool {
		return tg.Traces[i].ID < tg.Traces[j].ID
	})

	// Find the most recent / important digests and assign them an index. Everything else will
	// be given the sentinel value.
	digestIndices, totalDigests := computeDigestIndices(&tg, primary)
	tg.TotalDigests = totalDigests

	tg.Digests = make([]frontend.DigestStatus, len(digestIndices))
	for digest, idx := range digestIndices {
		tg.Digests[idx] = frontend.DigestStatus{
			Digest: digest,
			Status: expectations.Untriaged,
		}
	}

	for _, tr := range tg.Traces {
		for j, digest := range tr.RawTrace.Digests {
			if j == len(tr.RawTrace.Digests)-1 && isCL {
				// Put the CL Data (the primary digest here, aka index 0) as happening most
				// recently at head.
				tr.DigestIndices[j] = 0
				continue
			}
			if digest == tiling.MissingDigest {
				continue // There is already the missing index there.
			}
			idx, ok := digestIndices[digest]
			if ok {
				tr.DigestIndices[j] = idx
			} else {
				// Fold everything else into the last digest index (grey on the frontend).
				tr.DigestIndices[j] = maxDistinctDigestsToPresent - 1
			}
		}
	}
	return tg, nil
}

// emptyIndices returns an array of the given length with placeholder values for "missing data".
func emptyIndices(length int) []int {
	rv := make([]int, length)
	for i := range rv {
		rv[i] = missingDigestIndex
	}
	return rv
}

// GetPrimaryBranchParamset returns a possibly cached ParamSet of all visible traces over the last
// N tiles that correspond to the windowLength.
func (s *Impl) GetPrimaryBranchParamset(ctx context.Context) (paramtools.ReadOnlyParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "search2_GetPrimaryBranchParamset")
	defer span.End()
	const primaryBranchKey = "primary_branch"
	if val, ok := s.paramsetCache.Get(primaryBranchKey); ok {
		return val.(paramtools.ReadOnlyParamSet), nil
	}
	ctx, err := s.addCommitsData(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if s.isPublicView {
		roPS, err := s.getPublicParamsetForPrimaryBranch(ctx)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		s.paramsetCache.Set(primaryBranchKey, roPS, 5*time.Minute)
		return roPS, nil
	}

	const statement = `SELECT DISTINCT key, value FROM PrimaryBranchParams
WHERE tile_id >= $1`
	rows, err := s.db.Query(ctx, statement, common.GetFirstTileID(ctx))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	ps := paramtools.ParamSet{}
	var key string
	var value string

	for rows.Next() {
		if err := rows.Scan(&key, &value); err != nil {
			return nil, skerr.Wrap(err)
		}
		ps[key] = append(ps[key], value) // We rely on the SQL query to deduplicate values
	}
	ps.Normalize()
	roPS := paramtools.ReadOnlyParamSet(ps)
	s.paramsetCache.Set(primaryBranchKey, roPS, 5*time.Minute)
	return roPS, nil
}

func (s *Impl) getPublicParamsetForPrimaryBranch(ctx context.Context) (paramtools.ReadOnlyParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "getPublicParamsetForPrimaryBranch")
	defer span.End()

	const statement = `SELECT trace_id, keys FROM ValuesAtHead WHERE most_recent_commit_id >= $1`
	rows, err := s.db.Query(ctx, statement, common.GetFirstCommitID(ctx))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	combinedParamset := paramtools.ParamSet{}
	var ps paramtools.Params
	var traceID schema.TraceID
	var traceKey schema.MD5Hash

	s.mutex.RLock()
	for rows.Next() {
		if err := rows.Scan(&traceID, &ps); err != nil {
			s.mutex.RUnlock()
			return nil, skerr.Wrap(err)
		}
		copy(traceKey[:], traceID)
		if _, ok := s.publiclyVisibleTraces[traceKey]; ok {
			combinedParamset.AddParams(ps)
		}
	}
	s.mutex.RUnlock()
	combinedParamset.Normalize()
	return paramtools.ReadOnlyParamSet(combinedParamset), nil
}

// GetChangelistParamset returns a possibly cached ParamSet of all visible traces seen in the
// given changelist. It returns an error if no data has been seen for the given CL.
func (s *Impl) GetChangelistParamset(ctx context.Context, crs, clID string) (paramtools.ReadOnlyParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "search2_GetChangelistParamset")
	defer span.End()
	qCLID := sql.Qualify(crs, clID)
	if val, ok := s.paramsetCache.Get(qCLID); ok {
		return val.(paramtools.ReadOnlyParamSet), nil
	}
	if s.isPublicView {
		roPS, err := s.getPublicParamsetForCL(ctx, qCLID)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		s.paramsetCache.Set(qCLID, roPS, 5*time.Minute)
		return roPS, nil
	}
	const statement = `SELECT DISTINCT key, value FROM SecondaryBranchParams
WHERE branch_name = $1`
	rows, err := s.db.Query(ctx, statement, qCLID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	ps := paramtools.ParamSet{}
	var key string
	var value string

	for rows.Next() {
		if err := rows.Scan(&key, &value); err != nil {
			return nil, skerr.Wrap(err)
		}
		ps[key] = append(ps[key], value) // We rely on the SQL query to deduplicate values
	}
	if len(ps) == 0 {
		return nil, skerr.Fmt("Could not find params for CL %q in system %q", clID, crs)
	}
	ps.Normalize()
	roPS := paramtools.ReadOnlyParamSet(ps)
	s.paramsetCache.Set(qCLID, roPS, 5*time.Minute)
	return roPS, nil
}

func (s *Impl) getPublicParamsetForCL(ctx context.Context, qualifiedCLID string) (paramtools.ReadOnlyParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "getPublicParamsetForCL")
	defer span.End()

	const statement = `SELECT secondary_branch_trace_id FROM SecondaryBranchValues
WHERE branch_name = $1
`
	rows, err := s.db.Query(ctx, statement, qualifiedCLID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var traceID schema.TraceID
	var traceKey schema.MD5Hash
	var traceIDsToExpand []schema.TraceID
	s.mutex.RLock()
	for rows.Next() {
		if err := rows.Scan(&traceID); err != nil {
			s.mutex.RUnlock()
			return nil, skerr.Wrap(err)
		}
		copy(traceKey[:], traceID)
		if _, ok := s.publiclyVisibleTraces[traceKey]; ok {
			traceIDsToExpand = append(traceIDsToExpand, traceID)
		}
	}
	s.mutex.RUnlock()
	rows.Close()

	combinedParamset := paramtools.ParamSet{}
	for _, traceID := range traceIDsToExpand {
		ps, err := s.expandTraceToParams(ctx, traceID)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		combinedParamset.AddParams(ps)
	}

	combinedParamset.Normalize()
	return paramtools.ReadOnlyParamSet(combinedParamset), nil
}

// GetBlamesForUntriagedDigests implements the API interface.
func (s *Impl) GetBlamesForUntriagedDigests(ctx context.Context, corpus string) (BlameSummaryV1, error) {
	ctx, span := trace.StartSpan(ctx, "search2_GetBlamesForUntriagedDigests")
	defer span.End()

	ctx, err := s.addCommitsData(ctx)
	if err != nil {
		return BlameSummaryV1{}, skerr.Wrap(err)
	}
	// Find untriaged digests at head and the traces that produced them.
	tracesByDigest, err := s.traceDigestsProvider.GetTracesWithUntriagedDigestsAtHead(ctx, corpus)
	if err != nil {
		return BlameSummaryV1{}, skerr.Wrap(err)
	}
	if s.isPublicView {
		tracesByDigest = s.applyPublicFilter(ctx, tracesByDigest)
	}
	if len(tracesByDigest) == 0 {
		return BlameSummaryV1{}, nil // No data, we can stop here
	}
	// Return the trace histories for those traces, as well as a mapping of the unique
	// digest+grouping pairs in order to get expectations.
	histories, _, err := s.getHistoriesForTraces(ctx, tracesByDigest)
	if err != nil {
		return BlameSummaryV1{}, skerr.Wrap(err)
	}
	// Expand grouping_ids into full params
	groupings, err := s.expandGroupings(ctx, tracesByDigest)
	if err != nil {
		return BlameSummaryV1{}, skerr.Wrap(err)
	}
	commits, err := s.commitsProvider.GetCommits(ctx)
	if err != nil {
		return BlameSummaryV1{}, skerr.Wrap(err)
	}
	// Look at trace histories and identify ranges of commits that caused us to go from drawing
	// triaged digests to untriaged digests.
	ranges := combineIntoRanges(ctx, histories, groupings, commits)
	return BlameSummaryV1{
		Ranges: ranges,
	}, nil
}

type traceData []types.Digest

// applyPublicFilter filters the traces according to the publicly visible traces map.
func (s *Impl) applyPublicFilter(ctx context.Context, data map[common.GroupingDigestKey][]schema.TraceID) map[common.GroupingDigestKey][]schema.TraceID {
	ctx, span := trace.StartSpan(ctx, "applyPublicFilter")
	defer span.End()
	filtered := make(map[common.GroupingDigestKey][]schema.TraceID, len(data))
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	var traceKey schema.MD5Hash
	traceID := traceKey[:]
	for gdk, traces := range data {
		for _, tr := range traces {
			copy(traceID, tr)
			if _, ok := s.publiclyVisibleTraces[traceKey]; ok {
				filtered[gdk] = append(filtered[gdk], tr)
			}
		}
	}
	return filtered
}

// untriagedDigestAtHead represents a single untriaged digest in a particular grouping observed
// at head. It includes the histories of all traces that are
type untriagedDigestAtHead struct {
	atHead common.GroupingDigestKey
	traces []traceIDAndData
}

type traceIDAndData struct {
	id   schema.TraceID
	data traceData
}

// getHistoriesForTraces looks up the commits in the current window (aka the trace history) for all
// traces in the provided map. It returns them associated with the digest+grouping that was
// produced at head, as well as a map corresponding to all unique digests seen in these histories
// (to look up expectations).
func (s *Impl) getHistoriesForTraces(ctx context.Context, traces map[common.GroupingDigestKey][]schema.TraceID) ([]untriagedDigestAtHead, map[common.GroupingDigestKey]bool, error) {
	ctx, span := trace.StartSpan(ctx, "getHistoriesForTraces")
	defer span.End()
	tracesToDigest := map[schema.MD5Hash]common.GroupingDigestKey{}
	var tracesToLookup []schema.TraceID
	for gdk, traceIDs := range traces {
		for _, traceID := range traceIDs {
			tracesToDigest[sql.AsMD5Hash(traceID)] = gdk
			tracesToLookup = append(tracesToLookup, traceID)
		}
	}
	span.AddAttributes(trace.Int64Attribute("num_traces", int64(len(tracesToLookup))))
	const statement = `SELECT trace_id, commit_id, digest FROM TraceValues
WHERE commit_id >= $1 and trace_id = ANY($2)
ORDER BY trace_id`
	rows, err := s.db.Query(ctx, statement, common.GetFirstCommitID(ctx), tracesToLookup)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	defer rows.Close()
	traceLength := common.GetActualWindowLength(ctx)
	commitIdxMap := common.GetCommitToIdxMap(ctx)
	tracesByDigest := make(map[common.GroupingDigestKey][]traceIDAndData, len(traces))
	uniqueDigestsByGrouping := map[common.GroupingDigestKey]bool{}
	var currentTraceID schema.TraceID
	var currentTraceData traceData
	var key schema.MD5Hash
	for rows.Next() {
		var traceID schema.TraceID
		var commitID schema.CommitID
		var digest schema.DigestBytes
		if err := rows.Scan(&traceID, &commitID, &digest); err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		copy(key[:], traceID)
		gdk := tracesToDigest[key]
		// Note that we've seen this digest on this grouping so we can look up the expectations.
		uniqueDigestsByGrouping[common.GroupingDigestKey{
			GroupingID: gdk.GroupingID,
			Digest:     sql.AsMD5Hash(digest),
		}] = true
		if !bytes.Equal(traceID, currentTraceID) || currentTraceData == nil {
			currentTraceID = traceID
			// Make a new slice of digests (traceData) and associated it with the correct
			// grouping+digest
			currentTraceData = make(traceData, traceLength)
			tracesByDigest[gdk] = append(tracesByDigest[gdk], traceIDAndData{
				id:   currentTraceID,
				data: currentTraceData,
			})
		}
		idx, ok := commitIdxMap[commitID]
		if !ok {
			continue // commit is out of range or too new
		}
		currentTraceData[idx] = types.Digest(hex.EncodeToString(digest))
	}
	// Flatten the map into a sorted slice for determinism
	var rv []untriagedDigestAtHead
	for key, traces := range tracesByDigest {
		rv = append(rv, untriagedDigestAtHead{
			atHead: key,
			traces: traces,
		})
	}
	sort.Slice(rv, func(i, j int) bool {
		return bytes.Compare(rv[i].atHead.Digest[:], rv[j].atHead.Digest[:]) <= 0
	})
	return rv, uniqueDigestsByGrouping, nil
}

// expectationKey represents a digest+grouping in a way that is easier to look up from the data
// in a trace history (e.g. a hex-encoded digest).
type expectationKey struct {
	groupingID schema.MD5Hash
	digest     types.Digest
}

// getExpectations looks up the expectations for the given pairs of digests+grouping.
func (s *Impl) getExpectations(ctx context.Context, digests map[common.GroupingDigestKey]bool) (map[expectationKey]expectations.Label, error) {
	ctx, span := trace.StartSpan(ctx, "getExpectations")
	defer span.End()
	byGrouping := map[schema.MD5Hash][]schema.DigestBytes{}
	for gdk := range digests {
		byGrouping[gdk.GroupingID] = append(byGrouping[gdk.GroupingID], sql.FromMD5Hash(gdk.Digest))
	}

	exp := map[expectationKey]expectations.Label{}
	var mutex sync.Mutex
	eg, eCtx := errgroup.WithContext(ctx)
	for g, d := range byGrouping {
		groupingKey, digests := g, d
		eg.Go(func() error {
			const statement = `SELECT encode(digest, 'hex'), label FROM Expectations
WHERE grouping_id = $1 and digest = ANY($2)`
			rows, err := s.db.Query(eCtx, statement, groupingKey[:], digests)
			if err != nil {
				return skerr.Wrap(err)
			}
			defer rows.Close()
			mutex.Lock()
			defer mutex.Unlock()
			var digest types.Digest
			var label schema.ExpectationLabel
			for rows.Next() {
				if err := rows.Scan(&digest, &label); err != nil {
					return skerr.Wrap(err)
				}
				exp[expectationKey{
					groupingID: groupingKey,
					digest:     digest,
				}] = label.ToExpectation()
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, skerr.Wrap(err)
	}
	return exp, nil
}

// expandGroupings returns a map of schema.GroupingIDs (as md5 hashes) to their related params.
func (s *Impl) expandGroupings(ctx context.Context, groupings map[common.GroupingDigestKey][]schema.TraceID) (map[schema.MD5Hash]paramtools.Params, error) {
	ctx, span := trace.StartSpan(ctx, "getHistoriesForTraces")
	defer span.End()

	rv := map[schema.MD5Hash]paramtools.Params{}
	for gdk := range groupings {
		if _, ok := rv[gdk.GroupingID]; ok {
			continue
		}
		grouping, err := s.expandGrouping(ctx, gdk.GroupingID)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv[gdk.GroupingID] = grouping
	}
	return rv, nil
}

// combineIntoRanges looks at the histories for all the traces provided starting at the earliest
// (head) and working backwards. It looks for the change from drawing the untriaged digests at head
// to drawing a different digest, and tries to identify which commits caused that. There could be
// multiple commits in the window that have affected different tests, so this algorithm combines
// ranges and returns them as a slice, with the commits that produced the most untriaged digests
// coming first. It is recommended to look at the tests for this function to see some examples.
func combineIntoRanges(ctx context.Context, digests []untriagedDigestAtHead, groupings map[schema.MD5Hash]paramtools.Params, commits []frontend.Commit) []BlameEntry {
	ctx, span := trace.StartSpan(ctx, "combineIntoRanges")
	defer span.End()

	type tracesByBlameRange map[string][]schema.TraceID

	entriesByRange := map[string]BlameEntry{}
	for _, data := range digests {
		key := data.atHead
		// The digest in key represents an untriaged digest seen at head.
		// traces represents all the traces that are drawing that untriaged digest at head.
		// We would like to identify the narrowest range that this change could have happened.
		latestUntriagedDigest := types.Digest(hex.EncodeToString(sql.FromMD5Hash(key.Digest)))
		// Last commit before the untriaged digest at head was first produced.
		blameStartIdx := -1
		// First commit to produce the untriaged digest at head.
		blameEndIdx := len(commits)
		// (trace ID, digest) pairs for all traces that are drawing the untriaged digest at head.
		var traceIDsAndDigests []traceIDAndDigest

		for _, tr := range data.traces {
			traceIDsAndDigests = append(traceIDsAndDigests, traceIDAndDigest{
				id:     tr.id,
				digest: key.Digest[:],
			})

			// Identify the range at which the latest untriaged digest first occurred. For example,
			// the range at which the latest untriaged digest "c" first occurred in trace
			// "AAA-b--cc-" is 5:7.
			latestUntriagedDigestFound := false
			latestUntriagedDigestEarliestOccurrenceStartIdx := -1
			latestUntriagedDigestEarliestOccurrenceEndIdx := len(commits)

			for i := len(tr.data) - 1; i >= 0; i-- {
				digest := tr.data[i]
				if !latestUntriagedDigestFound {
					if digest == tiling.MissingDigest {
						continue
					} else if digest == latestUntriagedDigest {
						latestUntriagedDigestFound = true
					} else {
						break
					}
				}
				if digest == latestUntriagedDigest {
					latestUntriagedDigestEarliestOccurrenceStartIdx = i
					latestUntriagedDigestEarliestOccurrenceEndIdx = i
				} else if digest == tiling.MissingDigest {
					latestUntriagedDigestEarliestOccurrenceStartIdx = i
				} else {
					break
				}
			}

			// If the current blame range, and the range at which the latest untriaged digest first
			// occurred are disjoint, use the earliest of the two as the new blame range.
			//
			// Example traces:
			//
			//      AA--cccccc
			//      BABABB--cc
			//
			// In this example, the second trace is very flaky, and the untriaged digest is not
			// produced until several commits after the offending commit landed. The resulting
			// ranges are 2:4 and 6:8.
			//
			// We use the earliest range as the new blame range (2:4 in the above example) as that
			// is where the offending commit is likely found.
			disjointRanges := blameEndIdx < latestUntriagedDigestEarliestOccurrenceStartIdx ||
				latestUntriagedDigestEarliestOccurrenceEndIdx < blameStartIdx+1
			if disjointRanges {
				if latestUntriagedDigestEarliestOccurrenceEndIdx < blameStartIdx+1 {
					// Update blame range to equal the earliest of the two ranges.
					blameStartIdx = latestUntriagedDigestEarliestOccurrenceStartIdx - 1
					blameEndIdx = latestUntriagedDigestEarliestOccurrenceEndIdx
				} else {
					// Nothing to do, as the current blame range is the earliest of the two ranges.
				}
			} else {
				if blameStartIdx < latestUntriagedDigestEarliestOccurrenceStartIdx-1 {
					blameStartIdx = latestUntriagedDigestEarliestOccurrenceStartIdx - 1
				}
				if blameEndIdx > latestUntriagedDigestEarliestOccurrenceEndIdx {
					blameEndIdx = latestUntriagedDigestEarliestOccurrenceEndIdx
				}
			}
		}

		// blameStartIdx is now either -1 (for beginning of tile) or the index of the last known
		// digest that was different from the untriaged digest at head.
		if blameStartIdx == -1 && blameEndIdx == len(commits) {
			continue // We didn't find any untriaged digests on this trace
		}
		// We know have identified a blame range that has accounted for one additional untriaged
		// digest at head (and possibly others before that).
		blameRange, blameCommits := getRangeAndBlame(commits, blameStartIdx, blameEndIdx)
		entry, ok := entriesByRange[blameRange]
		if !ok {
			entry.CommitRange = blameRange
			entry.Commits = blameCommits
		}
		entry.TotalUntriagedDigests++
		// Find the grouping associated with this digest if it already is in the list.
		found := false
		for _, ag := range entry.AffectedGroupings {
			if ag.groupingID == key.GroupingID {
				found = true
				ag.UntriagedDigests++
				ag.traceIDsAndDigests = append(ag.traceIDsAndDigests, traceIDsAndDigests...)
				break
			}
		}
		if !found {
			entry.AffectedGroupings = append(entry.AffectedGroupings, &AffectedGrouping{
				Grouping:           groupings[key.GroupingID],
				UntriagedDigests:   1,
				SampleDigest:       types.Digest(hex.EncodeToString(key.Digest[:])),
				groupingID:         key.GroupingID,
				traceIDsAndDigests: traceIDsAndDigests,
			})
		}
		entriesByRange[blameRange] = entry
	}
	// Sort data so the "biggest changes" come first (and perform other cleanups)
	blameEntries := make([]BlameEntry, 0, len(entriesByRange))
	for _, entry := range entriesByRange {
		sort.Slice(entry.AffectedGroupings, func(i, j int) bool {
			if entry.AffectedGroupings[i].UntriagedDigests == entry.AffectedGroupings[j].UntriagedDigests {
				// Tiebreak on sample digest
				return entry.AffectedGroupings[i].SampleDigest < entry.AffectedGroupings[j].SampleDigest
			}
			// Otherwise, put the grouping with the most digests first
			return entry.AffectedGroupings[i].UntriagedDigests > entry.AffectedGroupings[j].UntriagedDigests
		})
		for _, ag := range entry.AffectedGroupings {
			// Sort the traceIDsAndDigests to ensure a deterministic response.
			sort.Slice(ag.traceIDsAndDigests, func(i, j int) bool {
				traceComparison := bytes.Compare(ag.traceIDsAndDigests[i].id, ag.traceIDsAndDigests[j].id)
				if traceComparison == -1 {
					return true
				} else if traceComparison == 0 {
					// Tiebreak on the digest.
					if bytes.Compare(ag.traceIDsAndDigests[i].digest, ag.traceIDsAndDigests[j].digest) == -1 {
						return true
					}
					return false
				}
				return false
			})
		}
		blameEntries = append(blameEntries, entry)
	}
	// Sort so those ranges with more untriaged digests come first.
	sort.Slice(blameEntries, func(i, j int) bool {
		if blameEntries[i].TotalUntriagedDigests == blameEntries[j].TotalUntriagedDigests {
			// tie break on the commit range
			return blameEntries[i].CommitRange < blameEntries[j].CommitRange
		}
		return blameEntries[i].TotalUntriagedDigests > blameEntries[j].TotalUntriagedDigests
	})
	return blameEntries
}

// getRangeAndBlame returns a range identifier (either a single commit id or a start and end
// commit id separated by a colon) and the corresponding web commit objects.
func getRangeAndBlame(commits []frontend.Commit, startIndex, endIndex int) (string, []frontend.Commit) {
	endCommit := commits[endIndex]
	// If the indexes are within 1 (or rarely, equal), we have pinned the range down to one commit.
	// If startIndex is -1, then we have no data all the way to the beginning of the window - this
	// commonly happens when a new test is added.
	if (endIndex-startIndex) == 1 || startIndex == endIndex || startIndex == -1 {
		return endCommit.ID, []frontend.Commit{endCommit}
	}
	// Add 1 because startIndex is the last known "good" index, and we want our blamelist to only
	// encompass "bad" commits.
	startCommit := commits[startIndex+1]
	return fmt.Sprintf("%s:%s", startCommit.ID, endCommit.ID), commits[startIndex+1 : endIndex+1]
}

// GetCluster implements the API interface.
// TODO(kjlubick) Handle CL data (frontend currently does not).
func (s *Impl) GetCluster(ctx context.Context, opts ClusterOptions) (frontend.ClusterDiffResult, error) {
	ctx, span := trace.StartSpan(ctx, "search2_GetCluster")
	defer span.End()

	ctx, err := s.addCommitsData(ctx)
	if err != nil {
		return frontend.ClusterDiffResult{}, skerr.Wrap(err)
	}
	// Fetch all digests and traces that match the options. If this is a public view, those traces
	// will be filtered.
	digestsAndTraces, err := s.getDigestsAndTracesForCluster(ctx, opts)
	if err != nil {
		return frontend.ClusterDiffResult{}, skerr.Wrap(err)
	}
	nodes, links, err := s.getLinks(ctx, digestsAndTraces)
	if err != nil {
		return frontend.ClusterDiffResult{}, skerr.Wrap(err)
	}
	byDigest, combined, err := s.getParamsetsForCluster(ctx, digestsAndTraces)
	if err != nil {
		return frontend.ClusterDiffResult{}, skerr.Wrap(err)
	}

	return frontend.ClusterDiffResult{
		Nodes:            nodes,
		Links:            links,
		Test:             types.TestName(opts.Grouping[types.PrimaryKeyField]),
		ParamsetByDigest: byDigest,
		ParamsetsUnion:   combined,
	}, nil
}

type digestClusterInfo struct {
	label      schema.ExpectationLabel
	traceIDs   []schema.TraceID
	optionsIDs []schema.OptionsID
}

// getDigestsAndTracesForCluster returns the digests, traces, and options *at head* that match the
// given cluster options.
func (s *Impl) getDigestsAndTracesForCluster(ctx context.Context, opts ClusterOptions) (map[schema.MD5Hash]*digestClusterInfo, error) {
	ctx, span := trace.StartSpan(ctx, "getDigestsAndTracesForCluster")
	defer span.End()
	statement := "WITH "
	dataOfInterest, err := clusterDataOfInterestStatement(opts)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	statement += dataOfInterest + `
SELECT DataOfInterest.*, label
FROM DataOfInterest JOIN Expectations
	ON Expectations.grouping_id = $1 and DataOfInterest.digest = Expectations.digest
WHERE label = ANY($3)
`
	var triageStatuses []schema.ExpectationLabel
	if opts.IncludeUntriagedDigests {
		triageStatuses = append(triageStatuses, schema.LabelUntriaged)
	}
	if opts.IncludeNegativeDigests {
		triageStatuses = append(triageStatuses, schema.LabelNegative)
	}
	if opts.IncludePositiveDigests {
		triageStatuses = append(triageStatuses, schema.LabelPositive)
	}
	if len(triageStatuses) == 0 {
		return nil, nil // If no triage status is set, there can be no results.
	}

	_, groupingID := sql.SerializeMap(opts.Grouping)
	rows, err := s.db.Query(ctx, statement, groupingID, common.GetFirstCommitID(ctx), triageStatuses)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var digestKey schema.MD5Hash
	var digest schema.DigestBytes
	rv := map[schema.MD5Hash]*digestClusterInfo{}
	for rows.Next() {
		var traceID schema.TraceID
		var optionsID schema.OptionsID
		var label schema.ExpectationLabel
		if err := rows.Scan(&traceID, &optionsID, &digest, &label); err != nil {
			return nil, skerr.Wrap(err)
		}
		copy(digestKey[:], digest)
		info := rv[digestKey]
		if info == nil {
			info = &digestClusterInfo{
				label: label,
			}
			rv[digestKey] = info
		}
		info.traceIDs = append(info.traceIDs, traceID)
		info.optionsIDs = append(info.optionsIDs, optionsID)
	}
	if s.isPublicView {
		s.mutex.RLock()
		defer s.mutex.RUnlock()
		for _, info := range rv {
			var filteredTraceIDs []schema.TraceID
			var filteredOptionIDs []schema.OptionsID
			var traceKey schema.MD5Hash
			tr := traceKey[:]
			for i := range info.traceIDs {
				copy(tr, info.traceIDs[i])
				if _, ok := s.publiclyVisibleTraces[traceKey]; ok {
					filteredTraceIDs = append(filteredTraceIDs, info.traceIDs[i])
					filteredOptionIDs = append(filteredOptionIDs, info.optionsIDs[i])
				}
			}
			info.traceIDs = filteredTraceIDs
			info.optionsIDs = filteredOptionIDs
		}
	}
	return rv, nil
}

// clusterDataOfInterestStatement returns a statement called DataOfInterest that contains
// the trace_id, options_id, digest from the ValuesAtHead table from the traces that match
// the given options. It will make use of the $1 placeholder for grouping_id and $2 for
// most_recent_commit_id
func clusterDataOfInterestStatement(opts ClusterOptions) (string, error) {
	if len(opts.Filters) == 0 {
		return `
DataOfInterest AS (
	SELECT trace_id, options_id, digest FROM ValuesAtHead
	WHERE grouping_id = $1 AND matches_any_ignore_rule = FALSE AND most_recent_commit_id >= $2
)`, nil
	}
	keys := make([]string, 0, len(opts.Filters))
	for key := range opts.Filters {
		keys = append(keys, key)
	}
	sort.Strings(keys) // sort for determinism

	statement := "\n"
	unionIndex := 0
	for _, key := range keys {
		if key != sql.Sanitize(key) {
			return "", skerr.Fmt("Invalid query key %q", key)
		}
		statement += fmt.Sprintf("U%d AS (\n", unionIndex)
		for j, value := range opts.Filters[key] {
			if value != sql.Sanitize(value) {
				return "", skerr.Fmt("Invalid query value %q", value)
			}
			if j != 0 {
				statement += "\tUNION\n"
			}
			statement += fmt.Sprintf("\tSELECT trace_id FROM Traces WHERE keys -> '%s' = '%q'\n", key, value)
		}
		statement += "),\n"
		unionIndex++
	}
	statement += "TracesOfInterest AS (\n"
	for i := 0; i < unionIndex; i++ {
		statement += fmt.Sprintf("\tSELECT trace_id FROM U%d\n\tINTERSECT\n", i)
	}
	// Include a final intersect for the corpus
	statement += `	SELECT trace_id FROM Traces WHERE grouping_id = $1 AND matches_any_ignore_rule = FALSE
),
DataOfInterest AS (
	SELECT ValuesAtHead.trace_id, options_id, digest FROM ValuesAtHead
	JOIN TracesOfInterest ON ValuesAtHead.trace_id = TracesOfInterest.trace_id
	WHERE most_recent_commit_id >= $2
)`
	return statement, nil
}

// getLinks returns the nodes and links that correspond to the digests and how each compares to
// the other digests.
func (s *Impl) getLinks(ctx context.Context, digests map[schema.MD5Hash]*digestClusterInfo) ([]frontend.Node, []frontend.Link, error) {
	ctx, span := trace.StartSpan(ctx, "getDigestsAndTracesForCluster")
	defer span.End()

	var digestsToLookup []schema.DigestBytes
	nodes := make([]frontend.Node, 0, len(digestsToLookup))
	for digest, info := range digests {
		if len(info.traceIDs) > 0 {
			digestsToLookup = append(digestsToLookup, sql.FromMD5Hash(digest))
			nodes = append(nodes, frontend.Node{
				Digest: types.Digest(hex.EncodeToString(digest[:])),
				Status: info.label.ToExpectation(),
			})
		}
	}
	if len(digestsToLookup) == 0 {
		return nil, nil, nil
	}
	// sort the nodes by digest for determinism.
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Digest < nodes[j].Digest
	})
	// Make a map so we can easily go from digest to index as we go through our results.
	digestToIndex := make(map[types.Digest]int, len(digestsToLookup))
	for i, n := range nodes {
		digestToIndex[n.Digest] = i
	}

	span.AddAttributes(trace.Int64Attribute("num_digests", int64(len(digestsToLookup))))
	const statement = `SELECT encode(left_digest, 'hex'), encode(right_digest, 'hex'), percent_pixels_diff
FROM DiffMetrics AS OF SYSTEM TIME '-0.1s'
WHERE left_digest = ANY($1) AND right_digest = ANY($1) AND left_digest < right_digest
ORDER BY 1, 2`
	rows, err := s.db.Query(ctx, statement, digestsToLookup)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var leftDigest types.Digest
	var rightDigest types.Digest
	links := make([]frontend.Link, 0, len(nodes)*(len(nodes)-1)/2)
	for rows.Next() {
		var link frontend.Link
		if err := rows.Scan(&leftDigest, &rightDigest, &link.Distance); err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		link.LeftIndex = digestToIndex[leftDigest]
		link.RightIndex = digestToIndex[rightDigest]
		links = append(links, link)
	}
	return nodes, links, nil
}

// getParamsetsForCluster looks up all the params for the given traces and options and returns
// them grouped by digest and in totality.
func (s *Impl) getParamsetsForCluster(ctx context.Context, digests map[schema.MD5Hash]*digestClusterInfo) (map[types.Digest]paramtools.ParamSet, paramtools.ParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "getParamsetsForCluster")
	defer span.End()
	if len(digests) == 0 {
		return nil, nil, nil
	}
	byDigest := make(map[types.Digest]paramtools.ParamSet, len(digests))
	combined := paramtools.ParamSet{}
	for d, info := range digests {
		digest := types.Digest(hex.EncodeToString(d[:]))
		thisDigestsParamset, ok := byDigest[digest]
		if !ok {
			thisDigestsParamset = paramtools.ParamSet{}
			byDigest[digest] = thisDigestsParamset
		}
		for _, traceID := range info.traceIDs {
			p, err := s.expandTraceToParams(ctx, traceID)
			if err != nil {
				return nil, nil, skerr.Wrap(err)
			}
			thisDigestsParamset.AddParams(p)
			combined.AddParams(p)
		}
		for _, optID := range info.optionsIDs {
			p, err := s.expandOptionsToParams(ctx, optID)
			if err != nil {
				return nil, nil, skerr.Wrap(err)
			}
			thisDigestsParamset.AddParams(p)
			combined.AddParams(p)
		}
	}
	// Normalize the paramsets for determinism
	for _, ps := range byDigest {
		ps.Normalize()
	}
	combined.Normalize()
	return byDigest, combined, nil
}

// GetCommitsInWindow implements the API interface
func (s *Impl) GetCommitsInWindow(ctx context.Context) ([]frontend.Commit, error) {
	ctx, span := trace.StartSpan(ctx, "getCommitsInWindow")
	defer span.End()
	return s.commitsProvider.GetCommitsInWindow(ctx)
}

// GetDigestsForGrouping implements the API interface.
func (s *Impl) GetDigestsForGrouping(ctx context.Context, grouping paramtools.Params) (frontend.DigestListResponse, error) {
	ctx, span := trace.StartSpan(ctx, "search2_GetDigestsForGrouping")
	defer span.End()
	_, groupingID := sql.SerializeMap(grouping)
	const statement = `WITH
RecentCommits AS (
	SELECT tile_id FROM CommitsWithData
	ORDER BY commit_id DESC LIMIT $1
),
FirstTileInWindow AS (
	SELECT tile_id FROM RecentCommits
	ORDER BY tile_id ASC LIMIT 1
)
SELECT DISTINCT encode(digest, 'hex') FROM TiledTraceDigests JOIN
FirstTileInWindow ON TiledTraceDigests.tile_id >= FirstTileInWindow.tile_id AND
  TiledTraceDigests.grouping_id = $2
ORDER BY 1`

	rows, err := s.db.Query(ctx, statement, s.windowLength, groupingID)
	if err != nil {
		return frontend.DigestListResponse{}, skerr.Wrapf(err, "getting digests for grouping %x - %#v", groupingID, grouping)
	}
	defer rows.Close()
	var resp frontend.DigestListResponse
	for rows.Next() {
		var digest types.Digest
		if err := rows.Scan(&digest); err != nil {
			return frontend.DigestListResponse{}, skerr.Wrap(err)
		}
		resp.Digests = append(resp.Digests, digest)
	}
	return resp, nil
}

// GetDigestDetails is very similar to the Search() function, but it only has one digest, so
// there is only one result.
func (s *Impl) GetDigestDetails(ctx context.Context, grouping paramtools.Params, digest types.Digest, clID, crs string) (frontend.DigestDetails, error) {
	ctx, span := trace.StartSpan(ctx, "search2_GetDigestDetails")
	defer span.End()

	ctx, err := s.addCommitsData(ctx)
	if err != nil {
		return frontend.DigestDetails{}, skerr.Wrap(err)
	}
	commits, err := s.commitsProvider.GetCommits(ctx)
	if err != nil {
		return frontend.DigestDetails{}, skerr.Wrap(err)
	}
	// Fill in a few values to allow us to use the same methods as Search.
	ctx = context.WithValue(ctx, common.QueryKey, query.Search{
		CodeReviewSystemID: crs, ChangelistID: clID,
		Offset: 0, Limit: 1, RGBAMaxFilter: 255,
	})

	var digestWithTraceAndGrouping []common.DigestWithTraceAndGrouping
	if clID == "" {
		digestWithTraceAndGrouping, err = s.traceDigestsProvider.GetTracesForGroupingAndDigest(ctx, grouping, digest)
		if err != nil {
			return frontend.DigestDetails{}, skerr.Wrap(err)
		}
	} else {
		if crs == "" {
			return frontend.DigestDetails{}, skerr.Fmt("Code Review System (crs) must be specified")
		}
		ctx = context.WithValue(ctx, common.QualifiedCLIDKey, sql.Qualify(crs, clID))
		digestWithTraceAndGrouping, err = s.traceDigestsProvider.GetTracesFromCLThatProduced(ctx, grouping, digest)
		if err != nil {
			return frontend.DigestDetails{}, skerr.Wrap(err)
		}
		if commits, err = s.addCLCommit(ctx, commits); err != nil {
			return frontend.DigestDetails{}, skerr.Wrap(err)
		}
	}

	if s.isPublicView {
		digestWithTraceAndGrouping = s.applyPublicFilterToDigestWithTraceAndGrouping(digestWithTraceAndGrouping)
	}

	// Lookup the closest diffs to the given digests. This returns a subset according to the
	// limit and offset in the query.
	digestAndClosestDiffs, _, err := s.getClosestDiffs(ctx, digestWithTraceAndGrouping)
	if err != nil {
		return frontend.DigestDetails{}, skerr.Wrap(err)
	}
	if len(digestAndClosestDiffs) == 0 {
		return frontend.DigestDetails{}, skerr.Fmt("No results found")
	}
	// Go fetch history and paramset (within this grouping, and respecting publiclyAllowedParams).
	paramsetsByDigest, err := s.getParamsetsForRightSide(ctx, digestAndClosestDiffs)
	if err != nil {
		return frontend.DigestDetails{}, skerr.Wrap(err)
	}
	// Flesh out the trace history with enough data to draw the dots diagram on the frontend.
	// The returned slice should be length 1 because we had only 1 digest and 1
	// digestAndClosestDiffs.
	resultSlice, err := s.fillOutTraceHistory(ctx, digestAndClosestDiffs)
	if err != nil {
		return frontend.DigestDetails{}, skerr.Wrap(err)
	}

	result := *resultSlice[0]
	// Fill in the paramsets of the reference images.
	for _, srdd := range result.RefDiffs {
		if srdd != nil {
			srdd.ParamSet = paramsetsByDigest[srdd.Digest]
		}
	}

	// Make sure the Test is set, even if the digest wasn't seen in the current window
	// The frontend relies on this field to be able to triage the results.
	result.Test = types.TestName(grouping[types.PrimaryKeyField])
	return frontend.DigestDetails{
		Commits: commits,
		Result:  result,
	}, nil
}

// applyPublicFilterToDigestWithTraceAndGrouping filters out any digestWithTraceAndGrouping for
// traces that are not publicly visible.
func (s *Impl) applyPublicFilterToDigestWithTraceAndGrouping(results []common.DigestWithTraceAndGrouping) []common.DigestWithTraceAndGrouping {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	filteredResults := make([]common.DigestWithTraceAndGrouping, 0, len(results))
	var traceKey schema.MD5Hash
	for _, result := range results {
		copy(traceKey[:], result.TraceID)
		if _, ok := s.publiclyVisibleTraces[traceKey]; ok {
			filteredResults = append(filteredResults, result)
		}
	}
	return filteredResults
}

// GetDigestsDiff implements the API interface.
func (s *Impl) GetDigestsDiff(ctx context.Context, grouping paramtools.Params, left, right types.Digest, clID, crs string) (frontend.DigestComparison, error) {
	ctx, span := trace.StartSpan(ctx, "web_GetDigestsDiff")
	defer span.End()
	_, groupingID := sql.SerializeMap(grouping)
	leftBytes, err := sql.DigestToBytes(left)
	if err != nil {
		return frontend.DigestComparison{}, skerr.Wrap(err)
	}
	rightBytes, err := sql.DigestToBytes(right)
	if err != nil {
		return frontend.DigestComparison{}, skerr.Wrap(err)
	}

	metrics, err := s.getDiffBetween(ctx, leftBytes, rightBytes)
	if err != nil {
		return frontend.DigestComparison{}, skerr.Wrapf(err, "missing diff information for %s-%s", left, right)
	}

	leftLabel, err := s.getExpectationsForDigest(ctx, groupingID, leftBytes, crs, clID)
	if err != nil {
		return frontend.DigestComparison{}, skerr.Wrap(err)
	}
	rightLabel, err := s.getExpectationsForDigest(ctx, groupingID, rightBytes, crs, clID)
	if err != nil {
		return frontend.DigestComparison{}, skerr.Wrap(err)
	}

	leftParamset, err := s.getParamsetsForTracesProducing(ctx, groupingID, leftBytes, crs, clID)
	if err != nil {
		return frontend.DigestComparison{}, skerr.Wrap(err)
	}
	rightParamset, err := s.getParamsetsForTracesProducing(ctx, groupingID, rightBytes, crs, clID)
	if err != nil {
		return frontend.DigestComparison{}, skerr.Wrap(err)
	}

	metrics.Digest = right
	metrics.Status = rightLabel
	metrics.ParamSet = rightParamset
	return frontend.DigestComparison{
		Left: frontend.LeftDiffInfo{
			Test:          types.TestName(grouping[types.PrimaryKeyField]),
			Digest:        left,
			Status:        leftLabel,
			TriageHistory: nil, // TODO(kjlubick)
			ParamSet:      leftParamset,
		},
		Right: metrics,
	}, nil

}

// getDiffBetween returns the diff metrics for the given digests.
func (s *Impl) getDiffBetween(ctx context.Context, left, right schema.DigestBytes) (frontend.SRDiffDigest, error) {
	ctx, span := trace.StartSpan(ctx, "getDiffBetween")
	defer span.End()
	const statement = `SELECT num_pixels_diff, percent_pixels_diff, max_rgba_diffs,
combined_metric, dimensions_differ
FROM DiffMetrics WHERE left_digest = $1 and right_digest = $2 LIMIT 1`
	row := s.db.QueryRow(ctx, statement, left, right)
	var rv frontend.SRDiffDigest
	if err := row.Scan(&rv.NumDiffPixels, &rv.PixelDiffPercent, &rv.MaxRGBADiffs,
		&rv.CombinedMetric, &rv.DimDiffer); err != nil {
		return frontend.SRDiffDigest{}, skerr.Wrap(err)
	}
	return rv, nil
}

// getExpectationsForDigest returns the expectations for the given digest and grouping pair. It
// assumes that the given digest is valid. If the provided digest was triaged on the given CL (and
// the CL is valid), the expectation from that CL takes precedence over the expectation from the
// primary branch. If no expectation is found, we assume that 1) the digest was ingested from a CL,
// 2) the digest was never seen before, and 3) the digest has not yet been triaged; therefore we
// return expectations.Untriaged. It is the caller's responsibility to detect whether the digest is
// valid in this case, if applicable (e.g. by inspecting the SecondaryBranchValues table).
//
// Defaulting to expectations.Untriaged is consistent with the way CL ingestion works: we do not
// populate the SecondaryBranchExpectations table with untriaged entries during ingestion.
func (s *Impl) getExpectationsForDigest(ctx context.Context, groupingID schema.GroupingID, digest schema.DigestBytes, crs, clID string) (expectations.Label, error) {
	ctx, span := trace.StartSpan(ctx, "getExpectationsForDigest")
	defer span.End()

	const statement = `WITH
-- If the CRS and CLID are blank here, this will be empty, but the COALESCE will fix things up.
ExpectationsFromCL AS (
	SELECT label, digest FROM SecondaryBranchExpectations
	WHERE grouping_id = $1
	AND digest = $2
	AND branch_name = $3
	LIMIT 1
),
ExpectationsFromPrimary AS (
	SELECT label, digest FROM Expectations
	WHERE grouping_id = $1
	AND digest = $2
	LIMIT 1
),
Digest AS (
	SELECT $2 AS digest
)
SELECT COALESCE(ExpectationsFromCL.label, ExpectationsFromPrimary.label, 'u') AS label
-- This ensures that the query result is never empty, which is necessary to generate the
-- potentially NULL expectations passed to COALESCE.
FROM Digest
FULL OUTER JOIN ExpectationsFromCL USING (digest)
FULL OUTER JOIN ExpectationsFromPrimary USING (digest)
`

	qCLID := sql.Qualify(crs, clID)
	row := s.db.QueryRow(ctx, statement, groupingID, digest, qCLID)
	var label schema.ExpectationLabel
	if err := row.Scan(&label); err != nil {
		if err == pgx.ErrNoRows {
			return "", skerr.Wrapf(err, "while querying expectations for %x on primary branch or cl %s for grouping %x: query returned 0 rows, which should never happen (this is a bug)", digest, qCLID, groupingID)
		}
		return "", skerr.Wrap(err)
	}
	return label.ToExpectation(), nil
}

// getParamsetsForTracesProducing returns the paramset of the traces on the primary branch which
// produced the digest at the given grouping. If a valid CRS and CLID are provided, the traces
// on that will also be included.
func (s *Impl) getParamsetsForTracesProducing(ctx context.Context, groupingID schema.GroupingID, digest schema.DigestBytes, crs, clID string) (paramtools.ParamSet, error) {
	ctx, span := trace.StartSpan(ctx, "getParamsetsForTracesProducing")
	defer span.End()

	const statement = `WITH
RecentCommits AS (
	SELECT commit_id, tile_id FROM CommitsWithData
	ORDER BY commit_id DESC LIMIT $1
),
OldestTileInWindow AS (
	SELECT tile_id FROM RecentCommits
	ORDER BY commit_id ASC LIMIT 1
),
TracesThatProducedDigestOnPrimary AS (
	SELECT DISTINCT trace_id FROM TiledTraceDigests
	JOIN OldestTileInWindow ON TiledTraceDigests.tile_id >= OldestTileInWindow.tile_id AND
	TiledTraceDigests.grouping_id = $2 AND TiledTraceDigests.digest = $3
),
TracesAndOptionsFromPrimary AS (
	SELECT TracesThatProducedDigestOnPrimary.trace_id, ValuesAtHead.options_id
	FROM TracesThatProducedDigestOnPrimary JOIN ValuesAtHead
		ON TracesThatProducedDigestOnPrimary.trace_id = ValuesAtHead.trace_id
),
-- If the crs and clID are empty or do not exist, this will be the empty set.
TracesAndOptionsThatProducedDigestOnCL AS (
	SELECT DISTINCT secondary_branch_trace_id AS trace_id, options_id FROM SecondaryBranchValues WHERE
	grouping_id = $2 AND digest = $3 AND branch_name = $4
)
SELECT trace_id, options_id FROM TracesAndOptionsFromPrimary
UNION
SELECT trace_id, options_id FROM TracesAndOptionsThatProducedDigestOnCL
`
	rows, err := s.db.Query(ctx, statement, s.windowLength, groupingID, digest, sql.Qualify(crs, clID))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var traces []schema.TraceID
	var opts []schema.OptionsID

	for rows.Next() {
		var trID schema.TraceID
		var optID schema.OptionsID
		if err := rows.Scan(&trID, &optID); err != nil {
			return nil, skerr.Wrap(err)
		}
		// trace ids should be unique due to the query we made
		traces = append(traces, trID)
		// There are generally few options, so a linear search to avoid duplicates is sufficient
		// to avoid a lot of cache lookups.
		existsAlready := false
		for _, o := range opts {
			if bytes.Equal(o, optID) {
				existsAlready = true
				break
			}
		}
		if !existsAlready {
			opts = append(opts, optID)
		}
	}

	paramset, err := s.lookupOrLoadParamSetFromCache(ctx, traces)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	for _, o := range opts {
		ps, err := s.expandOptionsToParams(ctx, o)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		paramset.AddParams(ps)
	}
	paramset.Normalize()
	return paramset, nil
}

// CountDigestsByTest counts only the digests at head that match the given query.
func (s *Impl) CountDigestsByTest(ctx context.Context, q frontend.ListTestsQuery) (frontend.ListTestsResponse, error) {
	ctx, span := trace.StartSpan(ctx, "countDigestsByTest")
	defer span.End()

	statement := `WITH
CommitsInWindow AS (
	SELECT commit_id, tile_id FROM CommitsWithData
	ORDER BY commit_id DESC LIMIT $1
),
OldestCommitInWindow AS (
	SELECT commit_id, tile_id FROM CommitsInWindow
	ORDER BY commit_id ASC LIMIT 1
),`
	digestsStatement, digestsArgs, err := digestCountTracesStatement(q)
	if err != nil {
		return frontend.ListTestsResponse{}, skerr.Wrap(err)
	}
	statement += digestsStatement
	statement += `DigestsWithLabels AS (
	SELECT Groupings.grouping_id, Groupings.keys AS grouping, label, DigestsOfInterest.digest
	FROM DigestsOfInterest
	JOIN Expectations ON DigestsOfInterest.grouping_id = Expectations.grouping_id
	                     AND DigestsOfInterest.digest = Expectations.digest
	JOIN Groupings ON DigestsOfInterest.grouping_id = Groupings.grouping_id
)
`
	selectStmt := `SELECT encode(grouping_id, 'hex'), grouping, label, COUNT(digest) FROM DigestsWithLabels
GROUP BY grouping_id, grouping, label ORDER BY grouping->>'name'`
	if s.dbType == config.Spanner {
		// Spanner does not support grouping based on JSONB fields. Hence we select all the
		// data first and then do the grouping later in memory.
		selectStmt = `SELECT grouping_id, grouping, label, digest FROM DigestsWithLabels`
	}
	statement += selectStmt

	arguments := []interface{}{s.windowLength}
	arguments = append(arguments, digestsArgs...)
	sklog.Infof("query: %#v", q)
	sklog.Infof("statement: %s", statement)
	sklog.Infof("arguments: %s", arguments)
	rows, err := s.db.Query(ctx, statement, arguments...)
	if err != nil {
		return frontend.ListTestsResponse{}, skerr.Wrap(err)
	}
	defer rows.Close()
	var summaries []*frontend.TestSummary
	if s.dbType == config.Spanner {
		summaries, err = s.getTestSummariesFromSpanner(rows)
		if err != nil {
			sklog.Errorf("Error retrieving test summaries from Spanner: %v", err)
			return frontend.ListTestsResponse{}, err
		}
	} else {
		var currentSummary *frontend.TestSummary
		var currentSummaryGroupingID string
		for rows.Next() {
			var groupingID string
			var grouping paramtools.Params
			var label schema.ExpectationLabel
			var count int
			if err := rows.Scan(&groupingID, &grouping, &label, &count); err != nil {
				return frontend.ListTestsResponse{}, skerr.Wrap(err)
			}
			if currentSummary == nil || currentSummaryGroupingID != groupingID {
				currentSummary = &frontend.TestSummary{Grouping: grouping}
				currentSummaryGroupingID = groupingID
				summaries = append(summaries, currentSummary)
			}
			if label == schema.LabelNegative {
				currentSummary.NegativeDigests = count
			} else if label == schema.LabelPositive {
				currentSummary.PositiveDigests = count
			} else {
				currentSummary.UntriagedDigests = count
			}
		}
	}

	withTotals := make([]frontend.TestSummary, 0, len(summaries))
	for _, s := range summaries {
		s.TotalDigests = s.UntriagedDigests + s.PositiveDigests + s.NegativeDigests
		withTotals = append(withTotals, *s)
	}
	return frontend.ListTestsResponse{Tests: withTotals}, nil
}

// getTestSummariesFromSpanner returns a list of test summaries from the rows returned from
// querying against the spanner database.
func (s *Impl) getTestSummariesFromSpanner(rows pgx.Rows) ([]*frontend.TestSummary, error) {
	// We need to get the number of digests per label (untriaged, positive, negative) for
	// each grouping. We do this by creating a map that uses the groupingID as the key and
	// the corresponding summary as the value. The query returns a separate row for each label
	// so we use this map to ensure that the same summary object is updated for the grouping
	// for each label that's corresponding to the groupingID.
	summaryMap := map[string]*frontend.TestSummary{}
	for rows.Next() {
		var grouping paramtools.Params
		var label schema.ExpectationLabel
		var groupingIdBytes []byte
		var digest []byte
		if err := rows.Scan(&groupingIdBytes, &grouping, &label, &digest); err != nil {
			return nil, skerr.Wrap(err)
		}
		groupingID := hex.EncodeToString(groupingIdBytes)

		if _, ok := summaryMap[groupingID]; !ok {
			// If an existing summary is not present, let's create a new one.
			summaryMap[groupingID] = &frontend.TestSummary{Grouping: grouping}
		}

		if label == schema.LabelNegative {
			summaryMap[groupingID].NegativeDigests++
		} else if label == schema.LabelPositive {
			summaryMap[groupingID].PositiveDigests++
		} else {
			summaryMap[groupingID].UntriagedDigests++
		}
	}

	// Now let's extract all the summary objects from the map and return
	// the resulting array.
	summaries := []*frontend.TestSummary{}
	for _, summary := range summaryMap {
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// digestCountTracesStatement returns a statement and arguments that will return all tests,
// digests and their grouping ids. The results will be in a table called DigestsWithLabels.
func digestCountTracesStatement(q frontend.ListTestsQuery) (string, []interface{}, error) {
	arguments := []interface{}{q.Corpus}
	statement := `DigestsOfInterest AS (
	SELECT DISTINCT keys->>'name' AS test_name, digest, grouping_id FROM ValuesAtHead
	JOIN OldestCommitInWindow ON ValuesAtHead.most_recent_commit_id >= OldestCommitInWindow.commit_id
	WHERE corpus = $2`
	if q.IgnoreState == types.ExcludeIgnoredTraces {
		statement += ` AND matches_any_ignore_rule = FALSE`
	}
	if len(q.TraceValues) > 0 {
		jObj := map[string]string{}
		for key, values := range q.TraceValues {
			if len(values) != 1 {
				return "", nil, skerr.Fmt("not implemented: we only support one value per key")
			}
			jObj[key] = values[0]
		}
		statement += ` AND keys @> $3`
		arguments = append(arguments, jObj)
	}
	statement += "),"
	return statement, arguments, nil
}

// ComputeGUIStatus implements the API interface. It has special logic for public views vs the
// normal views to avoid leaking.
func (s *Impl) ComputeGUIStatus(ctx context.Context) (frontend.GUIStatus, error) {
	ctx, span := trace.StartSpan(ctx, "search2_ComputeGUIStatus")
	defer span.End()

	commit, xcs, err := s.statusProvider.GetStatusForAllCorpora(ctx, s.isPublicView)
	if err != nil {
		return frontend.GUIStatus{}, err
	}
	return frontend.GUIStatus{
		LastCommit: commit,
		CorpStatus: xcs,
	}, nil
}

type digestCountAndLastSeen struct {
	digest types.Digest
	// count is how many times a digest has been seen in a TraceGroup.
	count int
	// lastSeenIndex refers to the commit index that this digest was most recently seen. That is,
	// a higher number means it was seen more recently. This digest might have seen much much earlier
	// than this index, but only the latest occurrence affects this value.
	lastSeenIndex int
}

const (
	// maxDistinctDigestsToPresent is the maximum number of digests we want to show
	// in a dotted line of traces. We assume that showing more digests yields
	// no additional information, because the trace is likely to be flaky.
	maxDistinctDigestsToPresent = 9

	// 0 is always the primary digest, no matter where (or if) it appears in the trace.
	primaryDigestIndex = 0

	// The frontend knows to handle -1 specially and show no dot.
	missingDigestIndex = -1

	mostRecentNDigests = 3
)

// ComputeDigestIndices assigns distinct digests an index ( up to MaxDistinctDigestsToPresent).
// This index
// maps to a color of dot on the frontend when representing traces. The indices are assigned to
// some of the most recent digests and some of the most common digests. All digests not in this
// map will be grouped under the highest index (represented by a grey color on the frontend).
// This hybrid approach was adapted in an effort to minimize the "interesting" digests that are
// globbed together under the grey color, which is harder to inspect from the frontend.
// See skbug.com/10387 for more context.
func computeDigestIndices(traceGroup *frontend.TraceGroup, primary types.Digest) (map[types.Digest]int, int) {
	// digestStats is a slice that has one entry per unique digest. This could be a map, but
	// we are going to sort it later, so it's cleaner to just use a slice initially especially
	// when the vast vast majority (99.9% of Skia's data) of our traces have fewer than 30 unique
	// digests. The worst case would be a few hundred unique digests, for which Ω(n) lookup isn't
	// terrible.
	digestStats := make([]digestCountAndLastSeen, 0, 5)
	// Populate digestStats, iterating over the digests from all traces from oldest to newest.
	// By construction, all traces in the TraceGroup will have the same length.
	traceLength := len(traceGroup.Traces[0].RawTrace.Digests)
	sawPrimary := false
	for idx := 0; idx < traceLength; idx++ {
		for _, tr := range traceGroup.Traces {
			digest := tr.RawTrace.Digests[idx]
			// Don't bother counting up data for missing digests.
			if digest == tiling.MissingDigest {
				continue
			}
			if digest == primary {
				sawPrimary = true
			}
			// Go look up the entry for this digest. The sentinel value -1 will tell us if we haven't
			// seen one and need to add one.
			dsIdxToUpdate := -1
			for i, ds := range digestStats {
				if ds.digest == digest {
					dsIdxToUpdate = i
					break
				}
			}
			if dsIdxToUpdate == -1 {
				dsIdxToUpdate = len(digestStats)
				digestStats = append(digestStats, digestCountAndLastSeen{
					digest: digest,
				})
			}
			digestStats[dsIdxToUpdate].count++
			digestStats[dsIdxToUpdate].lastSeenIndex = idx
		}
	}

	// Sort in order of highest last seen index, with tiebreaks being higher count and then
	// lexicographically by digest.
	sort.Slice(digestStats, func(i, j int) bool {
		statsA, statsB := digestStats[i], digestStats[j]
		if statsA.lastSeenIndex != statsB.lastSeenIndex {
			return statsA.lastSeenIndex > statsB.lastSeenIndex
		}
		if statsA.count != statsB.count {
			return statsA.count > statsB.count
		}
		return statsA.digest < statsB.digest
	})

	// Assign the primary digest the primaryDigestIndex.
	digestIndices := make(map[types.Digest]int, maxDistinctDigestsToPresent)
	digestIndices[primary] = primaryDigestIndex
	// Go through the slice until we have either added the n most recent digests or have run out
	// of unique digests. We are careful not to add a digest we've already added (e.g. the primary
	// digest). We start with the most recent digests to preserve a little bit of backwards
	// compatibility with the assigned colors (e.g. developers are used to green and orange being the
	// more recent digests).
	digestIndex := 1
	for i := 0; i < len(digestStats) && len(digestIndices) < 1+mostRecentNDigests; i++ {
		ds := digestStats[i]
		if _, ok := digestIndices[ds.digest]; ok {
			continue
		}
		digestIndices[ds.digest] = digestIndex
		digestIndex++
	}

	// Re-sort the slice in order of highest count, with tiebreaks being a higher last seen index
	// and then lexicographically by digest.
	sort.Slice(digestStats, func(i, j int) bool {
		statsA, statsB := digestStats[i], digestStats[j]
		if statsA.count != statsB.count {
			return statsA.count > statsB.count
		}
		if statsA.lastSeenIndex != statsB.lastSeenIndex {
			return statsA.lastSeenIndex > statsB.lastSeenIndex
		}
		return statsA.digest < statsB.digest
	})

	// Assign the rest of the indices in order of most common digests.
	for i := 0; i < len(digestStats) && len(digestIndices) < maxDistinctDigestsToPresent; i++ {
		ds := digestStats[i]
		if _, ok := digestIndices[ds.digest]; ok {
			continue
		}
		digestIndices[ds.digest] = digestIndex
		digestIndex++
	}
	totalDigests := len(digestStats)
	if !sawPrimary {
		totalDigests++
	}
	return digestIndices, totalDigests
}

// Make sure Impl implements the API interface.
var _ API = (*Impl)(nil)
