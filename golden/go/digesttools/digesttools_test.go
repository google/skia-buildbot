package digesttools

import (
	"math"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/types"
)

type MockDiffStore struct{}

func (m MockDiffStore) ImageHandler(urlPrefix string) (http.Handler, error)                   { return nil, nil }
func (m MockDiffStore) WarmDigests(priority int64, digests []string, sync bool)               {}
func (m MockDiffStore) WarmDiffs(priority int64, leftDigests []string, rightDigests []string) {}
func (m MockDiffStore) UnavailableDigests() map[string]*diff.DigestFailure                    { return nil }
func (m MockDiffStore) PurgeDigests(digests []string, purgeGCS bool) error                    { return nil }

// Get always finds that digest "eee" is closest to dMain.
func (m MockDiffStore) Get(priority int64, dMain string, dRest []string) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	for i, d := range dRest {
		diffPercent := float32(i + 2)
		if d == "eee" {
			diffPercent = 0.1
		}
		result[d] = &diff.DiffMetrics{
			PixelDiffPercent: diffPercent,
			MaxRGBADiffs:     []int{5, 3, 4, 0},
		}
	}
	return result, nil
}

func TestClosestDigest(t *testing.T) {
	testutils.SmallTest(t)
	diffStore := MockDiffStore{}
	testExp := types.TestExp{
		"foo": map[string]types.Label{
			"aaa": types.POSITIVE,
			"bbb": types.NEGATIVE,
			"ccc": types.UNTRIAGED,
			"ddd": types.UNTRIAGED,
			"eee": types.POSITIVE,
			"ggg": types.POSITIVE,
		},
	}
	tallies := tally.Tally{
		"aaa": 2,
		"bbb": 2,
		"ccc": 2,
		"ddd": 2,
		"eee": 2,
	}
	exp := types.NewTestExpBuilder(testExp)

	// First test against a test that has positive digests.
	c := ClosestDigest("foo", "fff", exp, tallies, diffStore, types.POSITIVE)
	assert.InDelta(t, 0.0372, float64(c.Diff), 0.01)
	assert.Equal(t, "eee", c.Digest)
	assert.Equal(t, []int{5, 3, 4, 0}, c.MaxRGBA)

	// Now test against a test with no positive digests.
	c = ClosestDigest("bar", "fff", exp, tallies, diffStore, types.POSITIVE)
	assert.Equal(t, float32(math.MaxFloat32), c.Diff)
	assert.Equal(t, "", c.Digest)
	assert.Equal(t, []int{}, c.MaxRGBA)

	// Now test against negative digests.
	c = ClosestDigest("foo", "fff", exp, tallies, diffStore, types.NEGATIVE)
	assert.InDelta(t, 0.166, float64(c.Diff), 0.01)
	assert.Equal(t, "bbb", c.Digest)
	assert.Equal(t, []int{5, 3, 4, 0}, c.MaxRGBA)
}

func TestCombinedDiffMetric(t *testing.T) {
	testutils.SmallTest(t)
	assert.InDelta(t, 1.0, combinedDiffMetric(0.0, []int{}), 0.000001)
	assert.InDelta(t, 1.0, combinedDiffMetric(1.0, []int{255, 255, 255, 255}), 0.000001)
	assert.InDelta(t, math.Sqrt(0.5), combinedDiffMetric(0.5, []int{255, 255, 255, 255}), 0.000001)
}
