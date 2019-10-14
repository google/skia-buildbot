// digesttools are utility functions for answering questions about digests.
package digesttools

import (
	"context"
	"math"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

// ClosestDiffFinder is able to query a diffstore for digests
// that are the closest negative or closest positive to
// a given digest. Implementations should be considered
// not-thread-safe.
type ClosestDiffFinder interface {
	// Precompute allows the implementation to warm any caches.
	// Call before doing a batch of ClosestDigest calls.
	Precompute()

	// ClosestDigest returns the closest digest of type 'label' to 'digest',
	// or NoDigestFound if there aren't any positive digests.
	//
	// If no digest of type 'label' is found then Closest.Digest is the empty string.
	ClosestDigest(test types.TestName, digest types.Digest, label expectations.Label) *Closest
}

// Closest describes one digest that is the closest another digest.
type Closest struct {
	// The closest digest, empty if there are no digests to compare to.
	Digest     types.Digest `json:"digest"`
	Diff       float32      `json:"diff"`       // A percent value.
	DiffPixels float32      `json:"diffPixels"` // A percent value.
	MaxRGBA    []int        `json:"maxRGBA"`
}

const NoDigestFound = types.Digest("")

// newClosest returns an initialized Closest struct, defaulting to
// NoDigestFound and related values.
func newClosest() *Closest {
	return &Closest{
		Digest:     NoDigestFound,
		Diff:       math.MaxFloat32,
		DiffPixels: math.MaxFloat32,
		MaxRGBA:    []int{},
	}
}

// Impl implements the ClosestDiffFinder interface
type Impl struct {
	expectations expectations.Expectations
	dCounter     digest_counter.DigestCounter
	diffStore    diff.DiffStore

	cachedUnavailableDigests map[types.Digest]*diff.DigestFailure
}

// NewClosestDiffFinder returns a *Impl loaded with the given data sources.
func NewClosestDiffFinder(exp expectations.Expectations, dCounter digest_counter.DigestCounter, diffStore diff.DiffStore) *Impl {
	return &Impl{
		expectations: exp,
		dCounter:     dCounter,
		diffStore:    diffStore,
	}
}

// Precompute implements the ClosestDiffFinder interface.
func (i *Impl) Precompute() {
	i.cachedUnavailableDigests = i.diffStore.UnavailableDigests(context.TODO())
}

// ClosestDigest implements the ClosestDiffFinder interface.
func (i *Impl) ClosestDigest(test types.TestName, digest types.Digest, label expectations.Label) *Closest {
	ret := newClosest()

	if _, ok := i.cachedUnavailableDigests[digest]; ok {
		return ret
	}

	// Locate all digests that this test produces and match the given label.
	selected := types.DigestSlice{}
	testDigests := i.dCounter.ByTest()[test]
	for d := range testDigests {
		if _, ok := i.cachedUnavailableDigests[d]; !ok && (i.expectations.Classification(test, d) == label) {
			selected = append(selected, d)
		}
	}

	if len(selected) == 0 {
		return ret
	}

	if diffMetrics, err := i.diffStore.Get(context.TODO(), diff.PRIORITY_NOW, digest, selected); err != nil {
		sklog.Errorf("ClosestDigest: Failed to get diff: %s", err)
		return ret
	} else {
		for digest, dm := range diffMetrics {
			if delta := diff.CombinedDiffMetric(dm, nil, nil); delta < ret.Diff {
				ret.Digest = digest
				ret.Diff = delta
				ret.DiffPixels = dm.PixelDiffPercent
				ret.MaxRGBA = dm.MaxRGBADiffs
			}
		}
		return ret
	}
}

// Make sure Impl fulfills the ClosestDiffFinder interface
var _ ClosestDiffFinder = (*Impl)(nil)
