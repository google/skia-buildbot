package search

import (
	"context"
	"sort"
	"sync"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
)

const (
	// MAX_REF_DIGESTS is the maximum number of digests we want to show
	// in a dotted line of traces. We assume that showing more digests yields
	// no additional information, because the trace is likely to be flaky.
	MAX_REF_DIGESTS = 9
)

// TODO (stephana): Remove the Search(...) function in
// search.go once the Search function below is feature
// complete. This requires renaming some of the types that
// currently have a "SR" prefix. Changes will include these types:
//     SRDigest
//     SRDiffDigest
//     NewSearchResponse
//
// Some function currently at the module level should
// be merged into the SearchAPI type. Some of these are:
//     CompareDigests
//     GetDigestDetails
//

// SRDigest is a single search result digest returned
// by the Search function below.
type SRDigest struct {
	Test       types.TestName                 `json:"test"`
	Digest     types.Digest                   `json:"digest"`
	Status     string                         `json:"status"`
	ParamSet   map[string][]string            `json:"paramset"`
	Traces     *Traces                        `json:"traces"`
	ClosestRef types.Digest                   `json:"closestRef"`
	RefDiffs   map[types.Digest]*SRDiffDigest `json:"refDiffs"`
}

// SRDiffDigest captures the diff information between
// a primary digest and the digest given here. The primary
// digest is given by the context where this is used.
type SRDiffDigest struct {
	*diff.DiffMetrics
	Test     types.TestName      `json:"test"`
	Digest   types.Digest        `json:"digest"`
	Status   string              `json:"status"`
	ParamSet map[string][]string `json:"paramset"`
	N        int                 `json:"n"`
}

// NewSearchResponse is the structure returned by the
// Search(...) function of SearchAPI and intended to be
// returned as JSON in an HTTP response.
type NewSearchResponse struct {
	Digests []*SRDigest        `json:"digests"`
	Offset  int                `json:"offset"`
	Size    int                `json:"size"`
	Commits []*tiling.Commit   `json:"commits"`
	Issue   *tryjobstore.Issue `json:"issue"`
}

// DigestDetails contains details about a digest.
type SRDigestDetails struct {
	Digest  *SRDigest        `json:"digest"`
	Commits []*tiling.Commit `json:"commits"`
}

// SearchAPI is type that exposes a search API to query the
// current tile (all images for the most recent commits).
type SearchAPI struct {
	DiffStore         diff.DiffStore
	ExpectationsStore expstorage.ExpectationsStore
	Indexer           *indexer.Indexer
	TryjobStore       tryjobstore.TryjobStore
	// optional. If specified, will only show the params that match this query. This is
	// opt-in, to avoid leaking.
	PubliclyViewableParams paramtools.ParamSet
}

// Search queries the current tile based on the parameters specified in
// the instance of Query.
func (s *SearchAPI) Search(ctx context.Context, q *Query) (*NewSearchResponse, error) {
	ctx, span := trace.StartSpan(ctx, "search/Search")
	defer span.End()

	// Keep track if we are including reference diffs. This is going to be true
	// for the majority of queries.
	getRefDiffs := !q.NoDiff

	isTryjobSearch := !types.IsMasterBranch(q.Issue)

	// Get the expectations and the current index, which we assume constant
	// for the duration of this query.
	exp, err := s.getExpectationsFromQuery(q)
	if err != nil {
		return nil, err
	}
	idx := s.Indexer.GetIndex()

	var inter srInterMap = nil
	var issue *tryjobstore.Issue = nil

	// Find the digests (left hand side) we are interested in.
	if isTryjobSearch {
		// Search the tryjob results for the issue at hand.
		inter, issue, err = s.queryIssue(ctx, q, idx, exp)
	} else {
		// Iterate through the tile and get an intermediate
		// representation that contains all the traces matching the queries.
		inter, err = s.filterTile(ctx, q, exp, idx)
	}
	if err != nil {
		return nil, err
	}

	// Convert the intermediate representation to the list of digests that we
	// are going to return to the client.
	ret := s.getDigestRecs(inter, exp)

	// Get reference diffs unless it was specifically disabled.
	if getRefDiffs {
		// Diff stage: Compare all digests found in the previous stages and find
		// reference points (positive, negative etc.) for each digest.
		s.getReferenceDiffs(ctx, ret, q.Metric, q.Match, q.RQuery, q.IgnoreState(), exp, idx)
		if err != nil {
			return nil, err
		}

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
	searchRet := &NewSearchResponse{
		Digests: ret,
		Offset:  offset,
		Size:    len(displayRet),
		Commits: idx.CpxTile().GetTile(types.ExcludeIgnoredTraces).Commits,
		Issue:   issue,
	}
	return searchRet, nil
}

// Summary returns a high level summary of a Gerrit issue and the tryjobs
// that have been run for it.
func (s *SearchAPI) Summary(issueID int64) (*IssueSummary, error) {
	// TODO(stephana): Implement this function, which currently serves dummy
	// data so we can implement the frontend and nail down the API.
	ret := &IssueSummary{
		ID:          issueID,
		TimeStampMs: util.TimeStampMs(),
		PatchSets: []*PatchsetSummary{
			{
				PatchsetID:   1,
				TotalJobs:    10,
				FinishedJobs: 10,
				TotalImg:     1520,
				NewImg:       91,
				UntriagedImg: 44,
			},
			{
				PatchsetID:   2,
				TotalJobs:    8,
				FinishedJobs: 8,
				TotalImg:     884,
				NewImg:       64,
				UntriagedImg: 12,
			},
			{
				PatchsetID:   5,
				TotalJobs:    8,
				FinishedJobs: 8,
				TotalImg:     553,
				NewImg:       12,
				UntriagedImg: 6,
			},
			{
				PatchsetID:   6,
				TotalJobs:    8,
				FinishedJobs: 4,
				TotalImg:     1912,
				NewImg:       8,
				UntriagedImg: 0,
			},
		},
	}

	return ret, nil
}

// GetDigestDetails returns details about a digest as an instance of SRDigestDetails.
func (s *SearchAPI) GetDigestDetails(test types.TestName, digest types.Digest) (*SRDigestDetails, error) {
	ctx := context.Background()
	idx := s.Indexer.GetIndex()
	tile := idx.CpxTile().GetTile(types.IncludeIgnoredTraces)

	exp, err := s.getExpectationsFromQuery(nil)
	if err != nil {
		return nil, err
	}

	oneInter := newSrIntermediate(test, digest, "", nil, nil)
	for traceId, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		if gTrace.TestName() != test {
			continue
		}

		for _, val := range gTrace.Digests {
			if val == digest {
				oneInter.add(traceId, trace, nil)
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
	if err != nil {
		return nil, err
	}

	if hasTraces {
		// Get the params and traces.
		s.addParamsAndTraces(ctx, ret, inter, exp, idx)
	}

	return &SRDigestDetails{
		Digest:  ret[0],
		Commits: tile.Commits,
	}, nil
}

// getExpectationsFromQuery returns a slice of expectations that should be
// used in the given query. It will add the issue expectations if this is
// querying tryjob results. If query is nil the expectations of the master
// tile are returned.
func (s *SearchAPI) getExpectationsFromQuery(q *Query) (ExpSlice, error) {
	ret := make(ExpSlice, 0, 2)

	if q != nil && !types.IsMasterBranch(q.Issue) {
		issueExpStore := s.ExpectationsStore.ForIssue(q.Issue)
		tjExp, err := issueExpStore.Get()
		if err != nil {
			return nil, sklog.FmtErrorf("Unable to load expectations for issue %d from tryjobstore: %s", q.Issue, err)
		}
		ret = append(ret, tjExp)
	}

	exp, err := s.ExpectationsStore.Get()
	if err != nil {
		return nil, sklog.FmtErrorf("Unable to load expectations for master: %s", err)
	}
	ret = append(ret, exp)
	return ret, nil
}

// query issue returns the digest related to this issues in intermediate representation.
func (s *SearchAPI) queryIssue(ctx context.Context, q *Query, idx *indexer.SearchIndex, exp ExpSlice) (srInterMap, *tryjobstore.Issue, error) {
	ctx, span := trace.StartSpan(ctx, "search/queryIssue")
	defer span.End()

	// Build the intermediate map to compare against the tile
	ret := srInterMap{}

	// Adjust the add function to exclude digests already in the master branch
	addFn := ret.add
	if !q.IncludeMaster {
		talliesByTest := idx.DigestCountsByTest(q.IgnoreState())
		addFn = func(test types.TestName, digest types.Digest, traceID tiling.TraceId, trace *types.GoldenTrace, params paramtools.ParamSet) {
			// Include the digest if either the test or the digest is not in the master tile.
			if _, ok := talliesByTest[test][digest]; !ok {
				ret.add(test, digest, traceID, trace, params)
			}
		}
	}

	issue, err := s.extractIssueDigests(ctx, q, idx, exp, addFn)
	if err != nil {
		return nil, nil, err
	}
	return ret, issue, nil
}

// filterAddFn is a filter and add function that is passed to the getIssueDigest interface. It will
// be called for each testName/digest combination and should accumulate the digests of interest.
type filterAddFn func(test types.TestName, digest types.Digest, traceID tiling.TraceId, trace *types.GoldenTrace, params paramtools.ParamSet)

// extractIssueDigests loads the issue and its tryjob results and then filters the
// results via the given query. For each testName/digest pair addFn is called.
func (s *SearchAPI) extractIssueDigests(ctx context.Context, q *Query, idx *indexer.SearchIndex, exp ExpSlice, addFn filterAddFn) (*tryjobstore.Issue, error) {
	_, span := trace.StartSpan(ctx, "search/queryIssue")
	defer span.End()

	// Get the issue.
	issue, err := s.TryjobStore.GetIssue(q.Issue, true)
	if err != nil {
		return nil, err
	}

	if issue == nil {
		return nil, sklog.FmtErrorf("Unable to find issue %d", q.Issue)
	}

	// If no patchsets were given we pick the last one that has tryjobs.
	issue.QueryPatchsets = q.Patchsets
	if len(issue.QueryPatchsets) == 0 {
		issue.QueryPatchsets = make([]int64, 0, len(issue.PatchsetDetails))
		for i := len(issue.PatchsetDetails) - 1; i >= 0; i-- {
			ps := issue.PatchsetDetails[i]
			if len(ps.Tryjobs) > 0 {
				issue.QueryPatchsets = append(issue.QueryPatchsets, ps.ID)
				break
			}
		}
	}

	// Extract the list of tryjobs to consider.
	tryjobs := []*tryjobstore.Tryjob{}
	for _, psID := range issue.QueryPatchsets {
		tryjobs = append(tryjobs, issue.FindPatchset(psID).Tryjobs...)
	}

	// If there are no tryjobs we are done.
	if len(tryjobs) == 0 {
		return issue, nil
	}

	// Get the results
	tjResults, err := s.TryjobStore.GetTryjobResults(tryjobs)
	if err != nil {
		return nil, err
	}

	// Filter the ignored results by setting the results to nil.
	if !q.IncludeIgnores {
		ignoreMatcher := idx.GetIgnoreMatcher()
		for _, oneTryjob := range tjResults {
			for idx, trj := range oneTryjob {
				if ignoreMatcher.MatchAny(trj.Params) {
					oneTryjob[idx] = nil
				}
			}
		}
	}

	// If we have a white list filter out anything that is not on the white list.
	if len(s.PubliclyViewableParams) > 0 {
		for _, oneTryjob := range tjResults {
			for idx, trj := range oneTryjob {
				if (trj != nil) && !s.PubliclyViewableParams.Matches(trj.Params) {
					oneTryjob[idx] = nil
				}
			}
		}
	}

	// Iterate over the remaining results.
	pq := paramtools.ParamSet(q.Query)
	for _, tryjobResults := range tjResults {
		for _, tjr := range tryjobResults {
			if tjr != nil {
				// Filter by query.
				if pq.Matches(tjr.Params) {
					// Filter by classification.
					cl := exp.Classification(tjr.TestName, tjr.Digest)
					if !q.excludeClassification(cl) {
						tn := types.TestName(tjr.Params[types.PRIMARY_KEY_FIELD][0])
						addFn(tn, tjr.Digest, tiling.TraceId(""), nil, tjr.Params)
					}
				}
			}
		}
	}
	return issue, nil
}

// TODO(kjlubick): The filterTile function should be merged with the
// filterTileCompare (see search.go).

// filterTile iterates over the tile and accumulates the traces
// that match the given query creating the initial search result.
func (s *SearchAPI) filterTile(ctx context.Context, q *Query, exp ExpSlice, idx *indexer.SearchIndex) (srInterMap, error) {
	_, span := trace.StartSpan(ctx, "search/filterTile")
	defer span.End()

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
		ret.add(test, digest, traceID, trace, nil)
	}

	if err := iterTile(q, addFn, acceptFn, exp, idx); err != nil {
		return nil, err
	}

	return ret, nil
}

// getDigestRecs takes the intermediate results and converts them to the list
// of records that will be returned to the client.
func (s *SearchAPI) getDigestRecs(inter srInterMap, exps ExpSlice) []*SRDigest {
	// Get the total number of digests we have at this point.
	nDigests := 0
	for _, digestInfo := range inter {
		nDigests += len(digestInfo)
	}

	retDigests := make([]*SRDigest, 0, nDigests)
	for _, testDigests := range inter {
		for _, interValue := range testDigests {
			retDigests = append(retDigests, &SRDigest{
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
func (s *SearchAPI) getReferenceDiffs(ctx context.Context, resultDigests []*SRDigest, metric string, match []string, rhsQuery paramtools.ParamSet, is types.IgnoreState, exp ExpSlice, idx *indexer.SearchIndex) {
	_, span := trace.StartSpan(ctx, "search/getReferenceDiffs")
	defer span.End()

	refDiffer := NewRefDiffer(exp, s.DiffStore, idx)
	var wg sync.WaitGroup
	wg.Add(len(resultDigests))
	for _, retDigest := range resultDigests {
		go func(retDigest *SRDigest) {
			closestRef, refDiffs := refDiffer.GetRefDiffs(metric, match, retDigest.Test, retDigest.Digest, retDigest.ParamSet, rhsQuery, is)
			retDigest.ClosestRef = closestRef
			retDigest.RefDiffs = refDiffs

			// Remove the paramset since it will not be necessary for all results.
			retDigest.ParamSet = nil
			wg.Done()
		}(retDigest)
	}
	wg.Wait()
}

// afterDiffResultFilter filters the results based on the diff results in 'digestInfo'.
func (s *SearchAPI) afterDiffResultFilter(ctx context.Context, digestInfo []*SRDigest, q *Query) []*SRDigest {
	_, span := trace.StartSpan(ctx, "search/afterDiffResultFilter")
	defer span.End()

	newDigestInfo := make([]*SRDigest, 0, len(digestInfo))
	filterRGBADiff := (q.FRGBAMin > 0) || (q.FRGBAMax < 255)
	filterDiffMax := (q.FDiffMax >= 0)
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
func (s *SearchAPI) sortAndLimitDigests(ctx context.Context, q *Query, digestInfo []*SRDigest, offset, limit int) ([]*SRDigest, int) {
	_, span := trace.StartSpan(ctx, "search/sortAndLimitDigests")
	defer span.End()

	fullLength := len(digestInfo)
	if offset >= fullLength {
		return []*SRDigest{}, 0
	}

	sortSlice := sort.Interface(newSRDigestSlice(q.Metric, digestInfo))
	if q.Sort == SORT_DESC {
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
func (s *SearchAPI) addParamsAndTraces(ctx context.Context, digestInfo []*SRDigest, inter srInterMap, exp ExpSlice, idx *indexer.SearchIndex) {
	_, span := trace.StartSpan(ctx, "search/addParamsAndTraces")
	defer span.End()

	tile := idx.CpxTile().GetTile(types.ExcludeIgnoredTraces)
	last := tile.LastCommitIndex()
	for _, di := range digestInfo {
		// Add the parameters and the drawable traces to the result.
		di.ParamSet = inter[di.Test][di.Digest].params
		di.Traces = s.getDrawableTraces(di.Test, di.Digest, last, exp, inter[di.Test][di.Digest].traces)
		di.Traces.TileSize = len(tile.Commits)
	}
}

// getDrawableTraces returns an instance of Traces which allows to draw the
// traces for the given test/digest.
func (s *SearchAPI) getDrawableTraces(test types.TestName, digest types.Digest, last int, exp ExpSlice, traces map[tiling.TraceId]*types.GoldenTrace) *Traces {
	// Get the information necessary to draw the traces.
	traceIDs := make(tiling.TraceIdSlice, 0, len(traces))
	for traceID := range traces {
		traceIDs = append(traceIDs, traceID)
	}
	sort.Sort(traceIDs)

	// Get the status for all digests in the traces.
	digestStatuses := make([]DigestStatus, 0, MAX_REF_DIGESTS)
	digestStatuses = append(digestStatuses, DigestStatus{
		Digest: digest,
		Status: exp.Classification(test, digest).String(),
	})

	outputTraces := make([]Trace, len(traces))
	for i, traceID := range traceIDs {
		// Create a new trace entry.
		oneTrace := traces[traceID]
		tr := &outputTraces[i]
		tr.ID = traceID
		tr.Params = oneTrace.Keys
		tr.Data = make([]Point, last+1)
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
						digestStatuses = append(digestStatuses, DigestStatus{
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
			tr.Data[insertNext] = Point{
				X: j,
				Y: i,
				S: refDigestStatus,
			}
			insertNext--
		}
		// Trim the leading traces if necessary.
		tr.Data = tr.Data[insertNext+1:]
	}

	return &Traces{
		Digests: digestStatuses,
		Traces:  outputTraces,
	}
}

// srDigestSlice is a utility type for sorting slices of SRDigest by their max diff.
type srDigestSliceLessFn func(i, j *SRDigest) bool
type srDigestSlice struct {
	slice  []*SRDigest
	lessFn srDigestSliceLessFn
}

// newSRDigestSlice creates a new instance of srDigestSlice that wraps around
// a slice of result digests.
func newSRDigestSlice(metric string, slice []*SRDigest) *srDigestSlice {
	// Sort by increasing by diff metric. Not having a diff metric puts the item at the bottom
	// of the list.
	lessFn := func(i, j *SRDigest) bool {
		if (i.ClosestRef == "") && (j.ClosestRef == "") {
			return i.Digest < j.Digest
		}

		if i.ClosestRef == "" {
			return false
		}
		if j.ClosestRef == "" {
			return true
		}
		iDiff := i.RefDiffs[i.ClosestRef].Diffs[metric]
		jDiff := j.RefDiffs[j.ClosestRef].Diffs[metric]

		// If they are the same then sort by digest to make the result stable.
		if iDiff == jDiff {
			return i.Digest < j.Digest
		}
		return iDiff < jDiff
	}

	return &srDigestSlice{
		slice:  slice,
		lessFn: lessFn,
	}
}

// Len, Less, Swap implement the sort.Interface.
func (s *srDigestSlice) Len() int           { return len(s.slice) }
func (s *srDigestSlice) Less(i, j int) bool { return s.lessFn(s.slice[i], s.slice[j]) }
func (s *srDigestSlice) Swap(i, j int)      { s.slice[i], s.slice[j] = s.slice[j], s.slice[i] }
