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
	d, diff, rgba := ClosestDigest("foo", "fff", exp, diffStore)
	assert.Equal(t, 0.1, diff)
	assert.Equal(t, "eee", d)
	assert.Equal(t, []int{5, 3, 4, 0}, rgba)

	// Now test against a test with no positive digests.
	d, diff, rgba = ClosestDigest("bar", "fff", exp, diffStore)
	assert.Equal(t, float32(math.MaxFloat32), diff)
	assert.Equal(t, "", d)
	assert.Equal(t, []int{}, rgba)
}
