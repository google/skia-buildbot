// warmer makes sure we've pre-warmed the diffcache for normal queries.
// TODO(kjlubick): can this be merged into digesttools?
package warmer

import (
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/digesttools"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/summary"
	"go.skia.org/infra/golden/go/types"
)

// Warmer pre-calculates diffs that will be requested by the front-end
// when showing diffs for a certain page.
type Warmer struct {
}

// New creates an new instance of Warmer.
func New() *Warmer {
	return &Warmer{}
}

// PrecomputeDiffs goes through all untriaged digests and precomputes their
// closest positive and closest negative digest. This will make sure diffstore
// has those diffs pre-drawn and can serve them quickly to the frontend.
func (w *Warmer) PrecomputeDiffs(summaries *summary.Summaries, dCounter digest_counter.DigestCounter, diffFinder digesttools.ClosestDiffFinder) {
	t := shared.NewMetricsTimer("warmer_loop")
	diffFinder.Precompute()
	for test, sum := range summaries.Get() {
		for _, digest := range sum.UntHashes {
			// Only pre-compute those diffs for the test_name+digest pair if it was observed
			t := dCounter.ByTest()[test]
			if t != nil {
				// Calculating the closest digest has the side effect of filling
				// in the diffstore with the diff images.
				diffFinder.ClosestDigest(test, digest, types.POSITIVE)
				diffFinder.ClosestDigest(test, digest, types.NEGATIVE)
			}
		}
	}
	t.Stop()

	// TODO(stephana): Add warmer for Tryjob digests.
}
