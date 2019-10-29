// Package search contains the core functionality for searching for digests across a tile.
package search

import (
	"strings"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

// AcceptFn is the callback function used by iterTile to determine whether to
// include a trace into the result or not. The second return value is an
// intermediate result that will be passed to addFn to avoid redundant computation.
// The second return value is application dependent since it will be passed
// into the call to the corresponding AddFn. Determining whether to accept a
// result might require an expensive computation and we want to avoid repeating
// that computation in the 'add' step. So we can return it here and it will
// be passed into the instance of AddFn.
type AcceptFn func(params paramtools.Params, digests types.DigestSlice) (bool, interface{})

// AddFn is the callback function used by iterTile to add a digest and it's
// trace to the result. acceptResult is the same value returned by the AcceptFn.
type AddFn func(test types.TestName, digest types.Digest, traceID tiling.TraceId, trace *types.GoldenTrace, acceptResult interface{})

// iterTile is a generic function to extract information from a tile.
// It iterates over the tile and filters against the given query. If calls
// acceptFn to determine whether to keep a trace (after it has already been
// tested against the query) and calls addFn to add a digest and its trace.
// acceptFn == nil equals unconditional acceptance.
func iterTile(q *query.Search, addFn AddFn, acceptFn AcceptFn, exp common.ExpSlice, idx indexer.IndexSearcher) error {
	cpxTile := idx.Tile()
	selectedTile := cpxTile.GetTile(q.IgnoreState())

	if acceptFn == nil {
		acceptFn = func(params paramtools.Params, digests types.DigestSlice) (bool, interface{}) { return true, nil }
	}

	digestCountsByTrace := idx.DigestCountsByTrace(q.IgnoreState())
	lastTraceIdx, traceView, err := getTraceViewFn(selectedTile, q.FCommitBegin, q.FCommitEnd)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Iterate through the tile.
	for id, trace := range selectedTile.Traces {
		// Check if the query matches.
		if tiling.Matches(trace, q.TraceValues) {
			fullTr := trace.(*types.GoldenTrace)
			params := fullTr.Params()
			reducedTr := traceView(fullTr)
			digests := digestsFromTrace(id, reducedTr, q.Head, lastTraceIdx, digestCountsByTrace)

			// If there is an acceptFn defined then check whether
			// we should include this trace.
			ok, acceptRet := acceptFn(params, digests)
			if !ok {
				continue
			}

			// Iterate over the digests and filter them.
			test := fullTr.TestName()
			for _, digest := range digests {
				cl := exp.Classification(test, digest)
				if q.ExcludesClassification(cl) {
					continue
				}

				// Fix blamer to make this easier.
				if q.BlameGroupID != "" {
					if cl == expectations.Untriaged {
						b := idx.GetBlame(test, digest, selectedTile.Commits)
						if b.IsEmpty() || q.BlameGroupID != blameGroupID(b, selectedTile.Commits) {
							continue
						}
					} else {
						continue
					}
				}

				// Add the digest to the results
				addFn(test, digest, id, fullTr, acceptRet)
			}
		}
	}
	return nil
}

// traceViewFn returns a view of a trace that contains a subset of values but the same params.
type traceViewFn func(*types.GoldenTrace) *types.GoldenTrace

// traceViewIdentity is a no-op traceViewFn that returns the exact trace that it
// receives. This is used when no commit range is provided in the query.
func traceViewIdentity(tr *types.GoldenTrace) *types.GoldenTrace {
	return tr
}

// getTraceViewFn returns a traceViewFn for the given Git hashes as well as the
// index of the last value in the resulting traces.
// If startHash occurs after endHash in the tile, an error is returned.
func getTraceViewFn(tile *tiling.Tile, startHash, endHash string) (int, traceViewFn, error) {
	if startHash == "" && endHash == "" {
		return tile.LastCommitIndex(), traceViewIdentity, nil
	}

	// Find the indices to slice the values of the trace.
	startIdx, _ := tiling.FindCommit(tile.Commits, startHash)
	endIdx, _ := tiling.FindCommit(tile.Commits, endHash)
	if (startIdx == -1) && (endIdx == -1) {
		return tile.LastCommitIndex(), traceViewIdentity, nil
	}

	// If either was not found set it to the beginning/end.
	if startIdx == -1 {
		startIdx = 0
	} else if endIdx == -1 {
		endIdx = tile.LastCommitIndex()
	}

	// Increment the last index for the slice operation in the function below.
	endIdx++
	if startIdx >= endIdx {
		return 0, nil, skerr.Fmt("Start commit occurs later than end commit.")
	}

	ret := func(trace *types.GoldenTrace) *types.GoldenTrace {
		return &types.GoldenTrace{
			Digests: trace.Digests[startIdx:endIdx],
			Keys:    trace.Params(),
		}
	}

	return (endIdx - startIdx) - 1, ret, nil
}

// digestsFromTrace returns all the digests in the given trace, controlled by
// 'head', and being robust to tallies not having been calculated for the
// trace.
func digestsFromTrace(id tiling.TraceId, tr *types.GoldenTrace, head bool, lastCommitIndex int, digestsByTrace map[tiling.TraceId]digest_counter.DigestCount) types.DigestSlice {
	digests := types.DigestSet{}
	if head {
		// Find the last non-missing value in the trace.
		for i := lastCommitIndex; i >= 0; i-- {
			if tr.IsMissing(i) {
				continue
			} else {
				digests[tr.Digests[i]] = true
				break
			}
		}
	} else {
		// Use the digestsByTrace if available, otherwise just inspect the trace.
		if t, ok := digestsByTrace[id]; ok {
			for k := range t {
				digests[k] = true
			}
		} else {
			for i := lastCommitIndex; i >= 0; i-- {
				if !tr.IsMissing(i) {
					digests[tr.Digests[i]] = true
				}
			}
		}
	}

	return digests.Keys()
}

// blameGroupID takes a blame distribution with just indices of commits and
// returns an id for the blame group, which is just a string, the concatenated
// git hashes in commit time order.
func blameGroupID(b blame.BlameDistribution, commits []*tiling.Commit) string {
	ret := []string{}
	for _, index := range b.Freq {
		ret = append(ret, commits[index].Hash)
	}
	return strings.Join(ret, ":")
}

// TODO(kjlubick): The whole srDigestSlice might be able to be replaced
// with a sort.Slice() call.
// srDigestSlice is a utility type for sorting slices of frontend.SRDigest by their max diff.
type srDigestSliceLessFn func(i, j *frontend.SRDigest) bool
type srDigestSlice struct {
	slice  []*frontend.SRDigest
	lessFn srDigestSliceLessFn
}

// newSRDigestSlice creates a new instance of srDigestSlice that wraps around
// a slice of result digests.
func newSRDigestSlice(metric string, slice []*frontend.SRDigest) *srDigestSlice {
	// Sort by increasing by diff metric. Not having a diff metric puts the item at the bottom
	// of the list.
	lessFn := func(i, j *frontend.SRDigest) bool {
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

// srIntermediate is the intermediate representation of a single digest
// found by the search. It is used to avoid multiple passes through the tile
// by accumulating the parameters that generated a specific digest and by
// capturing the traces.
type srIntermediate struct {
	test   types.TestName
	digest types.Digest
	traces map[tiling.TraceId]*types.GoldenTrace
	params paramtools.ParamSet
}

// newSrIntermediate creates a new srIntermediate for a digest and adds
// the given trace to it.
func newSrIntermediate(test types.TestName, digest types.Digest, traceID tiling.TraceId, trace tiling.Trace, pset paramtools.ParamSet) *srIntermediate {
	ret := &srIntermediate{
		test:   test,
		digest: digest,
		params: paramtools.ParamSet{},
		traces: map[tiling.TraceId]*types.GoldenTrace{},
	}
	ret.add(traceID, trace, pset)
	return ret
}

// add adds a new trace to an existing intermediate value for a digest
// found in search. If traceID or trace are "" or nil they will not be added.
// 'params' will always be added to the internal parameter set.
func (s *srIntermediate) add(traceID tiling.TraceId, trace tiling.Trace, pset paramtools.ParamSet) {
	if (traceID != "") && (trace != nil) {
		s.traces[traceID] = trace.(*types.GoldenTrace)
		s.params.AddParams(trace.Params())
	} else {
		s.params.AddParamSet(pset)
	}
}

// srInterMap maps [testName][Digest] to an srIntermediate instance that
// aggregates values during a search.
type srInterMap map[types.TestName]map[types.Digest]*srIntermediate

// Add adds the paramset associated with the given test and digest to the srInterMap instance.
func (sm srInterMap) Add(test types.TestName, digest types.Digest, traceID tiling.TraceId, trace *types.GoldenTrace, pset paramtools.ParamSet) {
	if testMap, ok := sm[test]; !ok {
		sm[test] = map[types.Digest]*srIntermediate{digest: newSrIntermediate(test, digest, traceID, trace, pset)}
	} else if entry, ok := testMap[digest]; !ok {
		testMap[digest] = newSrIntermediate(test, digest, traceID, trace, pset)
	} else {
		entry.add(traceID, trace, pset)
	}
}

// AddTestParams adds the params associated with the given test and digest to the srInterMap instance.
func (sm srInterMap) AddTestParams(test types.TestName, digest types.Digest, params paramtools.Params) {
	if testMap, ok := sm[test]; !ok {
		ns := &srIntermediate{
			test:   test,
			digest: digest,
			params: paramtools.ParamSet{},
			traces: map[tiling.TraceId]*types.GoldenTrace{},
		}
		ns.params.AddParams(params)
		sm[test] = map[types.Digest]*srIntermediate{
			digest: ns,
		}
	} else if entry, ok := testMap[digest]; !ok {
		ns := &srIntermediate{
			test:   test,
			digest: digest,
			params: paramtools.ParamSet{},
			traces: map[tiling.TraceId]*types.GoldenTrace{},
		}
		ns.params.AddParams(params)
		testMap[digest] = ns
	} else {
		entry.params.AddParams(params)
	}
}
