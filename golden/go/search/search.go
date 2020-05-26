// Package search contains the core functionality for searching for digests across a sliding window
// of the last N commits. N is set per instance, but is typically between 100 and 500 commits.
package search

import (
	"context"
	"sort"
	"sync"
	"time"

	ttlcache "github.com/patrickmn/go-cache"
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/comment"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/search/ref_differ"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/types"
	web_frontend "go.skia.org/infra/golden/go/web/frontend"
)

const (
	// maxDistinctDigestsToPresent is the maximum number of digests we want to show
	// in a dotted line of traces. We assume that showing more digests yields
	// no additional information, because the trace is likely to be flaky.
	maxDistinctDigestsToPresent = 9

	// TODO(kjlubick): no tests for this option yet.
	GROUP_TEST_MAX_COUNT = "count"

	// These params configure how long we should hold values in storeCache.
	// They are arbitrarily defined, loosely based on the idea that data flowing
	// into the store for a given CL does not change at all after ingestion is complete
	// and even during ingestion, things are likely to not change much over a time period
	// less than one minute.
	searchCacheFreshness = 1 * time.Minute
	searchCacheCleanup   = 5 * time.Minute
)

// SearchImpl holds onto various objects needed to search the latest
// tile for digests. It implements the SearchAPI interface.
type SearchImpl struct {
	diffStore         diff.DiffStore
	expectationsStore expectations.Store
	indexSource       indexer.IndexSource
	changeListStore   clstore.Store
	tryJobStore       tjstore.Store
	commentStore      comment.Store

	// storeCache allows for better performance by caching values from changeListStore and
	// tryJobStore for a little while, before evicting them.
	// See skbug.com/9476
	storeCache *ttlcache.Cache

	// triageHistoryCache maps expectation.ID to frontend.TriageHistory. Entries get removed if
	// we see an event indicating expectations for that ID changed.
	triageHistoryCache *sync.Map

	// optional. If specified, will only show the params that match this query. This is
	// opt-in, to avoid leaking.
	publiclyViewableParams paramtools.ParamSet
}

// New returns a new SearchImpl instance.
func New(ds diff.DiffStore, es expectations.Store, cer expectations.ChangeEventRegisterer, is indexer.IndexSource, cls clstore.Store, tjs tjstore.Store, cs comment.Store, publiclyViewableParams paramtools.ParamSet) *SearchImpl {
	var triageHistoryCache sync.Map
	if cer != nil {
		// If the expectations change for a given ID, we should purge it from our cache so as not
		// to serve stale data.
		cer.ListenForChange(func(id expectations.ID) {
			triageHistoryCache.Delete(id)
		})
	}

	return &SearchImpl{
		diffStore:              ds,
		expectationsStore:      es,
		indexSource:            is,
		changeListStore:        cls,
		tryJobStore:            tjs,
		commentStore:           cs,
		publiclyViewableParams: publiclyViewableParams,

		storeCache:         ttlcache.New(searchCacheFreshness, searchCacheCleanup),
		triageHistoryCache: &triageHistoryCache,
	}
}

// Search implements the SearchAPI interface.
func (s *SearchImpl) Search(ctx context.Context, q *query.Search) (*frontend.SearchResponse, error) {
	defer metrics2.FuncTimer().Stop()
	if q == nil {
		return nil, skerr.Fmt("nil query")
	}

	// Keep track if we are including reference diffs. This is going to be true
	// for the majority of queries. TODO(kjlubick) Who uses this? Do we have tests? Can it go?
	getRefDiffs := !q.NoDiff
	// TODO(kjlubick) remove the legacy check against "0" once the frontend is updated
	//   not to pass it.
	isChangeListSearch := q.ChangeListID != "" && q.ChangeListID != "0"
	// Get the expectations and the current index, which we assume constant
	// for the duration of this query.
	crs := ""
	if s.changeListStore != nil {
		crs = s.changeListStore.System()
	}
	exp, err := s.getExpectations(ctx, q.ChangeListID, crs)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	idx := s.indexSource.GetIndex()

	var results []*frontend.SearchResult
	// Find the digests (left hand side) we are interested in.
	if isChangeListSearch {
		results, err = s.queryChangeList(ctx, q, idx, exp)
		if err != nil {
			return nil, skerr.Wrapf(err, "getting digests from new clstore/tjstore")
		}
	} else {
		// Iterate through the tile and find the digests that match the queries.
		results, err = s.filterTile(ctx, q, idx, exp)
		if err != nil {
			return nil, skerr.Wrapf(err, "getting digests from master tile")
		}
	}
	// At this first point, results will only be partially filled out. The next steps will fill
	// in the remaining pieces.

	// Add the expectation values to the results.
	addExpectations(results, exp)

	// Get reference diffs unless it was specifically disabled.
	if getRefDiffs {
		// Diff stage: Compare all digests found in the previous stages and find
		// reference points (positive, negative etc.) for each digest.
		if err := s.getReferenceDiffs(ctx, results, q.Metric, q.Match, q.RightTraceValues, q.IgnoreState(), exp, idx); err != nil {
			return nil, skerr.Wrapf(err, "fetching reference diffs for %#v", q)
		}

		// Post-diff stage: Apply all filters that are relevant once we have
		// diff values for the digests.
		results = s.afterDiffResultFilter(ctx, results, q)
	}

	// Sort the digests and fill the ones that are going to be displayed with
	// additional data.
	displayRet, offset := s.sortAndLimitDigests(ctx, q, results, int(q.Offset), int(q.Limit))
	s.addTriageHistory(ctx, s.makeTriageHistoryGetter(crs, q.ChangeListID), displayRet)
	traceComments := s.addTracesAndComments(ctx, displayRet, exp, idx)

	// Return all digests with the selected offset within the result set.
	searchRet := &frontend.SearchResponse{
		Results:       displayRet,
		Offset:        offset,
		Size:          len(results),
		Commits:       web_frontend.FromTilingCommits(idx.Tile().GetTile(types.ExcludeIgnoredTraces).Commits),
		TraceComments: traceComments,
	}
	return searchRet, nil
}

// GetDigestDetails implements the SearchAPI interface.
func (s *SearchImpl) GetDigestDetails(ctx context.Context, test types.TestName, digest types.Digest, clID, crs string) (*frontend.DigestDetails, error) {
	defer metrics2.FuncTimer().Stop()
	idx := s.indexSource.GetIndex()

	// Make sure we have valid data, i.e. we know about that test/digest
	dct := idx.DigestCountsByTest(types.IncludeIgnoredTraces)

	digests, ok := dct[test]
	if !ok {
		if clID != "" {
			clIdx := s.indexSource.GetIndexForCL(crs, clID)
			if clIdx == nil || !util.In(string(test), clIdx.ParamSet[types.PrimaryKeyField]) {
				return nil, skerr.Fmt("unknown test %s for cl %s", test, clID)
			}
			return s.getCLOnlyDigestDetails(ctx, test, digest, clID, crs)
		}
		return nil, skerr.Fmt("unknown test %s", test)
	}

	tile := idx.Tile().GetTile(types.IncludeIgnoredTraces)

	exp, err := s.getExpectations(ctx, clID, crs)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	result := frontend.SearchResult{
		Test:     test,
		Digest:   digest,
		ParamSet: paramtools.ParamSet{},
	}

	if _, ok := digests[digest]; ok {
		// We know a digest is somewhere in at least one trace. Iterate through all of them
		// to find which ones.
		byTrace := idx.DigestCountsByTrace(types.IncludeIgnoredTraces)
		for traceID, trace := range tile.Traces {
			if trace.TestName() != test {
				continue
			}
			if _, ok := byTrace[traceID][digest]; ok {
				result.ParamSet.AddParams(trace.Params())
				result.TraceGroup.Traces = append(result.TraceGroup.Traces, frontend.Trace{
					ID:       traceID,
					RawTrace: trace,
					Params:   trace.Params(),
				})
			}
		}
	}
	// Normalizing the ParamSet makes the return values deterministic.
	result.ParamSet.Normalize()

	// We wrap the result in a slice so we can re-use the search functions.
	results := []*frontend.SearchResult{&result}
	addExpectations(results, exp)
	err = s.getReferenceDiffs(ctx, results, diff.CombinedMetric, []string{types.PrimaryKeyField}, nil, types.ExcludeIgnoredTraces, exp, idx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Fetching reference diffs for test %s, digest %s", test, digest)
	}

	var traceComments []frontend.TraceComment
	if len(result.TraceGroup.Traces) > 0 {
		// Get the params and traces.
		traceComments = s.addTracesAndComments(ctx, results, exp, idx)
	}
	s.addTriageHistory(ctx, s.makeTriageHistoryGetter(crs, clID), results)

	return &frontend.DigestDetails{
		Result:        result,
		Commits:       web_frontend.FromTilingCommits(tile.Commits),
		TraceComments: traceComments,
	}, nil
}

// getExpectations returns a slice of expectations that should be
// used in the given query. It will add the issue expectations if this is
// querying ChangeList results. If query is nil the expectations of the master
// tile are returned.
func (s *SearchImpl) getExpectations(ctx context.Context, clID, crs string) (expectations.Classifier, error) {
	exp, err := s.expectationsStore.Get(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "loading expectations for master")
	}
	// TODO(kjlubick) remove the legacy value "0" once frontend changes have baked in.
	if clID != "" && clID != "0" {
		issueExpStore := s.expectationsStore.ForChangeList(clID, crs)
		tjExp, err := issueExpStore.Get(ctx)
		if err != nil {
			return nil, skerr.Wrapf(err, "loading expectations for cl %s (%s)", clID, crs)
		}
		return expectations.Join(tjExp, exp), nil
	}

	return exp, nil
}

// getCLOnlyDigestDetails returns details for a digest when it is newly added to a CL (and does
// not exist on the master branch). This is handled as its own special case because the existing
// master branch index, which normally aids in filling out these details (e.g. has a map from
// digest to traces) does not help us here and we must re-scan the list of tryjob results
// ourselves.
func (s *SearchImpl) getCLOnlyDigestDetails(ctx context.Context, test types.TestName, digest types.Digest, clID, crs string) (*frontend.DigestDetails, error) {
	exp, err := s.getExpectations(ctx, clID, crs)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// We know xps is sorted by order, if it is non-nil.
	xps, err := s.getPatchSets(ctx, clID)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting PatchSets for CL %s", clID)
	}
	if len(xps) == 0 {
		return nil, skerr.Fmt("No data for CL %s", clID)
	}

	latestPatchSet := xps[len(xps)-1]
	id := tjstore.CombinedPSID{
		CL:  latestPatchSet.ChangeListID,
		CRS: crs,
		PS:  latestPatchSet.SystemID,
	}
	xtr, err := s.getTryJobResults(ctx, id)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting tryjob results for %v", id)
	}
	paramSet := paramtools.ParamSet{}
	for _, tr := range xtr { // this could be done in parallel, if needed for performance reasons.
		if tr.Digest != digest {
			continue
		}
		if tr.ResultParams[types.PrimaryKeyField] != string(test) {
			continue
		}
		p := paramtools.Params{}
		p.Add(tr.GroupParams, tr.Options, tr.ResultParams)
		// If we've been given a set of PubliclyViewableParams, only show those.
		if len(s.publiclyViewableParams) > 0 {
			if !s.publiclyViewableParams.MatchesParams(p) {
				continue
			}
		}
		paramSet.AddParams(p)
	}

	return &frontend.DigestDetails{
		TraceComments: nil, // TODO(skbug.com/6630)
		Result: frontend.SearchResult{
			Test:          test,
			Digest:        digest,
			Status:        exp.Classification(test, digest).String(),
			TriageHistory: nil, // TODO(skbug.com/10097)
			ParamSet:      paramSet,
			// The trace-related fields can be omitted because there are no traces on master branch of
			// which to show the history
		},
	}, nil
}

// queryChangeList returns the digests associated with the ChangeList referenced by q.CRSAndCLID
// in intermediate representation. It returns the filtered digests as specified by q. The param
// exp should contain the expectations for the given ChangeList.
func (s *SearchImpl) queryChangeList(ctx context.Context, q *query.Search, idx indexer.IndexSearcher, exp expectations.Classifier) ([]*frontend.SearchResult, error) {
	// Build the intermediate map to group results belonging to the same test and digest.
	resultsByGroupingAndDigest := map[groupingAndDigest]*frontend.SearchResult{}
	talliesByTest := idx.DigestCountsByTest(q.IgnoreState())

	addByGroupAndDigest := func(test types.TestName, digest types.Digest, params paramtools.Params) {
		if !q.IncludeDigestsProducedOnMaster {
			if _, ok := talliesByTest[test][digest]; ok {
				return // skip this entry because it was already seen on master branch.
			}
		}
		// TODO(kjlubick) If we want to include trace data in the CLs, we'll have to find a way to
		//   derive the trace id here. This is a bit tricky because params includes optional keys.
		key := groupingAndDigest{grouping: test, digest: digest}
		existing := resultsByGroupingAndDigest[key]
		if existing == nil {
			existing = &frontend.SearchResult{
				Test:     test,
				Digest:   digest,
				ParamSet: paramtools.ParamSet{},
			}
			resultsByGroupingAndDigest[key] = existing
		}
		existing.ParamSet.AddParams(params)
	}

	err := s.extractChangeListDigests(ctx, q, idx, exp, addByGroupAndDigest)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	ret := make([]*frontend.SearchResult, 0, len(resultsByGroupingAndDigest))
	for _, srd := range resultsByGroupingAndDigest {
		// Normalizing the ParamSet makes the return values deterministic.
		srd.ParamSet.Normalize()
		ret = append(ret, srd)
	}
	return ret, nil
}

// filterAddFn is a filter and add function that is passed to the getIssueDigest interface. It will
// be called for each testName/digest combination and should accumulate the digests of interest.
type filterAddFn func(test types.TestName, digest types.Digest, params paramtools.Params)

// extractFilterShards dictates how to break up the filtering of extractChangeListDigests after
// they have been fetched from the TryJobStore. It was determined experimentally on
// BenchmarkExtractChangeListDigests. It sped up things by about a factor of 6 and was a good
// balance of dividing up and mutex contention.
const extractFilterShards = 16

// extractChangeListDigests loads the ChangeList referenced by q.CRSAndCLID and the TryJobResults
// associated with it. Then, it filters those results with the given query. For each
// testName/digest pair that matches the query, it calls addFn (which the supplier will likely use
// to build up a list of those results.
func (s *SearchImpl) extractChangeListDigests(ctx context.Context, q *query.Search, idx indexer.IndexSearcher, exp expectations.Classifier, addFn filterAddFn) error {
	clID := q.ChangeListID
	// We know xps is sorted by order, if it is non-nil.
	xps, err := s.getPatchSets(ctx, clID)
	if err != nil {
		return skerr.Wrapf(err, "getting PatchSets for CL %s", clID)
	}

	if len(xps) == 0 {
		return skerr.Fmt("No data for CL %s", clID)
	}

	// Default to the latest PatchSet
	ps := xps[len(xps)-1]
	if len(q.PatchSets) > 0 {
		// legacy code used to request multiple patchsets at once - we don't do that
		// so we just look at the first one mentioned by the query.
		psOrder := int(q.PatchSets[0])
		found := false
		for _, p := range xps {
			if p.Order == psOrder {
				ps = p
				found = true
				break
			}
		}
		if !found {
			return skerr.Fmt("Could not find PS with order %d in CL %s", psOrder, clID)
		}
	}

	id := tjstore.CombinedPSID{
		CL:  ps.ChangeListID,
		CRS: s.changeListStore.System(),
		PS:  ps.SystemID,
	}

	var xtr []tjstore.TryJobResult
	wasCached := false
	if q.IncludeUntriagedDigests && !q.IncludePositiveDigests && !q.IncludeNegativeDigests {
		// If the search is just for untriaged digests, we can use the CL index for this.
		clIdx := s.indexSource.GetIndexForCL(id.CRS, id.CL)
		if clIdx != nil && clIdx.LatestPatchSet.Equal(id) {
			xtr = clIdx.UntriagedResults
			wasCached = true
		}
	}
	if !wasCached {
		xtr, err = s.getTryJobResults(ctx, id)
		if err != nil {
			return skerr.Wrapf(err, "getting tryjob results for %v", id)
		}
	} else {
		sklog.Debugf("Cache hit for untriaged tryjob results")
	}

	addMutex := sync.Mutex{}
	chunkSize := len(xtr) / extractFilterShards
	// Very small shards are likely not worth the overhead.
	if chunkSize < 50 {
		chunkSize = 50
	}
	queryParams := q.TraceValues
	ignoreMatcher := idx.GetIgnoreMatcher()

	return util.ChunkIterParallel(ctx, len(xtr), chunkSize, func(ctx context.Context, start, stop int) error {
		sliced := xtr[start:stop]
		for _, tr := range sliced {
			if err := ctx.Err(); err != nil {
				return skerr.Wrap(err)
			}
			tn := types.TestName(tr.ResultParams[types.PrimaryKeyField])
			// Filter by classification.
			c := exp.Classification(tn, tr.Digest)
			if q.ExcludesClassification(c) {
				continue
			}
			p := make(paramtools.Params, len(tr.ResultParams)+len(tr.GroupParams)+len(tr.Options))
			p.Add(tr.GroupParams)
			p.Add(tr.Options)
			p.Add(tr.ResultParams)
			// Filter the ignored results
			if !q.IncludeIgnoredTraces {
				// Because ignores can happen on a mix of params from Result, Group, and Options,
				// we have to invoke the matcher the whole set of params.
				if ignoreMatcher.MatchAnyParams(p) {
					continue
				}
			}
			// If we've been given a set of PubliclyViewableParams, only show those.
			if len(s.publiclyViewableParams) > 0 {
				if !s.publiclyViewableParams.MatchesParams(p) {
					continue
				}
			}
			// Filter by query.
			if queryParams.MatchesParams(p) {
				func() {
					addMutex.Lock()
					addFn(tn, tr.Digest, p)
					addMutex.Unlock()
				}()
			}
		}
		return nil
	})
}

// getPatchSets returns the PatchSets for a given CL either from the store or from the cache.
func (s *SearchImpl) getPatchSets(ctx context.Context, id string) ([]code_review.PatchSet, error) {
	key := "patchsets_" + id
	if xtr, ok := s.storeCache.Get(key); ok {
		return xtr.([]code_review.PatchSet), nil
	}
	xps, err := s.changeListStore.GetPatchSets(ctx, id)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	s.storeCache.SetDefault(key, xps)
	return xps, nil
}

// getTryJobResults returns the TryJobResults for a given CL either from the store or
// from the cache.
func (s *SearchImpl) getTryJobResults(ctx context.Context, id tjstore.CombinedPSID) ([]tjstore.TryJobResult, error) {
	key := "tjresults_" + id.Key()
	if xtr, ok := s.storeCache.Get(key); ok {
		return xtr.([]tjstore.TryJobResult), nil
	}
	xtr, err := s.tryJobStore.GetResults(ctx, id)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	s.storeCache.SetDefault(key, xtr)
	return xtr, nil
}

// DiffDigests implements the SearchAPI interface.
func (s *SearchImpl) DiffDigests(ctx context.Context, test types.TestName, left, right types.Digest, clID string, crs string) (*frontend.DigestComparison, error) {
	defer metrics2.FuncTimer().Stop()
	// Get the diff between the two digests
	diffResult, err := s.diffStore.Get(ctx, left, types.DigestSlice{right})
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Return an error if we could not find the diff.
	if len(diffResult) != 1 {
		return nil, skerr.Fmt("could not find diff between %s and %s", left, right)
	}

	exp, err := s.getExpectations(ctx, clID, crs)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	idx := s.indexSource.GetIndex()

	psLeft := idx.GetParamsetSummary(test, left, types.IncludeIgnoredTraces)
	// Normalizing the ParamSet makes the return values deterministic.
	psLeft.Normalize()
	psRight := idx.GetParamsetSummary(test, right, types.IncludeIgnoredTraces)
	psRight.Normalize()

	history := s.makeTriageHistoryGetter(crs, clID)

	return &frontend.DigestComparison{
		Left: frontend.SearchResult{
			Test:          test,
			Digest:        left,
			Status:        exp.Classification(test, left).String(),
			TriageHistory: s.getTriageHistory(ctx, history, test, left),
			ParamSet:      psLeft,
		},
		Right: &frontend.SRDiffDigest{
			Digest:      right,
			Status:      exp.Classification(test, right).String(),
			ParamSet:    psRight,
			DiffMetrics: diffResult[right],
		},
	}, nil
}

// filterTile iterates over the tile and accumulates the traces
// that match the given query creating the initial search result.
func (s *SearchImpl) filterTile(ctx context.Context, q *query.Search, idx indexer.IndexSearcher, exp expectations.Classifier) ([]*frontend.SearchResult, error) {
	var acceptFn iterTileAcceptFn
	if q.GroupTestFilter == GROUP_TEST_MAX_COUNT {
		maxDigestsByTest := idx.MaxDigestsByTest(q.IgnoreState())
		acceptFn = func(params paramtools.Params, digests types.DigestSlice) bool {
			testName := types.TestName(params[types.PrimaryKeyField])
			for _, d := range digests {
				if maxDigestsByTest[testName][d] {
					return true
				}
			}
			return false
		}
	}

	// We'll want to find all traces that generate a given digest for a given grouping.
	resultsByGroupingAndDigest := map[groupingAndDigest]*frontend.SearchResult{}
	mutex := sync.Mutex{}
	// For each trace that does, we'll add the params the trace has to the paramset of associated
	// with the digest and include the trace in slice of traces.
	addFn := func(test types.TestName, digest types.Digest, traceID tiling.TraceID, trace *tiling.Trace) {
		mutex.Lock()
		defer mutex.Unlock()
		key := groupingAndDigest{grouping: test, digest: digest}
		existing := resultsByGroupingAndDigest[key]
		if existing == nil {
			existing = &frontend.SearchResult{
				Test:     test,
				Digest:   digest,
				ParamSet: paramtools.ParamSet{},
			}
			resultsByGroupingAndDigest[key] = existing
		}
		existing.ParamSet.AddParams(trace.Params())
		// It is tempting to think we could just convert the RawTrace into the frontend.Trace right
		// here, but in fact we need all the traces for a given digest (i.e. in a given TraceGroup)
		// to be able to do that. Specifically, we want to be able to share the digest indices.
		existing.TraceGroup.Traces = append(existing.TraceGroup.Traces, frontend.Trace{
			ID:       traceID,
			RawTrace: trace,
			Params:   trace.Params(),
		})
	}

	if err := iterTile(ctx, q, addFn, acceptFn, exp, idx); err != nil {
		return nil, skerr.Wrap(err)
	}

	results := make([]*frontend.SearchResult, 0, len(resultsByGroupingAndDigest))
	for _, srd := range resultsByGroupingAndDigest {
		// Normalizing the ParamSet makes the return values deterministic.
		srd.ParamSet.Normalize()
		results = append(results, srd)
	}

	return results, nil
}

type groupingAndDigest struct {
	grouping types.TestName
	digest   types.Digest
}

// addExpectations adds the expectations to the current set of results using the provided
// Classifier. TODO(kjlubick) this can be moved into filterTile/etc
func addExpectations(results []*frontend.SearchResult, exp expectations.Classifier) {
	for _, r := range results {
		r.Status = exp.Classification(r.Test, r.Digest).String()
	}
}

// getReferenceDiffs compares all digests collected in the intermediate representation
// and compares them to the other known results for the test at hand.
func (s *SearchImpl) getReferenceDiffs(ctx context.Context, resultDigests []*frontend.SearchResult, metric string, match []string, rhsQuery paramtools.ParamSet, is types.IgnoreState, exp expectations.Classifier, idx indexer.IndexSearcher) error {
	defer shared.NewMetricsTimer("getReferenceDiffs").Stop()
	refDiffer := ref_differ.New(exp, s.diffStore, idx)
	errGroup, gCtx := errgroup.WithContext(ctx)
	sklog.Infof("Going to spawn %d goroutines to get reference diffs", len(resultDigests))
	for _, retDigest := range resultDigests {
		func(d *frontend.SearchResult) {
			errGroup.Go(func() error {
				defer metrics2.NewTimer("gold_find_closest_digests").Stop()
				err := refDiffer.FillRefDiffs(gCtx, d, metric, match, rhsQuery, is)
				if err != nil {
					sklog.Warningf("Error while computing ref diffs: %s", err)
					return nil
				}

				// TODO(kjlubick): if we decide we want the TriageHistory on the right hand side
				//   digests, we could add it here.
				return nil
			})
		}(retDigest)
	}
	return skerr.Wrap(errGroup.Wait())
}

// afterDiffResultFilter filters the results based on the diff results in 'digestInfo'.
func (s *SearchImpl) afterDiffResultFilter(ctx context.Context, digestInfo []*frontend.SearchResult, q *query.Search) []*frontend.SearchResult {
	newDigestInfo := make([]*frontend.SearchResult, 0, len(digestInfo))
	filterRGBADiff := (q.RGBAMinFilter > 0) || (q.RGBAMaxFilter < 255)
	filterDiffMax := q.DiffMaxFilter >= 0
	for _, digest := range digestInfo {
		ref, ok := digest.RefDiffs[digest.ClosestRef]

		// Filter all digests where MaxRGBA is within the given band.
		if filterRGBADiff {
			// If there is no diff metric we exclude the digest.
			if !ok {
				continue
			}

			rgbaMaxDiff := int32(util.MaxInt(ref.MaxRGBADiffs[:]...))
			if (rgbaMaxDiff < q.RGBAMinFilter) || (rgbaMaxDiff > q.RGBAMaxFilter) {
				continue
			}
		}

		// Filter all digests where the diff is below the given threshold.
		if filterDiffMax && (!ok || (ref.Diffs[q.Metric] > q.DiffMaxFilter)) {
			continue
		}

		// If selected only consider digests that have a reference to compare to.
		if q.MustIncludeReferenceFilter && !ok {
			continue
		}

		newDigestInfo = append(newDigestInfo, digest)
	}
	return newDigestInfo
}

// sortAndLimitDigests sorts the digests based on the settings in the Query
// instance. It then paginates the digests according to the query and returns
// the slice that should be shown on the page with its offset in the entire
// result set.
func (s *SearchImpl) sortAndLimitDigests(ctx context.Context, q *query.Search, digestInfo []*frontend.SearchResult, offset, limit int) ([]*frontend.SearchResult, int) {
	fullLength := len(digestInfo)
	if offset >= fullLength {
		return []*frontend.SearchResult{}, 0
	}

	sortSlice := sort.Interface(newSRDigestSlice(q.Metric, digestInfo))
	if q.Sort == query.SortDescending {
		sortSlice = sort.Reverse(sortSlice)
	}
	sort.Sort(sortSlice)

	// Fill in the extra information for the traces we are interested in.
	if limit <= 0 {
		limit = fullLength
	}
	end := util.MinInt(fullLength, offset+limit)
	return digestInfo[offset:end], offset
}

// addTracesAndComments adds information to the given result that is necessary
// to draw them, e.g. the information what digest/image appears at what commit.
// This should be only done for digests that are intended to be displayed.
// TODO(kjlubick) This function should be broken into two parts.
func (s *SearchImpl) addTracesAndComments(ctx context.Context, searchResults []*frontend.SearchResult, exp expectations.Classifier, idx indexer.IndexSearcher) []frontend.TraceComment {
	tile := idx.Tile().GetTile(types.ExcludeIgnoredTraces)
	last := tile.LastCommitIndex()
	var traceComments []frontend.TraceComment
	// TODO(kjlubick) remove this check once the commentStore is implemented and included from main.
	if s.commentStore != nil {
		xtc, err := s.commentStore.ListComments(ctx)
		if err != nil {
			sklog.Warningf("Omitting comments due to error: %s", err)
			traceComments = nil
		} else {
			for _, tc := range xtc {
				traceComments = append(traceComments, frontend.ToTraceComment(tc))
			}
			sort.Slice(traceComments, func(i, j int) bool {
				return traceComments[i].UpdatedTS.Before(traceComments[j].UpdatedTS)
			})
		}
	}

	for _, di := range searchResults {
		// Add the drawable traces to the result.
		s.fillInFrontEndTraceData(di.Test, di.Digest, last, exp, &di.TraceGroup, traceComments)
		di.TraceGroup.TileSize = len(tile.Commits)
	}
	return traceComments
}

const missingDigestIndex = -1

// fillInFrontEndTraceData fills in the data needed to draw the traces for the given test/digest
// and to connect the traces to the appropriate comments.
func (s *SearchImpl) fillInFrontEndTraceData(test types.TestName, digest types.Digest, last int, exp expectations.Classifier, traceGroup *frontend.TraceGroup, comments []frontend.TraceComment) {
	// Put the traces in a deterministic order
	sort.Slice(traceGroup.Traces, func(i, j int) bool {
		return traceGroup.Traces[i].ID < traceGroup.Traces[j].ID
	})

	// Get the status for all digests in the traces.
	digestStatuses := make([]frontend.DigestStatus, 0, maxDistinctDigestsToPresent)
	digestStatuses = append(digestStatuses, frontend.DigestStatus{
		Digest: digest,
		Status: exp.Classification(test, digest).String(),
	})
	uniqueDigests := map[types.Digest]bool{}

	for idx, oneTrace := range traceGroup.Traces {
		// Create a new trace entry.
		oneTrace.DigestIndices = make([]int, last+1)

		// We start at HEAD and work our way backwards. The digest that is the focus of this
		// search result is digestStatuses[0]. The most recently-seen digest that is not the
		// digest of focus will be digestStatus[1] and so on, up until we hit
		// maxDistinctDigestsToPresent.
		for j := last; j >= 0; j-- {
			d := oneTrace.RawTrace.Digests[j]
			if d == tiling.MissingDigest {
				oneTrace.DigestIndices[j] = missingDigestIndex
				continue
			}
			uniqueDigests[d] = true
			digestIndex := 0
			if d != digest {
				if index := findDigestIndex(d, digestStatuses); index != -1 {
					digestIndex = index
				} else {
					if len(digestStatuses) < maxDistinctDigestsToPresent {
						digestStatuses = append(digestStatuses, frontend.DigestStatus{
							Digest: d,
							Status: exp.Classification(test, d).String(),
						})
						digestIndex = len(digestStatuses) - 1
					} else {
						// Fold this into the last digest.
						digestIndex = maxDistinctDigestsToPresent - 1
					}
				}
			}
			oneTrace.DigestIndices[j] = digestIndex
		}

		for i, c := range comments {
			if c.QueryToMatch.MatchesParams(oneTrace.Params) {
				oneTrace.CommentIndices = append(oneTrace.CommentIndices, i)
			}
		}
		// No longer need the RawTrace data, now that it has been turned into the frontend version.
		oneTrace.RawTrace = nil
		traceGroup.Traces[idx] = oneTrace
	}
	traceGroup.Digests = digestStatuses
	traceGroup.TotalDigests = len(uniqueDigests)
}

// findDigestIndex returns the index of the digest d in digestInfo, or -1 if not found.
func findDigestIndex(d types.Digest, digestInfo []frontend.DigestStatus) int {
	for i, di := range digestInfo {
		if di.Digest == d {
			return i
		}
	}
	return -1
}

// UntriagedUnignoredTryJobExclusiveDigests implements the SearchAPI interface. It uses the cached
// TryJobResults, so as to improve performance.
func (s *SearchImpl) UntriagedUnignoredTryJobExclusiveDigests(ctx context.Context, psID tjstore.CombinedPSID) (*frontend.UntriagedDigestList, error) {
	var resultsForThisPS []tjstore.TryJobResult
	listTS := time.Now()
	clIdx := s.indexSource.GetIndexForCL(psID.CRS, psID.CL)
	if clIdx != nil && clIdx.LatestPatchSet.Equal(psID) {
		resultsForThisPS = clIdx.UntriagedResults
		listTS = clIdx.ComputedTS
	} else {
		// Index either has not yet been created for this CL or was too old to have been indexed.
		var err error
		resultsForThisPS, err = s.getTryJobResults(ctx, psID)
		if err != nil {
			return nil, skerr.Wrapf(err, "getting tryjob results for %v", psID)
		}
	}

	exp, err := s.getExpectations(ctx, psID.CL, psID.CRS)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	idx := s.indexSource.GetIndex()
	ignoreMatcher := idx.GetIgnoreMatcher()
	knownDigestsForTest := idx.DigestCountsByTest(types.IncludeIgnoredTraces)

	var returnDigests []types.Digest
	var returnCorpora []string

	for _, tr := range resultsForThisPS {
		if err := ctx.Err(); err != nil {
			return nil, skerr.Wrap(err)
		}
		tn := types.TestName(tr.ResultParams[types.PrimaryKeyField])
		if exp.Classification(tn, tr.Digest) != expectations.Untriaged {
			// It's been triaged already.
			continue
		}
		if _, ok := knownDigestsForTest[tn][tr.Digest]; ok {
			// It's already been seen on master
			continue
		}
		p := make(paramtools.Params, len(tr.ResultParams)+len(tr.GroupParams)+len(tr.Options))
		p.Add(tr.GroupParams)
		p.Add(tr.Options)
		p.Add(tr.ResultParams)
		if ignoreMatcher.MatchAnyParams(p) {
			// This trace matches an ignore
			continue
		}
		if corpus := p[types.CorpusField]; !util.In(corpus, returnCorpora) {
			returnCorpora = append(returnCorpora, corpus)
		}
		returnDigests = append(returnDigests, tr.Digest)
	}
	// Sort digests alphabetically for determinism.
	sort.Slice(returnDigests, func(i, j int) bool {
		return returnDigests[i] < returnDigests[j]
	})
	return &frontend.UntriagedDigestList{
		Digests: returnDigests,
		Corpora: returnCorpora,
		TS:      listTS,
	}, nil
}

// getTriageHistory returns all TriageHistory for a given name and digest.
func (s *SearchImpl) getTriageHistory(ctx context.Context, history triageHistoryGetter, name types.TestName, digest types.Digest) []frontend.TriageHistory {
	id := expectations.ID{
		Grouping: name,
		Digest:   digest,
	}
	if cv, ok := s.triageHistoryCache.Load(id); ok {
		if rv, ok := cv.([]frontend.TriageHistory); ok {
			return rv
		}
		// purge the corrupt entry from the cache
		s.triageHistoryCache.Delete(id)
	}
	xth, err := history.GetTriageHistory(ctx, name, digest)
	if err != nil {
		metrics2.GetCounter("gold_search_triage_history_failures").Inc(1)
		sklog.Errorf("Could not get triage history, falling back to no history: %s", err)
		return nil
	}
	var rv []frontend.TriageHistory
	for _, th := range xth {
		rv = append(rv, frontend.TriageHistory{
			User: th.User,
			TS:   th.TS,
		})
	}
	s.triageHistoryCache.Store(id, rv)
	return rv
}

// addTriageHistory fills in the TriageHistory field of the passed in SRDigests. It does so in
// parallel to reduce latency of the response.
func (s *SearchImpl) addTriageHistory(ctx context.Context, history triageHistoryGetter, digestResults []*frontend.SearchResult) {
	defer shared.NewMetricsTimer("addTriageHistory").Stop()
	wg := sync.WaitGroup{}
	wg.Add(len(digestResults))
	for i, dr := range digestResults {
		go func(i int, dr *frontend.SearchResult) {
			defer wg.Done()
			if dr == nil {
				// This should never happen
				return
			}
			digestResults[i].TriageHistory = s.getTriageHistory(ctx, history, dr.Test, dr.Digest)
		}(i, dr)
	}
	wg.Wait()
}

// Make sure SearchImpl fulfills the SearchAPI interface.
var _ SearchAPI = (*SearchImpl)(nil)
