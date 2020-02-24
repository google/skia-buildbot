// warmer makes sure we've pre-warmed the diffcache for normal queries.
// TODO(kjlubick): can this be merged into digesttools?
package warmer

import (
	"context"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/digesttools"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/summary"
	"go.skia.org/infra/golden/go/types"
)

// Data contains what is is necessary to precompute the diffs.
type Data struct {
	// TestSummaries summarize the untriaged digests for a given test.
	TestSummaries []*summary.TriageStatus

	// DigestsByTest maps test names to the digests they produce.
	DigestsByTest map[types.TestName]digest_counter.DigestCount

	// SubsetOfTests is the subset of tests to warm the digest for. If empty,
	// all will be warmed.
	SubsetOfTests types.TestNameSet
}

// DiffWarmer pre-calculates diffs that will be requested by the front-end
// when showing diffs for a certain page.
type DiffWarmer interface {
	// PrecomputeDiffs goes through all untriaged digests and precomputes their
	// closest positive and closest negative digest. This will make sure diffstore
	// has those diffs pre-drawn and can serve them quickly to the frontend.
	// If testNames is not empty, only those the diffs for those names will be
	// precomputed.
	PrecomputeDiffs(ctx context.Context, d Data, diffFinder digesttools.ClosestDiffFinder) error
}

type WarmerImpl struct {
}

// New creates an new instance of WarmerImpl.
func New() *WarmerImpl {
	return &WarmerImpl{}
}

// PrecomputeDiffs implements the DiffWarmer interface
func (w *WarmerImpl) PrecomputeDiffs(ctx context.Context, d Data, diffFinder digesttools.ClosestDiffFinder) error {
	defer shared.NewMetricsTimer("warmer_loop").Stop()
	err := diffFinder.Precompute(ctx)
	if err != nil {
		return skerr.Wrapf(err, "preparing to compute diffs")
	}
	// We go through all the diffs to precompute. If there was a flake or something and we get an
	// error from the diffFinder, we keep going to try to warm as much as possible (unless our
	// context signals us to stop).
	var firstErr error
	errCount := 0
	for _, sum := range d.TestSummaries {
		test := sum.Name
		if ctx.Err() != nil {
			sklog.Warningf("PrecomputeDiffs stopped by context error: %s", ctx.Err())
			break
		}
		if len(d.SubsetOfTests) > 0 && !d.SubsetOfTests[test] {
			// Skipping this test because it wasn't in the set of tests to precompute.
			continue
		}
		for _, digest := range sum.UntHashes {
			// Only pre-compute those diffs for the test_name+digest pair if it was observed
			t := d.DigestsByTest[test]
			if t != nil {
				nt := metrics2.NewTimer("gold_warmer_cycle")
				// Calculating the closest digest has the side effect of filling
				// in the diffstore with the diff images.
				_, err := diffFinder.ClosestDigest(ctx, test, digest, expectations.Positive)
				if err != nil {
					if firstErr == nil {
						firstErr = err
					}
					sklog.Debugf("non-terminating error precomputing diff: %s", err)
					errCount++
				}
				_, err = diffFinder.ClosestDigest(ctx, test, digest, expectations.Negative)
				if err != nil {
					if firstErr == nil {
						firstErr = err
					}
					sklog.Debugf("non-terminating error precomputing diff: %s", err)
					errCount++
				}
				nt.Stop()
			}
		}
	}
	if errCount > 0 {
		return skerr.Wrapf(firstErr, "and %d other error(s) precomputing diffs", errCount-1)
	}
	return ctx.Err()
}

// Make sure WarmerImpl fulfills the DiffWarmer interface
var _ DiffWarmer = (*WarmerImpl)(nil)
