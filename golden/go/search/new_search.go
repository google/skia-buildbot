package search

import (
	"fmt"
	"sort"
	"sync"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/storage"
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
	Test       string                   `json:"test"`
	Digest     string                   `json:"digest"`
	Status     string                   `json:"status"`
	ParamSet   map[string][]string      `json:"paramset"`
	Traces     *Traces                  `json:"traces"`
	ClosestRef string                   `json:"closestRef"`
	RefDiffs   map[string]*SRDiffDigest `json:"refDiffs"`
	Blame      *blame.BlameDistribution `json:"blame"`
}

// SRDiffDigest captures the diff information between
// a primary digest and the digest given here. The primary
// digest is given by the context where this is used.
type SRDiffDigest struct {
	*diff.DiffMetrics
	Test     string              `json:"test"`
	Digest   string              `json:"digest"`
	Status   string              `json:"status"`
	ParamSet map[string][]string `json:"paramset"`
	N        int                 `json:"n"`
}

// NewSearchResponse is the structure returned by the
// Search(...) function of SearchAPI and intended to be
// returned as JSON in an HTTP response.
type NewSearchResponse struct {
	Digests []*SRDigest      `json:"digests"`
	Offset  int              `json:"offset"`
	Size    int              `json:"size"`
	Commits []*tiling.Commit `json:"commits"`
}

// DigestDetails contains details about a digest.
type SRDigestDetails struct {
	Digest  *SRDigest        `json:"digest"`
	Commits []*tiling.Commit `json:"commits"`
}

// SearchAPI is type that exposes a search API to query the
// current tile (all images for the most recent commits).
type SearchAPI struct {
	storages *storage.Storage
	ixr      *indexer.Indexer
}

// Create a new instance of SearchAPI.
func NewSearchAPI(storages *storage.Storage, ixr *indexer.Indexer) (*SearchAPI, error) {
	return &SearchAPI{
		storages: storages,
		ixr:      ixr,
	}, nil
}

// Search queries the current tile based on the parameters specified in
// the instance of Query.
func (s *SearchAPI) Search(q *Query) (*NewSearchResponse, error) {
	// Get the expectations and the current index, which we assume constant
	// for the duration of this query.
	exp, err := s.storages.ExpectationsStore.Get()
	if err != nil {
		return nil, err
	}
	idx := s.ixr.GetIndex()

	// Unconditional query stage. Iterate through the tile and get an intermediate
	// representation that contains all the traces matching the queries.
	inter, err := s.filterTile(q, idx)

	// Diff stage: Compare all digests found in the previous stages and find
	// reference points (positive, negative etc.) for each digest.
	ret := s.getReferenceDiffs(q.Metric, q.Match, q.IncludeIgnores, inter, exp, idx)
	if err != nil {
		return nil, err
	}

	// Post-diff stage: Apply all filters that are relevant once we have
	// diff values for the digests.
	ret = s.afterDiffResultFilter(ret, q)

	// Sort the digests and fill the ones that are going to be displayed with
	// additional data. Note we are returning all digests found, so we can do
	// bulk triage, but only the digests that are going to be shown are padded
	// with additional information.
	displayRet, offset := s.sortAndLimitDigests(q, ret, int(q.Offset), int(q.Limit))
	s.addParamsAndTraces(displayRet, inter, exp, idx)

	// Return all digests with the selected offset within the result set.
	return &NewSearchResponse{
		Digests: ret,
		Offset:  offset,
		Size:    len(displayRet),
		Commits: idx.GetTile(false).Commits,
	}, nil
}

// GetDigestDetails returns details about a digest as an instance of SRDigestDetails.
func (s *SearchAPI) GetDigestDetails(test, digest string) (*SRDigestDetails, error) {
	sklog.Infof("\n\nGOT: %s   %s\n\n", test, digest)

	idx := s.ixr.GetIndex()
	tile := idx.GetTile(true)

	exp, err := s.storages.ExpectationsStore.Get()
	if err != nil {
		return nil, err
	}

	oneInter := newSrIntermediate(test, digest, "", nil)
	for traceId, trace := range tile.Traces {
		if trace.Params()[types.PRIMARY_KEY_FIELD] != test {
			continue
		}
		gTrace := trace.(*types.GoldenTrace)
		for _, val := range gTrace.Values {
			if val == digest {
				oneInter.Add(traceId, trace)
				break
			}
		}
	}

	// Make sure we have at least one trace.
	if len(oneInter.traces) == 0 {
		return nil, fmt.Errorf("Unknown test and digest.")
	}

	// TODO(stephana): Make the metric, match and ignores parameters for the comparison.

	// Wrap the intermediate value in a map so we can re-use the search function for this.
	inter := map[string]map[string]*srIntermediate{test: {digest: oneInter}}
	ret := s.getReferenceDiffs(diff.METRIC_COMBINED, []string{types.PRIMARY_KEY_FIELD}, true, inter, exp, idx)
	if err != nil {
		return nil, err
	}

	// Get the params and traces.
	s.addParamsAndTraces(ret, inter, exp, idx)

	return &SRDigestDetails{
		Digest:  ret[0],
		Commits: tile.Commits,
	}, nil
}

// srIntermediate is the intermediate representation of a single digest
// found by the search. It is used to avoid multiple passes through the tile
// by accumulating the parameters that generated a specific digest and by
// capturing the traces.
type srIntermediate struct {
	test   string
	digest string
	traces map[string]*types.GoldenTrace
	params paramtools.ParamSet
}

// newSrIntermediate creates a new srIntermediate for a digest and adds
// the given trace to it.
func newSrIntermediate(test, digest, traceID string, trace tiling.Trace) *srIntermediate {
	ret := &srIntermediate{
		test:   test,
		digest: digest,
		params: paramtools.ParamSet{},
		traces: map[string]*types.GoldenTrace{},
	}
	if (traceID != "") && (trace != nil) {
		ret.Add(traceID, trace)
	}
	return ret
}

// Add adds a new trace to an existing intermediate value for a digest
// found in search.
func (s *srIntermediate) Add(traceID string, trace tiling.Trace) {
	s.traces[traceID] = trace.(*types.GoldenTrace)
	s.params.AddParams(trace.Params())
}

// TODO(stephana): The filterTile function should be merged with the
// function of the same name at the module level (see search.go).

// filterTile iterates over the tile and accumulates the traces
// that match the given query creating the initial search result.
func (s *SearchAPI) filterTile(q *Query, idx *indexer.SearchIndex) (map[string]map[string]*srIntermediate, error) {
	var acceptFn AcceptFn = nil
	if q.FGroupTest == GROUP_TEST_MAX_COUNT {
		maxDigestsByTest := idx.MaxDigestsByTest(q.IncludeIgnores)
		acceptFn = func(trace *types.GoldenTrace, digests []string) (bool, interface{}) {
			testName := trace.Params_[types.PRIMARY_KEY_FIELD]
			for _, d := range digests {
				if maxDigestsByTest[testName][d] {
					return true, nil
				}
			}
			return false, nil
		}
	}

	// Add digest/trace to the result.
	ret := map[string]map[string]*srIntermediate{}
	addFn := func(test, digest, traceID string, trace *types.GoldenTrace, acceptRet interface{}) {
		if testMap, ok := ret[test]; !ok {
			ret[test] = map[string]*srIntermediate{digest: newSrIntermediate(test, digest, traceID, trace)}
		} else if entry, ok := testMap[digest]; !ok {
			testMap[digest] = newSrIntermediate(test, digest, traceID, trace)
		} else {
			entry.Add(traceID, trace)
		}
	}

	if err := iterTile(q, addFn, acceptFn, s.storages, idx); err != nil {
		return nil, err
	}

	return ret, nil
}

// getReferenceDiffs compares all digests collected in the intermediate representation
// and compares them to the other known results for the test at hand.
func (s *SearchAPI) getReferenceDiffs(metric string, match []string, includeIgnores bool, inter map[string]map[string]*srIntermediate, exp *expstorage.Expectations, idx *indexer.SearchIndex) []*SRDigest {
	// Get the total number of digests we have at this point.
	// This allows to allocate the result below more efficiently and avoid using
	// a lock to write the results.
	nDigests := 0
	for _, digestInfo := range inter {
		nDigests += len(digestInfo)
	}

	refDiffer := NewRefDiffer(exp, s.storages.DiffStore, idx)
	retDigests := make([]*SRDigest, nDigests, nDigests)
	index := 0
	var wg sync.WaitGroup
	wg.Add(nDigests)
	for _, testDigests := range inter {
		for _, interValue := range testDigests {
			go func(i *srIntermediate, index int) {
				closestRef, refDiffs := refDiffer.GetRefDiffs(metric, match, i.test, i.digest, i.params, i.traces, includeIgnores)
				retDigests[index] = &SRDigest{
					Test:       i.test,
					Digest:     i.digest,
					Status:     exp.Classification(i.test, i.digest).String(),
					ClosestRef: closestRef,
					RefDiffs:   refDiffs,
				}
				wg.Done()
			}(interValue, index)
			index++
		}
	}
	wg.Wait()
	return retDigests
}

// afterDiffResultFilter filters the results based on the diff results in 'digestInfo'.
func (s *SearchAPI) afterDiffResultFilter(digestInfo []*SRDigest, q *Query) []*SRDigest {
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
func (s *SearchAPI) sortAndLimitDigests(q *Query, digestInfo []*SRDigest, offset, limit int) ([]*SRDigest, int) {
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
func (s *SearchAPI) addParamsAndTraces(digestInfo []*SRDigest, inter map[string]map[string]*srIntermediate, exp *expstorage.Expectations, idx *indexer.SearchIndex) {
	tile := idx.GetTile(false)
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
func (s *SearchAPI) getDrawableTraces(test, digest string, last int, exp *expstorage.Expectations, traces map[string]*types.GoldenTrace) *Traces {
	// Get the information necessary to draw the traces.
	traceIDs := make([]string, 0, len(traces))
	for traceID := range traces {
		traceIDs = append(traceIDs, traceID)
	}
	sort.Strings(traceIDs)

	// Get the status for all digests in the traces.
	digestStatuses := make([]DigestStatus, 0, MAX_REF_DIGESTS)
	digestStatuses = append(digestStatuses, DigestStatus{
		Digest: digest,
		Status: exp.Classification(test, digest).String(),
	})

	outputTraces := make([]Trace, len(traces), len(traces))
	for i, traceID := range traceIDs {
		// Create a new trace entry.
		oneTrace := traces[traceID]
		tr := &outputTraces[i]
		tr.ID = traceID
		tr.Params = oneTrace.Params_
		tr.Data = make([]Point, last+1, last+1)
		insertNext := last

		for j := last; j >= 0; j-- {
			d := oneTrace.Values[j]
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
