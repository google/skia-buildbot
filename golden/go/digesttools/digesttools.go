// digesttools are utility functions for answering questions about digests.
package digesttools

import (
	"math"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

// ClosestDigest returns the closest positive digest to 'digest', or "" if there aren't any positive digests.
//
// The diff, as a float32 percent, is also returned, along with the maximum
// RGBA channel diffs.  If no positive digest is found it returns 0.0, and
// []int{} for those values.
func ClosestDigest(test string, digest string, exp *expstorage.Expectations, diffStore diff.DiffStore) (string, float32, []int) {
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
		return "", math.MaxFloat32, []int{}
	} else {
		bestDigest := ""
		bestRGBA := []int{}
		var bestDiff float32 = math.MaxFloat32
		for digest, diff := range diffMetrics {
			if diff.PixelDiffPercent < bestDiff {
				bestDigest = digest
				bestDiff = diff.PixelDiffPercent
				bestRGBA = diff.MaxRGBADiffs
			}
		}
		return bestDigest, bestDiff, bestRGBA
	}
}
