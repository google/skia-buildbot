package indexer

import (
	"net/url"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/summary"
	"go.skia.org/infra/golden/go/types"
)

type IndexSource interface {
	// GetIndex returns an IndexSearcher, which can be considered immutable (the underlying
	// Tile won't change). It should be used to handle an entire request to provide
	// consistent information.
	GetIndex() IndexSearcher
}

type IndexSearcher interface {
	// Tile returns the current complex tile from which simpler tiles, like one without ignored
	// traces, can be retrieved
	Tile() types.ComplexTile

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
	DigestCountsByQuery(query url.Values, is types.IgnoreState) digest_counter.DigestCount

	// GetSummaries returns all summaries that were computed for this index.
	GetSummaries(is types.IgnoreState) []*summary.TriageStatus

	// CalcSummaries returns those summaries that match the given inputs. They may
	// be filtered by any of: query, is at head or not.
	CalcSummaries(query url.Values, is types.IgnoreState, head bool) ([]*summary.TriageStatus, error)

	// GetParamsetSummary Returns the ParamSetSummary that matches the given test/digest.
	GetParamsetSummary(test types.TestName, digest types.Digest, is types.IgnoreState) paramtools.ParamSet

	// GetParamsetSummaryByTest returns all ParamSetSummaries in this tile grouped by test name.
	GetParamsetSummaryByTest(is types.IgnoreState) map[types.TestName]map[types.Digest]paramtools.ParamSet

	// GetBlame returns the blame computed for the given test/digest.
	GetBlame(test types.TestName, digest types.Digest, commits []*tiling.Commit) blame.BlameDistribution
}
