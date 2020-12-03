package indexer

import (
	"context"
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/summary"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/types"
)

type IndexSource interface {
	// GetIndex returns an IndexSearcher, which can be considered immutable (the underlying
	// Tile won't change). It should be used to handle an entire request to provide
	// consistent information.
	GetIndex() IndexSearcher

	// GetIndexForCL returns an index object for a given Changelist.
	GetIndexForCL(crs, clID string) *ChangelistIndex
}

type IndexSearcher interface {
	// Tile returns the current complex tile from which simpler tiles, like one without ignored
	// traces, can be retrieved
	Tile() tiling.ComplexTile

	// GetIgnoreMatcher returns a matcher for the ignore rules that were used to
	// build the tile with ignores.
	GetIgnoreMatcher() paramtools.ParamMatcher

	// DigestCountsByTest returns the counts of digests grouped by test name.
	DigestCountsByTest(is types.IgnoreState) map[types.TestName]digest_counter.DigestCount

	// MaxDigestsByTest returns the digests per test that were seen the most.
	MaxDigestsByTest(is types.IgnoreState) map[types.TestName]types.DigestSet

	// DigestCountsByTrace returns the counts of digests grouped by trace id.
	DigestCountsByTrace(is types.IgnoreState) map[tiling.TraceID]digest_counter.DigestCount

	// DigestCountsByQuery returns a DigestCount of all the digests that match the given query.
	DigestCountsByQuery(query paramtools.ParamSet, is types.IgnoreState) digest_counter.DigestCount

	// GetSummaries returns all summaries that were computed for this index.
	GetSummaries(is types.IgnoreState) []*summary.TriageStatus

	// SummarizeByGrouping returns those summaries from a given corpus that match the given inputs.
	// They may be filtered by any of: query, is at head or not.
	SummarizeByGrouping(ctx context.Context, corpus string, query paramtools.ParamSet, is types.IgnoreState, head bool) ([]*summary.TriageStatus, error)

	// GetParamsetSummary Returns the ParamSetSummary that matches the given test/digest.
	GetParamsetSummary(test types.TestName, digest types.Digest, is types.IgnoreState) paramtools.ParamSet

	// GetParamsetSummaryByTest returns all ParamSetSummaries in this tile grouped by test name.
	GetParamsetSummaryByTest(is types.IgnoreState) map[types.TestName]map[types.Digest]paramtools.ParamSet

	// GetBlame returns the blame computed for the given test/digest.
	GetBlame(test types.TestName, digest types.Digest, commits []tiling.Commit) blame.BlameDistribution

	// SlicedTraces returns a slice of TracePairs that match the query and the ignore state.
	// This is meant to be a partial slice, as only the corpus and testname from the query are
	// used to create the subslice.
	SlicedTraces(is types.IgnoreState, query map[string][]string) []*tiling.TracePair

	// MostRecentPositiveDigest returns the most recent positive digest for the given trace.
	MostRecentPositiveDigest(ctx context.Context, traceID tiling.TraceID) (types.Digest, error)
}

// ChangelistIndex is an index about data seen for the most recent Patchset of a given Changelist.
// We only keep the most recent Patchset around because that's the data that is most likely to
// be searched by a user.
type ChangelistIndex struct {
	// LatestPatchset is the most recent Patchset that was seen for this CL (and that the rest of
	// the data in the index belongs to).
	LatestPatchset tjstore.CombinedPSID
	// UntriagedResults is a map of all results that were untriaged the last time the index was built.
	UntriagedResults []tjstore.TryJobResult

	// ParamSet is a set of all keys seen in trace data for this CL, as well as any values associated
	// with those keys. A best effort is made to include Params from all Patchsets on this CL.
	ParamSet paramtools.ParamSet

	// ComputedTS is when this index was created. This helps clients determine how fresh the data is.
	ComputedTS time.Time
}

// Copy returns a deep copy of the ChangelistIndex.
func (c *ChangelistIndex) Copy() *ChangelistIndex {
	rv := &ChangelistIndex{
		LatestPatchset:   c.LatestPatchset,
		ComputedTS:       c.ComputedTS,
		UntriagedResults: make([]tjstore.TryJobResult, len(c.UntriagedResults)),
		ParamSet:         c.ParamSet.Copy(),
	}
	copy(rv.UntriagedResults, c.UntriagedResults)
	return rv
}
