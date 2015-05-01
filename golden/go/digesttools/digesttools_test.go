package digesttools

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

type MockDiffStore struct{}

func (m MockDiffStore) AbsPath(digest []string) map[string]string { return nil }
func (m MockDiffStore) UnavailableDigests() map[string]bool       { return nil }
func (m MockDiffStore) CalculateDiffs([]string)                   {}

// Get always finds that digest "eee" is closest to dMain.
func (m MockDiffStore) Get(dMain string, dRest []string) (map[string]*diff.DiffMetrics, error) {
	result := map[string]*diff.DiffMetrics{}
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
	diffStore := MockDiffStore{}
	exp := &expstorage.Expectations{
		Tests: map[string]types.TestClassification{
			"foo": map[string]types.Label{
				"aaa": types.POSITIVE,
				"bbb": types.NEGATIVE,
				"ccc": types.UNTRIAGED,
				"ddd": types.UNTRIAGED,
				"eee": types.POSITIVE,
			},
		},
	}

	// First test against a test that has positive digests.
	c := ClosestDigest("foo", "fff", exp, diffStore, types.POSITIVE)
	assert.Equal(t, 0.1, c.Diff)
	assert.Equal(t, "eee", c.Digest)
	assert.Equal(t, []int{5, 3, 4, 0}, c.MaxRGBA)

	// Now test against a test with no positive digests.
	c = ClosestDigest("bar", "fff", exp, diffStore, types.POSITIVE)
	assert.Equal(t, float32(math.MaxFloat32), c.Diff)
	assert.Equal(t, "", c.Digest)
	assert.Equal(t, []int{}, c.MaxRGBA)

	// Now test against negative digests.
	c = ClosestDigest("foo", "fff", exp, diffStore, types.NEGATIVE)
	assert.Equal(t, 2.0, c.Diff)
	assert.Equal(t, "bbb", c.Digest)
	assert.Equal(t, []int{5, 3, 4, 0}, c.MaxRGBA)
}
