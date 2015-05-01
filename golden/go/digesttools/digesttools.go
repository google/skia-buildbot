// digesttools are utility functions for answering questions about digests.
package digesttools

import (
	"math"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

// Closest describes one digest that is the closest another digest.
type Closest struct {
	Digest  string  `json:"digest"` // The closest digest, empty if there are no digests to compare to.
	Diff    float32 `json:"diff"`
	MaxRGBA []int   `json:"maxRGBA"`
}

func newClosest() *Closest {
	return &Closest{
		Diff:    math.MaxFloat32,
		MaxRGBA: []int{},
	}
}

// ClosestDigest returns the closest positive digest to 'digest', or "" if there aren't any positive digests.
//
// If no positive digest is found Closest.Digest is the empty string.
func ClosestDigest(test string, digest string, exp *expstorage.Expectations, diffStore diff.DiffStore) *Closest {
	ret := newClosest()
	positives := []string{}
	if e, ok := exp.Tests[test]; ok {
		for d, label := range e {
			if label == types.POSITIVE {
				positives = append(positives, d)
			}
		}
	}
	if diffMetrics, err := diffStore.Get(digest, positives); err != nil {
		glog.Errorf("ClosestDigest: Failed to get diff: %s", err)
		return ret
	} else {
		for digest, diff := range diffMetrics {
			if diff.PixelDiffPercent < ret.Diff {
				ret.Digest = digest
				ret.Diff = diff.PixelDiffPercent
				ret.MaxRGBA = diff.MaxRGBADiffs
			}
		}
		return ret
	}
}
