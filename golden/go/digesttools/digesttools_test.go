package digesttools

import (
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/types"
)

// TestClosestDigest tests the basic interaction between the DiffFinder
// and DiffStore for finding the closest positive and negative diffs.
func TestClosestDigest(t *testing.T) {
	unittest.SmallTest(t)
	mds := &mocks.DiffStore{}
	mdc := &mocks.DigestCounter{}
	defer mds.AssertExpectations(t)
	defer mdc.AssertExpectations(t)

	exp := types.Expectations{
		mockTest: map[types.Digest]types.Label{
			mockDigestA: types.POSITIVE,
			mockDigestB: types.NEGATIVE,
			mockDigestC: types.UNTRIAGED,
			mockDigestD: types.UNTRIAGED,
			mockDigestF: types.POSITIVE,
			mockDigestG: types.POSITIVE,
		},
	}
	digestCounts := map[types.TestName]digest_counter.DigestCount{
		mockTest: {
			mockDigestA: 2,
			mockDigestB: 2,
			mockDigestC: 2,
			mockDigestD: 2,
			mockDigestE: 2,
		},
	}

	mdc.On("ByTest").Return(digestCounts)
	mds.On("UnavailableDigests").Return(map[types.Digest]*diff.DigestFailure{})

	cdf := NewClosestDiffFinder(exp, mdc, mds)

	cdf.Precompute()

	// Only mockDigestA is both triaged positive and in the digestCounts (meaning, we saw that digest
	// in this tile).
	expectedToCompareAgainst := types.DigestSlice{mockDigestA}
	mds.On("Get", diff.PRIORITY_NOW, mockDigestF, expectedToCompareAgainst).Return(diffEIsClosest(), nil).Once()
	// First test against a test that has positive digests.
	c := cdf.ClosestDigest(mockTest, mockDigestF, types.POSITIVE)
	assert.InDelta(t, 0.0372, float64(c.Diff), 0.01)
	assert.Equal(t, mockDigestE, c.Digest)
	assert.Equal(t, []int{5, 3, 4, 0}, c.MaxRGBA)

	// mockDigestB is the only negative digest that shows up in the tile.
	expectedToCompareAgainst = types.DigestSlice{mockDigestB}
	mds.On("Get", diff.PRIORITY_NOW, mockDigestF, expectedToCompareAgainst).Return(diffBIsClosest(), nil).Once()
	// Now test against negative digests.
	c = cdf.ClosestDigest(mockTest, mockDigestF, types.NEGATIVE)
	assert.InDelta(t, 0.0558, float64(c.Diff), 0.01)
	assert.Equal(t, mockDigestB, c.Digest)
	assert.Equal(t, []int{2, 7, 1, 3}, c.MaxRGBA)
}

// TestClosestDigestWithUnavailable tests some more tricky logic dealing
// with unavailable digests and tests with no digests.
func TestClosestDigestWithUnavailable(t *testing.T) {
	unittest.SmallTest(t)
	mds := &mocks.DiffStore{}
	mdc := &mocks.DigestCounter{}
	defer mds.AssertExpectations(t)
	defer mdc.AssertExpectations(t)

	exp := types.Expectations{
		mockTest: map[types.Digest]types.Label{
			mockDigestA: types.POSITIVE,
			mockDigestB: types.NEGATIVE,
			mockDigestC: types.POSITIVE,
			mockDigestD: types.POSITIVE,
			mockDigestF: types.POSITIVE,
			mockDigestG: types.POSITIVE,
		},
	}
	digestCounts := map[types.TestName]digest_counter.DigestCount{
		mockTest: {
			mockDigestA: 2,
			mockDigestB: 2,
			mockDigestC: 2,
			mockDigestD: 2,
			mockDigestE: 2,
		},
	}

	mdc.On("ByTest").Return(digestCounts)
	mds.On("UnavailableDigests").Return(map[types.Digest]*diff.DigestFailure{
		mockDigestA: {},
		mockDigestB: {},
	})

	cdf := NewClosestDiffFinder(exp, mdc, mds)

	cdf.Precompute()

	// mockDigestA should not be in this list because it is in the unavailable list.
	expectedToCompareAgainst := types.DigestSlice{mockDigestC, mockDigestD}
	sort.Sort(expectedToCompareAgainst)
	mds.On("Get", diff.PRIORITY_NOW, mockDigestF, mock.AnythingOfType("types.DigestSlice")).Run(func(args mock.Arguments) {
		actual := args.Get(2).(types.DigestSlice)
		sort.Sort(actual)
		assert.Equal(t, expectedToCompareAgainst, actual)
	}).Return(diffEIsClosest(), nil).Once()

	c := cdf.ClosestDigest(mockTest, mockDigestF, types.POSITIVE)
	assert.InDelta(t, 0.0372, float64(c.Diff), 0.01)
	assert.Equal(t, mockDigestE, c.Digest)
	assert.Equal(t, []int{5, 3, 4, 0}, c.MaxRGBA)

	// There is only one negative digest, and it is in the unavailable list, so it should
	// return that it couldn't find one.
	c = cdf.ClosestDigest(mockTest, mockDigestF, types.NEGATIVE)
	assert.InDelta(t, math.MaxFloat32, float64(c.Diff), 0.01)
	assert.Equal(t, NoDigestFound, c.Digest)
	assert.Equal(t, []int{}, c.MaxRGBA)

	// Now test against a test with no digests at all in the latest tile.
	c = cdf.ClosestDigest(testThatDoesNotExist, mockDigestF, types.POSITIVE)
	assert.Equal(t, float32(math.MaxFloat32), c.Diff)
	assert.Equal(t, NoDigestFound, c.Digest)
	assert.Equal(t, []int{}, c.MaxRGBA)
}

func TestCombinedDiffMetric(t *testing.T) {
	unittest.SmallTest(t)
	assert.InDelta(t, 1.0, combinedDiffMetric(0.0, []int{}), 0.000001)
	assert.InDelta(t, 1.0, combinedDiffMetric(1.0, []int{255, 255, 255, 255}), 0.000001)
	assert.InDelta(t, math.Sqrt(0.5), combinedDiffMetric(0.5, []int{255, 255, 255, 255}), 0.000001)
}

const (
	mockTest             = types.TestName("test_foo")
	testThatDoesNotExist = types.TestName("test_bar")

	mockDigestA = types.Digest("aaa")
	mockDigestB = types.Digest("bbb")
	mockDigestC = types.Digest("ccc")
	mockDigestD = types.Digest("ddd")
	mockDigestE = types.Digest("eee")
	mockDigestF = types.Digest("fff")
	mockDigestG = types.Digest("ggg")
)

// diffEIsClosest creates data such that mockDigestE is the closest match.
func diffEIsClosest() map[types.Digest]interface{} {
	return map[types.Digest]interface{}{
		mockDigestE: &diff.DiffMetrics{
			PixelDiffPercent: 0.1,
			MaxRGBADiffs:     []int{5, 3, 4, 0},
		},
		mockDigestA: &diff.DiffMetrics{
			PixelDiffPercent: 10,
			MaxRGBADiffs:     []int{15, 13, 14, 10},
		},
		mockDigestB: &diff.DiffMetrics{
			PixelDiffPercent: 20,
			MaxRGBADiffs:     []int{25, 23, 24, 20},
		},
	}
}

// diffBIsClosest creates data such that mockDigestB is the closest match.
func diffBIsClosest() map[types.Digest]interface{} {
	return map[types.Digest]interface{}{
		mockDigestE: &diff.DiffMetrics{
			PixelDiffPercent: 30,
			MaxRGBADiffs:     []int{35, 33, 34, 30},
		},
		mockDigestA: &diff.DiffMetrics{
			PixelDiffPercent: 10,
			MaxRGBADiffs:     []int{15, 13, 14, 10},
		},
		mockDigestB: &diff.DiffMetrics{
			PixelDiffPercent: .2,
			MaxRGBADiffs:     []int{2, 7, 1, 3},
		},
	}
}
