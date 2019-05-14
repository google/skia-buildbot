// digesttools are utility functions for answering questions about digests.
package digesttools

import (
	"math"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/types"
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
	ClosestDigest(test types.TestName, digest types.Digest, label types.Label) *Closest
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
	expectations types.Expectations
	dCounter     digest_counter.DigestCounter
	diffStore    diff.DiffStore

	cachedUnavailableDigests map[types.Digest]*diff.DigestFailure
}

// NewClosestDiffFinder returns a *Impl loaded with the given data sources.
func NewClosestDiffFinder(exp types.Expectations, dCounter digest_counter.DigestCounter, diffStore diff.DiffStore) *Impl {
	return &Impl{
		expectations: exp,
		dCounter:     dCounter,
		diffStore:    diffStore,
	}
}

// Precompute implements the ClosestDiffFinder interface.
func (i *Impl) Precompute() {
	i.cachedUnavailableDigests = i.diffStore.UnavailableDigests()
}

// ClosestDigest implements the ClosestDiffFinder interface.
func (i *Impl) ClosestDigest(test types.TestName, digest types.Digest, label types.Label) *Closest {
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

	if diffMetrics, err := i.diffStore.Get(diff.PRIORITY_NOW, digest, selected); err != nil {
		sklog.Errorf("ClosestDigest: Failed to get diff: %s", err)
		return ret
	} else {
		for digest, diffs := range diffMetrics {
			dm := diffs.(*diff.DiffMetrics)
			if delta := combinedDiffMetric(dm.PixelDiffPercent, dm.MaxRGBADiffs); delta < ret.Diff {
				ret.Digest = digest
				ret.Diff = delta
				ret.DiffPixels = dm.PixelDiffPercent
				ret.MaxRGBA = dm.MaxRGBADiffs
			}
		}
		return ret
	}
}

// ClosestFromDiffMetrics returns an instance of Closest with the values of the
// given diff.DiffMetrics. The Digest field will be left empty.
func ClosestFromDiffMetrics(diff *diff.DiffMetrics) *Closest {
	return &Closest{
		Diff:       combinedDiffMetric(diff.PixelDiffPercent, diff.MaxRGBADiffs),
		DiffPixels: diff.PixelDiffPercent,
		MaxRGBA:    diff.MaxRGBADiffs,
	}
}

// combinedDiffMetric returns a value in [0, 1] that represents how large
// the diff is between two images.
func combinedDiffMetric(pixelDiffPercent float32, maxRGBA []int) float32 {
	if len(maxRGBA) == 0 {
		return 1.0
	}
	// Turn maxRGBA into a percent by taking the root mean square difference from
	// [0, 0, 0, 0].
	sum := 0.0
	for _, c := range maxRGBA {
		sum += float64(c) * float64(c)
	}
	normalizedRGBA := math.Sqrt(sum/float64(len(maxRGBA))) / 255.0
	// We take the sqrt of (pixelDiffPercent * normalizedRGBA) to straigten out
	// the curve, i.e. think about what a plot of x^2 would look like in the
	// range [0, 1].
	return float32(math.Sqrt(float64(pixelDiffPercent) * normalizedRGBA))
}

// Make sure Impl fulfills the ClosestDiffFinder interface
var _ ClosestDiffFinder = (*Impl)(nil)
