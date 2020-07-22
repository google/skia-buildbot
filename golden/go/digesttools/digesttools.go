// digesttools are utility functions for answering questions about digests.
package digesttools

import (
	"context"
	"math"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/types"
)

// ClosestDiffFinder is able to query a diffstore for digests
// that are the closest negative or closest positive to
// a given digest. Implementations should be considered
// not-thread-safe.
type ClosestDiffFinder interface {
	// Precompute allows the implementation to warm any caches.
	// Call before doing a batch of ClosestDigest calls.
	Precompute(ctx context.Context) error

	// ClosestDigest returns the closest digest of type 'label' to 'digest',
	// or NoDigestFound if there aren't any positive digests.
	//
	// If no digest of type 'label' is found then Closest.Digest is the empty string.
	ClosestDigest(ctx context.Context, test types.TestName, digest types.Digest, label expectations.Label) (*Closest, error)
}

// Closest describes one digest that is the closest another digest.
type Closest struct {
	// The closest digest, empty if there are no digests to compare to.
	Digest     types.Digest `json:"digest"`
	Diff       float32      `json:"diff"`       // A percent value.
	DiffPixels float32      `json:"diffPixels"` // A percent value.
	MaxRGBA    [4]int       `json:"maxRGBA"`
}

const NoDigestFound = types.Digest("")

// newClosest returns an initialized Closest struct, defaulting to
// NoDigestFound and related values.
func newClosest() *Closest {
	return &Closest{
		Digest:     NoDigestFound,
		Diff:       math.MaxFloat32,
		DiffPixels: math.MaxFloat32,
		MaxRGBA:    [4]int{},
	}
}

// Impl implements the ClosestDiffFinder interface
type Impl struct {
	expectations expectations.ReadOnly
	dCounter     digest_counter.DigestCounter
	diffStore    diff.DiffStore
}

// NewClosestDiffFinder returns a *Impl loaded with the given data sources.
func NewClosestDiffFinder(exp expectations.ReadOnly, dCounter digest_counter.DigestCounter, diffStore diff.DiffStore) *Impl {
	return &Impl{
		expectations: exp,
		dCounter:     dCounter,
		diffStore:    diffStore,
	}
}

// Precompute implements the ClosestDiffFinder interface.
func (i *Impl) Precompute(ctx context.Context) error {
	return nil
}

// ClosestDigest implements the ClosestDiffFinder interface.
func (i *Impl) ClosestDigest(ctx context.Context, test types.TestName, digest types.Digest, label expectations.Label) (*Closest, error) {
	ret := newClosest()

	// Locate all digests that this test produces and match the given label.
	selected := types.DigestSlice{}
	testDigests := i.dCounter.ByTest()[test]
	for d := range testDigests {
		if i.expectations.Classification(test, d) == label {
			selected = append(selected, d)
		}
	}

	if len(selected) == 0 {
		return ret, nil
	}

	if diffMetrics, err := i.diffStore.Get(ctx, digest, selected); err != nil {
		return nil, skerr.Wrapf(err, "getting diffs for %s and %d comparisons", digest, len(selected))
	} else {
		for digest, dm := range diffMetrics {
			if delta := diff.CombinedDiffMetric(dm, nil, nil); delta < ret.Diff {
				ret.Digest = digest
				ret.Diff = delta
				ret.DiffPixels = dm.PixelDiffPercent
				ret.MaxRGBA = dm.MaxRGBADiffs
			}
		}
		return ret, nil
	}
}

// Make sure Impl fulfills the ClosestDiffFinder interface
var _ ClosestDiffFinder = (*Impl)(nil)
