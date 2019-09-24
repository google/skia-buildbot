// The search package contains the core functionality for searching
// for digests across a tile.
package search

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/search/ref_differ"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/types"
)

const (
	// MAX_REF_DIGESTS is the maximum number of digests we want to show
	// in a dotted line of traces. We assume that showing more digests yields
	// no additional information, because the trace is likely to be flaky.
	MAX_REF_DIGESTS = 9

	// TODO(kjlubick): no tests for this option yet.
	GROUP_TEST_MAX_COUNT = "count"
)

// SearchImpl holds onto various objects needed to search the latest
// tile for digests. It implements the SearchAPI interface.
type SearchImpl struct {
	diffStore         diff.DiffStore
	expectationsStore expstorage.ExpectationsStore
	indexSource       indexer.IndexSource
	changeListStore   clstore.Store
	tryJobStore       tjstore.Store

	// optional. If specified, will only show the params that match this query. This is
	// opt-in, to avoid leaking.
	publiclyViewableParams paramtools.ParamSet
}

// New returns a new SearchImpl instance.
func New(ds diff.DiffStore, es expstorage.ExpectationsStore, is indexer.IndexSource, cls clstore.Store, tjs tjstore.Store, publiclyViewableParams paramtools.ParamSet) *SearchImpl {
	return &SearchImpl{
		diffStore:              ds,
		expectationsStore:      es,
		indexSource:            is,
		changeListStore:        cls,
		tryJobStore:            tjs,
		publiclyViewableParams: publiclyViewableParams,
	}
}

// Search implements the SearchAPI interface.
func (s *SearchImpl) Search(ctx context.Context, q *query.Search) (*frontend.SearchResponse, error) {
	defer metrics2.FuncTimer().Stop()
	if q == nil {
		return nil, skerr.Fmt("nil query")
	}

	// Keep track if we are including reference diffs. This is going to be true
	// for the majority of queries.
	getRefDiffs := !q.NoDiff
	isChangeListSearch := !types.IsMasterBranch(q.DeprecatedIssue)
	// Get the expectations and the current index, which we assume constant
	// for the duration of this query.
	crs := ""
	if s.changeListStore != nil {
		crs = s.changeListStore.System()
	}
	exp, err := s.getExpectationsFromQuery(q.ChangeListID, crs)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	idx := s.indexSource.GetIndex()

	var inter srInterMap = nil

	// Find the digests (left hand side) we are interested in.
	if isChangeListSearch {
		if q.NewCLStore {
			inter, err = s.queryChangeList(ctx, q, idx, exp)
			if err != nil {
				return nil, skerr.Wrapf(err, "getting digests from new clstore/tjstore")
			}
		}
	} else {
		// Iterate through the tile and get an intermediate
		// representation that contains all the traces matching the queries.
		inter, err = s.filterTile(ctx, q, exp, idx)
		if err != nil {
			return nil, skerr.Wrapf(err, "getting digests from master tile")
		}
	}

	// Convert the intermediate representation to the list of digests that we
	// are going to return to the client.
	ret := s.getDigestRecs(inter, exp)

	// Get reference diffs unless it was specifically disabled.
	if getRefDiffs {
		// Diff stage: Compare all digests found in the previous stages and find
		// reference points (positive, negative etc.) for each digest.
		s.getReferenceDiffs(ctx, ret, q.Metric, q.Match, q.RTraceValues, q.IgnoreState(), exp, idx)

		// Post-diff stage: Apply all filters that are relevant once we have
		// diff values for the digests.
		ret = s.afterDiffResultFilter(ctx, ret, q)
	}

	// Sort the digests and fill the ones that are going to be displayed with
	// additional data. Note we are returning all digests found, so we can do
	// bulk triage, but only the digests that are going to be shown are padded
	// with additional information.
	displayRet, offset := s.sortAndLimitDigests(ctx, q, ret, int(q.Offset), int(q.Limit))
	s.addParamsAndTraces(ctx, displayRet, inter, exp, idx)

	// Return all digests with the selected offset within the result set.
	searchRet := &frontend.SearchResponse{
		Digests: ret,
		Offset:  offset,
		Size:    len(displayRet),
		// TODO(kjlubick) maybe omit Commits for ChangeList Queries.
		Commits: idx.Tile().GetTile(types.ExcludeIgnoredTraces).Commits,
	}
	return searchRet, nil
}

// GetDigestDetails implements the SearchAPI interface.
func (s *SearchImpl) GetDigestDetails(test types.TestName, digest types.Digest) (*frontend.DigestDetails, error) {
	defer metrics2.FuncTimer().Stop()
	ctx := context.TODO()
	idx := s.indexSource.GetIndex()
	tile := idx.Tile().GetTile(types.IncludeIgnoredTraces)

	exp, err := s.getExpectationsFromQuery("", s.changeListStore.System())
	if err != nil {
		return nil, err
	}

	oneInter := newSrIntermediate(test, digest, "", nil, nil)
	for traceId, t := range tile.Traces {
		gTrace := t.(*types.GoldenTrace)
		if gTrace.TestName() != test {
			continue
		}

		for _, val := range gTrace.Digests {
			if val == digest {
				oneInter.add(traceId, t, nil)
				break
			}
		}
	}

	// TODO(stephana): Make the metric, match and ignores parameters for the comparison.

	// If there are no traces or params then set them to nil to signal there are none.
	hasTraces := len(oneInter.traces) > 0
	if !hasTraces {
		oneInter.traces = nil
		oneInter.params = nil
	}

	// Wrap the intermediate value in a map so we can re-use the search function for this.
	inter := srInterMap{test: {digest: oneInter}}
	ret := s.getDigestRecs(inter, exp)
	s.getReferenceDiffs(ctx, ret, diff.METRIC_COMBINED, []string{types.PRIMARY_KEY_FIELD}, nil, types.ExcludeIgnoredTraces, exp, idx)

	if hasTraces {
		// Get the params and traces.
		s.addParamsAndTraces(ctx, ret, inter, exp, idx)
	}

	return &frontend.DigestDetails{
		Digest:  ret[0],
		Commits: tile.Commits,
	}, nil
}

// getExpectationsFromQuery returns a slice of expectations that should be
// used in the given query. It will add the issue expectations if this is
// querying ChangeList results. If query is nil the expectations of the master
// tile are returned.
func (s *SearchImpl) getExpectationsFromQuery(clID, crs string) (common.ExpSlice, error) {
	ret := make(common.ExpSlice, 0, 2)

	// TODO(kjlubick) remove the legacy value "0" once frontend changes have baked in.
	if clID != "" && clID != "0" {
		issueExpStore := s.expectationsStore.ForChangeList(clID, crs)
		tjExp, err := issueExpStore.Get()
		if err != nil {
			return nil, skerr.Wrapf(err, "loading expectations for cl %s (%s)", clID, crs)
		}
		ret = append(ret, tjExp)
	}

	exp, err := s.expectationsStore.Get()
	if err != nil {
		return nil, skerr.Wrapf(err, "loading expectations for master")
	}
	ret = append(ret, exp)
	return ret, nil
}

// queryChangeList returns the digests associated with the ChangeList referenced by q.CRSAndCLID
// in intermediate representation. It returns the filtered digests as specified by q. The param
// exp should contain the expectations for the given ChangeList.
func (s *SearchImpl) queryChangeList(ctx context.Context, q *query.Search, idx indexer.IndexSearcher, exp common.ExpSlice) (srInterMap, error) {
	// Build the intermediate map to compare against the tile
	ret := srInterMap{}

	// Adjust the add function to exclude digests already in the master branch
	addFn := ret.AddTestParams
	if !q.IncludeMaster {
		talliesByTest := idx.DigestCountsByTest(q.IgnoreState())
		addFn = func(test types.TestName, digest types.Digest, params paramtools.Params) {
			// Include the digest if either the test or the digest is not in the master tile.
			if _, ok := talliesByTest[test][digest]; !ok {
				ret.AddTestParams(test, digest, params)
			}
		}
	}

	err := s.extractChangeListDigests(ctx, q, idx, exp, addFn)
	if err != nil {
		return nil, skerr.Wrap(err)
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
func (s *SearchImpl) extractChangeListDigests(ctx context.Context, q *query.Search, idx indexer.IndexSearcher, exp common.ExpSlice, addFn filterAddFn) error {
	clID := q.ChangeListID
	// We know xps is sorted by order, if it is non-nil.
	xps, err := s.changeListStore.GetPatchSets(ctx, clID)
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

	xtr, err := s.tryJobStore.GetResults(ctx, id)
	if err != nil {
		return skerr.Wrapf(err, "getting tryjob results for %v", id)
	}
	wg := sync.WaitGroup{}
	addMutex := sync.Mutex{}
	chunkSize := len(xtr) / extractFilterShards
	// Very small shards are likely not worth the overhead.
	if chunkSize < 50 {
		chunkSize = 50
	}
	queryParams := paramtools.ParamSet(q.TraceValues)
	ignoreMatcher := idx.GetIgnoreMatcher()

	// passed in func does not return error, so neither will ChunkIter
	_ = util.ChunkIter(len(xtr), chunkSize, func(start, stop int) error {
		wg.Add(1)
		// stop is exclusive
		go func(start, stop int) {
			defer wg.Done()
			sliced := xtr[start:stop]
			for _, tr := range sliced {
				tn := types.TestName(tr.ResultParams[types.PRIMARY_KEY_FIELD])
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
				if !q.IncludeIgnores {
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
		}(start, stop)
		return nil
	})

	wg.Wait()
	return nil
}

// DiffDigests implements the SearchAPI interface.
func (s *SearchImpl) DiffDigests(test types.TestName, left, right types.Digest) (*frontend.DigestComparison, error) {
	defer metrics2.FuncTimer().Stop()
	// Get the diff between the two digests
	diffResult, err := s.diffStore.Get(diff.PRIORITY_NOW, left, types.DigestSlice{right})
	if err != nil {
		return nil, err
	}

	// Return an error if we could not find the diff.
	if len(diffResult) != 1 {
		return nil, fmt.Errorf("could not find diff between %s and %s", left, right)
	}

	exp, err := s.expectationsStore.Get()
	if err != nil {
		return nil, err
	}

	idx := s.indexSource.GetIndex()

	return &frontend.DigestComparison{
		Left: &frontend.SRDigest{
			Test:     test,
			Digest:   left,
			Status:   exp.Classification(test, left).String(),
			ParamSet: idx.GetParamsetSummary(test, left, types.IncludeIgnoredTraces),
		},
		Right: &frontend.SRDiffDigest{
			Digest:      right,
			Status:      exp.Classification(test, right).String(),
			ParamSet:    idx.GetParamsetSummary(test, right, types.IncludeIgnoredTraces),
			DiffMetrics: diffResult[right].(*diff.DiffMetrics),
		},
	}, nil
}

// TODO(kjlubick): The filterTile function should be merged with the
// filterTileCompare (see search.go).

// filterTile iterates over the tile and accumulates the traces
// that match the given query creating the initial search result.
func (s *SearchImpl) filterTile(ctx context.Context, q *query.Search, exp common.ExpSlice, idx indexer.IndexSearcher) (srInterMap, error) {
	var acceptFn AcceptFn = nil
	if q.FGroupTest == GROUP_TEST_MAX_COUNT {
		maxDigestsByTest := idx.MaxDigestsByTest(q.IgnoreState())
		acceptFn = func(params paramtools.Params, digests types.DigestSlice) (bool, interface{}) {
			testName := types.TestName(params[types.PRIMARY_KEY_FIELD])
			for _, d := range digests {
				if maxDigestsByTest[testName][d] {
					return true, nil
				}
			}
			return false, nil
		}
	}

	// Add digest/trace to the result.
	ret := srInterMap{}
	addFn := func(test types.TestName, digest types.Digest, traceID tiling.TraceId, trace *types.GoldenTrace, acceptRet interface{}) {
		ret.Add(test, digest, traceID, trace, nil)
	}

	if err := iterTile(q, addFn, acceptFn, exp, idx); err != nil {
		return nil, skerr.Wrap(err)
	}

	return ret, nil
}

// getDigestRecs takes the intermediate results and converts them to the list
// of records that will be returned to the client.
func (s *SearchImpl) getDigestRecs(inter srInterMap, exps common.ExpSlice) []*frontend.SRDigest {
	// Get the total number of digests we have at this point.
	nDigests := 0
	for _, digestInfo := range inter {
		nDigests += len(digestInfo)
	}

	retDigests := make([]*frontend.SRDigest, 0, nDigests)
	for _, testDigests := range inter {
		for _, interValue := range testDigests {
			retDigests = append(retDigests, &frontend.SRDigest{
				Test:     interValue.test,
				Digest:   interValue.digest,
				Status:   exps.Classification(interValue.test, interValue.digest).String(),
				ParamSet: interValue.params,
			})
		}
	}
	return retDigests
}

// getReferenceDiffs compares all digests collected in the intermediate representation
// and compares them to the other known results for the test at hand.
func (s *SearchImpl) getReferenceDiffs(ctx context.Context, resultDigests []*frontend.SRDigest, metric string, match []string, rhsQuery paramtools.ParamSet, is types.IgnoreState, exp common.ExpSlice, idx indexer.IndexSearcher) {
	refDiffer := ref_differ.New(exp, s.diffStore, idx)
	var wg sync.WaitGroup
	wg.Add(len(resultDigests))
	for _, retDigest := range resultDigests {
		go func(d *frontend.SRDigest) {
			refDiffer.FillRefDiffs(d, metric, match, rhsQuery, is)
			// Remove the paramset since it will not be necessary for all results.
			d.ParamSet = nil
			wg.Done()
		}(retDigest)
	}
	wg.Wait()
}

// afterDiffResultFilter filters the results based on the diff results in 'digestInfo'.
func (s *SearchImpl) afterDiffResultFilter(ctx context.Context, digestInfo []*frontend.SRDigest, q *query.Search) []*frontend.SRDigest {
	newDigestInfo := make([]*frontend.SRDigest, 0, len(digestInfo))
	filterRGBADiff := (q.FRGBAMin > 0) || (q.FRGBAMax < 255)
	filterDiffMax := q.FDiffMax >= 0
	for _, digest := range digestInfo {
		ref, ok := digest.RefDiffs[digest.ClosestRef]

		// Filter all digests where MaxRGBA is within the given band.
		if filterRGBADiff {
			// If there is no diff metric we exclude the digest.
			if !ok {
				continue
			}

			rgbaMaxDiff := int32(util.MaxInt(ref.MaxRGBADiffs...))
			if (rgbaMaxDiff < q.FRGBAMin) || (rgbaMaxDiff > q.FRGBAMax) {
				continue
			}
		}

		// Filter all digests where the diff is below the given threshold.
		if filterDiffMax && (!ok || (ref.Diffs[q.Metric] > q.FDiffMax)) {
			continue
		}

		// If selected only consider digests that have a reference to compare to.
		if q.FRef && !ok {
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
func (s *SearchImpl) sortAndLimitDigests(ctx context.Context, q *query.Search, digestInfo []*frontend.SRDigest, offset, limit int) ([]*frontend.SRDigest, int) {
	fullLength := len(digestInfo)
	if offset >= fullLength {
		return []*frontend.SRDigest{}, 0
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

// addParamsAndTraces adds information to the given result that is necessary
// to draw them, i.e. the information what digest/image appears at what commit and
// what were the union of parameters that generate the digest. This should be
// only done for digests that are intended to be displayed.
func (s *SearchImpl) addParamsAndTraces(ctx context.Context, digestInfo []*frontend.SRDigest, inter srInterMap, exp common.ExpSlice, idx indexer.IndexSearcher) {
	tile := idx.Tile().GetTile(types.ExcludeIgnoredTraces)
	last := tile.LastCommitIndex()
	for _, di := range digestInfo {
		// Add the parameters and the drawable traces to the result.
		di.ParamSet = inter[di.Test][di.Digest].params
		di.ParamSet.Normalize()
		di.Traces = s.getDrawableTraces(di.Test, di.Digest, last, exp, inter[di.Test][di.Digest].traces)
		di.Traces.TileSize = len(tile.Commits)
	}
}

// getDrawableTraces returns an instance of TraceGroup which allows us
// to draw the traces for the given test/digest.
func (s *SearchImpl) getDrawableTraces(test types.TestName, digest types.Digest, last int, exp common.ExpSlice, traces map[tiling.TraceId]*types.GoldenTrace) *frontend.TraceGroup {
	// Get the information necessary to draw the traces.
	traceIDs := make(tiling.TraceIdSlice, 0, len(traces))
	for traceID := range traces {
		traceIDs = append(traceIDs, traceID)
	}
	sort.Sort(traceIDs)

	// Get the status for all digests in the traces.
	digestStatuses := make([]frontend.DigestStatus, 0, MAX_REF_DIGESTS)
	digestStatuses = append(digestStatuses, frontend.DigestStatus{
		Digest: digest,
		Status: exp.Classification(test, digest).String(),
	})

	outputTraces := make([]frontend.Trace, len(traces))
	for i, traceID := range traceIDs {
		// Create a new trace entry.
		oneTrace := traces[traceID]
		tr := &outputTraces[i]
		tr.ID = traceID
		tr.Params = oneTrace.Keys
		tr.Data = make([]frontend.Point, last+1)
		insertNext := last

		for j := last; j >= 0; j-- {
			d := oneTrace.Digests[j]
			if d == types.MISSING_DIGEST {
				continue
			}
			refDigestStatus := 0
			if d != digest {
				if index := digestIndex(d, digestStatuses); index != -1 {
					refDigestStatus = index
				} else {
					if len(digestStatuses) < MAX_REF_DIGESTS {
						digestStatuses = append(digestStatuses, frontend.DigestStatus{
							Digest: d,
							Status: exp.Classification(test, d).String(),
						})
						refDigestStatus = len(digestStatuses) - 1
					} else {
						// Fold this into the last digest.
						refDigestStatus = MAX_REF_DIGESTS - 1
					}
				}
			}

			// Insert the trace points from last to first.
			tr.Data[insertNext] = frontend.Point{
				X: j,
				Y: i,
				S: refDigestStatus,
			}
			insertNext--
		}
		// Trim the leading traces if necessary.
		tr.Data = tr.Data[insertNext+1:]
	}

	return &frontend.TraceGroup{
		Digests: digestStatuses,
		Traces:  outputTraces,
	}
}

// digestIndex returns the index of the digest d in digestInfo, or -1 if not found.
func digestIndex(d types.Digest, digestInfo []frontend.DigestStatus) int {
	for i, di := range digestInfo {
		if di.Digest == d {
			return i
		}
	}
	return -1
}

// Make sure SearchImpl fulfills the SearchAPI interface.
var _ SearchAPI = (*SearchImpl)(nil)
