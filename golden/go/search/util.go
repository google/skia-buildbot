// Package search contains the core functionality for searching for digests across a tile.
package search

import (
	"context"
	"strings"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

// iterTileAcceptFn is the callback function used by iterTile to determine whether to
// include a trace into the result or not.
// It must be safe to be called from multiple goroutines.
type iterTileAcceptFn func(params paramtools.Params, digests types.DigestSlice) bool

// iterTileAddFn is the callback function used by iterTile to add a digest and its trace to the
// result. It must be safe to be called from multiple goroutines.
type iterTileAddFn func(test types.TestName, digest types.Digest, traceID tiling.TraceID, trace *tiling.Trace)

// iterTile is a generic function to extract information from a tile.
// It iterates over the tile and filters against the given query. If calls
// iterTileAcceptFn to determine whether to keep a trace (after it has already been
// tested against the query) and calls iterTileAddFn to add a digest and its trace.
// iterTileAcceptFn == nil equals unconditional acceptance.
func iterTile(ctx context.Context, q *query.Search, addFn iterTileAddFn, acceptFn iterTileAcceptFn, exp expectations.Classifier, idx indexer.IndexSearcher) error {
	if acceptFn == nil {
		acceptFn = func(params paramtools.Params, digests types.DigestSlice) bool { return true }
	}
	cpxTile := idx.Tile()
	traceView, err := getTraceViewFn(cpxTile.DataCommits(), q.CommitBeginFilter, q.CommitEndFilter)
	if err != nil {
		return skerr.Wrap(err)
	}
	// traces is pre-sliced by corpus and test name, if provided.
	traces := idx.SlicedTraces(q.IgnoreState(), q.TraceValues)
	digestCountsByTrace := idx.DigestCountsByTrace(q.IgnoreState())

	const numChunks = 4 // arbitrarily picked, could likely be tuned based on contention of the
	// mutexes in iterTileAddFn/iterTileAcceptFn
	chunkSize := (len(traces) / numChunks) + 1 // add one to avoid integer truncation.

	// Iterate through the tile in parallel.
	return util.ChunkIterParallel(ctx, len(traces), chunkSize, func(ctx context.Context, startIdx int, endIdx int) error {
		for _, tp := range traces[startIdx:endIdx] {
			if err := ctx.Err(); err != nil {
				return skerr.Wrap(err)
			}
			id, trace := tp.ID, tp.Trace
			// Check if the query matches.
			if trace.Matches(q.TraceValues) {
				params := trace.KeysAndOptions()
				reducedTr := traceView(trace)
				digests := digestsFromTrace(id, reducedTr, q.OnlyIncludeDigestsProducedAtHead, digestCountsByTrace)

				// If there is an iterTileAcceptFn defined then check whether
				// we should include this trace.
				ok := acceptFn(params, digests)
				if !ok {
					continue
				}

				// Iterate over the digests and filter them.
				test := trace.TestName()
				for _, digest := range digests {
					cl := exp.Classification(test, digest)
					if q.ExcludesClassification(cl) {
						continue
					}

					// Fix blamer to make this easier.
					if q.BlameGroupID != "" {
						if cl == expectations.UntriagedStr {
							b := idx.GetBlame(test, digest, cpxTile.DataCommits())
							if b.IsEmpty() || q.BlameGroupID != blameGroupID(b, cpxTile.DataCommits()) {
								continue
							}
						} else {
							continue
						}
					}

					// Add the digest to the results
					addFn(test, digest, id, trace)
				}
			}
		}
		return nil
	})
}

// traceViewFn returns a view of a trace that contains a subset of values but the same params.
type traceViewFn func(*tiling.Trace) *tiling.Trace

// traceViewIdentity is a no-op traceViewFn that returns the exact trace that it
// receives. This is used when no commit range is provided in the query.
func traceViewIdentity(tr *tiling.Trace) *tiling.Trace {
	return tr
}

// getTraceViewFn returns a traceViewFn for the given Git hashes.
// If startHash occurs after endHash in the tile, an error is returned.
func getTraceViewFn(commits []tiling.Commit, startHash, endHash string) (traceViewFn, error) {
	if startHash == "" && endHash == "" {
		return traceViewIdentity, nil
	}

	// Find the indices to slice the values of the trace.
	startIdx, _ := findCommit(commits, startHash)
	endIdx, _ := findCommit(commits, endHash)
	if (startIdx == -1) && (endIdx == -1) {
		return traceViewIdentity, nil
	}

	// If either was not found set it to the beginning/end.
	if startIdx == -1 {
		startIdx = 0
	} else if endIdx == -1 {
		endIdx = len(commits) - 1
	}

	// Increment the last index for the slice operation in the function below.
	endIdx++
	if startIdx >= endIdx {
		return nil, skerr.Fmt("Start commit occurs later than end commit.")
	}

	ret := func(trace *tiling.Trace) *tiling.Trace {
		return tiling.NewTrace(trace.Digests[startIdx:endIdx], trace.Keys(), trace.Options())
	}
	return ret, nil
}

// findCommit searches the given commits for the given hash and returns the
// index of the commit and the commit itself. If the commit cannot be
// found -1 is returned for index.
func findCommit(commits []tiling.Commit, targetHash string) (int, tiling.Commit) {
	if targetHash == "" {
		return -1, tiling.Commit{}
	}
	for idx, commit := range commits {
		if commit.Hash == targetHash {
			return idx, commit
		}
	}
	return -1, tiling.Commit{}
}

// digestsFromTrace returns all the digests in the given trace, controlled by
// 'head', and being robust to tallies not having been calculated for the
// trace.
func digestsFromTrace(id tiling.TraceID, tr *tiling.Trace, head bool, digestsByTrace map[tiling.TraceID]digest_counter.DigestCount) types.DigestSlice {
	digests := types.DigestSet{}
	if head {
		// Find the last non-missing value in the trace.
		for i := tr.Len() - 1; i >= 0; i-- {
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
			for i := tr.Len() - 1; i >= 0; i-- {
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
func blameGroupID(b blame.BlameDistribution, commits []tiling.Commit) string {
	var ret []string
	for _, index := range b.Freq {
		ret = append(ret, commits[index].Hash)
	}
	return strings.Join(ret, ":")
}

// TODO(kjlubick): The whole srDigestSlice might be able to be replaced
// with a sort.Slice() call.
// srDigestSlice is a utility type for sorting slices of frontend.SearchResult by their max diff.
type srDigestSliceLessFn func(i, j *frontend.SearchResult) bool
type srDigestSlice struct {
	slice  []*frontend.SearchResult
	lessFn srDigestSliceLessFn
}

// newSRDigestSlice creates a new instance of srDigestSlice that wraps around
// a slice of result digests.
func newSRDigestSlice(metric string, slice []*frontend.SearchResult) *srDigestSlice {
	// Sort by increasing by diff metric. Not having a diff metric puts the item at the bottom
	// of the list.
	lessFn := func(i, j *frontend.SearchResult) bool {
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

type triageHistoryGetter interface {
	GetTriageHistory(ctx context.Context, grouping types.TestName, digest types.Digest) ([]expectations.TriageHistory, error)
}

type joinedHistories struct {
	masterBranch expectations.Store
	changeList   expectations.Store
}

// GetTriageHistory returns a combined history from both the master branch and the changelist,
// if configured for one.
func (j *joinedHistories) GetTriageHistory(ctx context.Context, grouping types.TestName, digest types.Digest) ([]expectations.TriageHistory, error) {
	var history []expectations.TriageHistory
	if j.changeList != nil {
		xth, err := j.changeList.GetTriageHistory(ctx, grouping, digest)
		if err != nil {
			return nil, skerr.Wrapf(err, "Looking in CL history")
		}
		history = xth
	}
	xth, err := j.masterBranch.GetTriageHistory(ctx, grouping, digest)
	if err != nil {
		return nil, skerr.Wrapf(err, "Looking in master branch history")
	}
	if len(xth) > 0 {
		history = append(history, xth...)
	}
	return history, nil
}

// makeTriageHistoryGetter will return either a view of the master branch triage history or a
// combined view of the given changeList's triage history and the master branch's.
func (s *SearchImpl) makeTriageHistoryGetter(crs, clID string) triageHistoryGetter {
	if crs == "" || clID == "" {
		return &joinedHistories{masterBranch: s.expectationsStore}
	}
	return &joinedHistories{
		masterBranch: s.expectationsStore,
		changeList:   s.expectationsStore.ForChangeList(clID, crs),
	}
}
