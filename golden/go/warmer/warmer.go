// warmer makes sure we've pre-warmed the diffcache for normal queries.
// TODO(kjlubick): can this be merged into digesttools?
package warmer

import (
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/digesttools"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/summary"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

// DiffWarmer pre-calculates diffs that will be requested by the front-end
// when showing diffs for a certain page.
type DiffWarmer interface {
	// PrecomputeDiffs goes through all untriaged digests and precomputes their
	// closest positive and closest negative digest. This will make sure diffstore
	// has those diffs pre-drawn and can serve them quickly to the frontend.
	// If testNames is not empty, only those the diffs for those names will be
	// precomputed.
	PrecomputeDiffs(summaries summary.SummaryMap, testNames types.TestNameSet, dCounter digest_counter.DigestCounter, diffFinder digesttools.ClosestDiffFinder)
}

type WarmerImpl struct {
}

// New creates an new instance of WarmerImpl.
func New() *WarmerImpl {
	return &WarmerImpl{}
}

// PrecomputeDiffs implements the DiffWarmer interface
func (w *WarmerImpl) PrecomputeDiffs(summaries summary.SummaryMap, testNames types.TestNameSet, dCounter digest_counter.DigestCounter, diffFinder digesttools.ClosestDiffFinder) {
	t := shared.NewMetricsTimer("warmer_loop")
	diffFinder.Precompute()
	for test, sum := range summaries {
		if len(testNames) > 0 && !testNames[test] {
			// Skipping this test because it wasn't in the set of tests to precompute.
			continue
		}
		for _, digest := range sum.UntHashes {
			// Only pre-compute those diffs for the test_name+digest pair if it was observed
			t := dCounter.ByTest()[test]
			if t != nil {
				// Calculating the closest digest has the side effect of filling
				// in the diffstore with the diff images.
				diffFinder.ClosestDigest(test, digest, expectations.Positive)
				diffFinder.ClosestDigest(test, digest, expectations.Negative)
			}
		}
	}
	t.Stop()

	// TODO(stephana): Add warmer for Tryjob digests.
}

// Make sure WarmerImpl fulfills the DiffWarmer interface
var _ DiffWarmer = (*WarmerImpl)(nil)
