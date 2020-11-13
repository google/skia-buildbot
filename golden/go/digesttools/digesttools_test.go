package digesttools_test

import (
	"context"
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	mock_diffstore "go.skia.org/infra/golden/go/diffstore/mocks"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/digesttools"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/types"
)

// TestClosestDigest tests the basic interaction between the DiffFinder
// and DiffStore for finding the closest positive and negative diffs.
func TestClosestDigest(t *testing.T) {
	unittest.SmallTest(t)
	mds := &mock_diffstore.DiffStore{}
	mdc := &mocks.DigestCounter{}
	defer mds.AssertExpectations(t)
	defer mdc.AssertExpectations(t)

	var exp expectations.Expectations
	exp.Set(mockTest, mockDigestA, expectations.Positive)
	exp.Set(mockTest, mockDigestB, expectations.Negative)
	exp.Set(mockTest, mockDigestC, expectations.Untriaged)
	exp.Set(mockTest, mockDigestD, expectations.Untriaged)
	exp.Set(mockTest, mockDigestF, expectations.Positive)
	exp.Set(mockTest, mockDigestG, expectations.Positive)

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

	cdf := digesttools.NewClosestDiffFinder(&exp, mdc, mds)

	err := cdf.Precompute(context.Background())
	require.NoError(t, err)

	// Only mockDigestA is both triaged positive and in the digestCounts (meaning, we saw that digest
	// in this tile).
	expectedToCompareAgainst := types.DigestSlice{mockDigestA}
	mds.On("Get", testutils.AnyContext, mockDigestF, expectedToCompareAgainst).Return(diffEIsClosest(), nil).Once()
	// First test against a test that has positive digests.
	c, err := cdf.ClosestDigest(context.Background(), mockTest, mockDigestF, expectations.Positive)
	require.NoError(t, err)
	require.InDelta(t, 1.234, float64(c.Diff), 0.01)
	require.Equal(t, mockDigestE, c.Digest)
	require.Equal(t, [4]int{5, 3, 4, 0}, c.MaxRGBA)

	// mockDigestB is the only negative digest that shows up in the tile.
	expectedToCompareAgainst = types.DigestSlice{mockDigestB}
	mds.On("Get", testutils.AnyContext, mockDigestF, expectedToCompareAgainst).Return(diffBIsClosest(), nil).Once()
	// Now test against negative digests.
	c, err = cdf.ClosestDigest(context.Background(), mockTest, mockDigestF, expectations.Negative)
	require.NoError(t, err)
	require.InDelta(t, 1.234, float64(c.Diff), 0.01)
	require.Equal(t, mockDigestB, c.Digest)
	require.Equal(t, [4]int{2, 7, 1, 3}, c.MaxRGBA)
}

// TestClosestDigest_TestHasNoDigest_ReturnsNoDigestFound tests some more tricky logic dealing
// with tests with no digests.
func TestClosestDigest_TestHasNoDigest_ReturnsNoDigestFound(t *testing.T) {
	unittest.SmallTest(t)
	mds := &mock_diffstore.DiffStore{}
	mdc := &mocks.DigestCounter{}
	defer mds.AssertExpectations(t)
	defer mdc.AssertExpectations(t)

	var exp expectations.Expectations
	exp.Set(mockTest, mockDigestA, expectations.Positive)
	exp.Set(mockTest, mockDigestB, expectations.Negative)
	exp.Set(mockTest, mockDigestC, expectations.Positive)
	exp.Set(mockTest, mockDigestD, expectations.Positive)
	exp.Set(mockTest, mockDigestF, expectations.Positive)
	exp.Set(mockTest, mockDigestG, expectations.Positive)

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

	cdf := digesttools.NewClosestDiffFinder(&exp, mdc, mds)

	err := cdf.Precompute(context.Background())
	require.NoError(t, err)

	expectedDigests := mock.MatchedBy(func(actual types.DigestSlice) bool {
		expectedToCompareAgainst := types.DigestSlice{mockDigestA, mockDigestC, mockDigestD}
		sort.Sort(expectedToCompareAgainst)
		sort.Sort(actual)
		assert.Equal(t, expectedToCompareAgainst, actual)
		return true
	})

	mds.On("Get", testutils.AnyContext, mockDigestF, expectedDigests).Return(diffEIsClosest(), nil).Once()

	c, err := cdf.ClosestDigest(context.Background(), mockTest, mockDigestF, expectations.Positive)
	require.NoError(t, err)
	require.InDelta(t, 1.234, float64(c.Diff), 0.01)
	require.Equal(t, mockDigestE, c.Digest)
	require.Equal(t, [4]int{5, 3, 4, 0}, c.MaxRGBA)

	// Now test against a test with no digests at all in the latest tile.
	c, err = cdf.ClosestDigest(context.Background(), testThatDoesNotExist, mockDigestF, expectations.Positive)
	require.NoError(t, err)
	require.Equal(t, float32(math.MaxFloat32), c.Diff)
	require.Equal(t, digesttools.NoDigestFound, c.Digest)
	require.Equal(t, [4]int{}, c.MaxRGBA)
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
func diffEIsClosest() map[types.Digest]*diff.DiffMetrics {
	return map[types.Digest]*diff.DiffMetrics{
		mockDigestE: {
			PixelDiffPercent: 0.1,
			MaxRGBADiffs:     [4]int{5, 3, 4, 0},
			CombinedMetric:   1.234,
		},
		mockDigestA: {
			PixelDiffPercent: 10,
			MaxRGBADiffs:     [4]int{15, 13, 14, 10},
			CombinedMetric:   2.345,
		},
		mockDigestB: {
			PixelDiffPercent: 20,
			MaxRGBADiffs:     [4]int{25, 23, 24, 20},
			CombinedMetric:   3.456,
		},
	}
}

// diffBIsClosest creates data such that mockDigestB is the closest match.
func diffBIsClosest() map[types.Digest]*diff.DiffMetrics {
	return map[types.Digest]*diff.DiffMetrics{
		mockDigestE: {
			PixelDiffPercent: 30,
			MaxRGBADiffs:     [4]int{35, 33, 34, 30},
			CombinedMetric:   2.345,
		},
		mockDigestA: {
			PixelDiffPercent: 10,
			MaxRGBADiffs:     [4]int{15, 13, 14, 10},
			CombinedMetric:   3.456,
		},
		mockDigestB: {
			PixelDiffPercent: .2,
			MaxRGBADiffs:     [4]int{2, 7, 1, 3},
			CombinedMetric:   1.234,
		},
	}
}
