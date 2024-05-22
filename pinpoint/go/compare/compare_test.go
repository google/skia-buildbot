package compare

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/pinpoint/go/compare/stats"
	"go.skia.org/infra/pinpoint/go/compare/thresholds"
)

func convertNormalizedToRawMagnitude(a, b []float64, normMagnitude float64) float64 {
	all_values := append(a, b...)
	sort.Float64s(all_values)
	iqr := all_values[len(all_values)*3/4] - all_values[len(all_values)/4]
	return normMagnitude * iqr
}

func TestCompareFunctional_GivenNoData_ReturnsError(t *testing.T) {
	x := []float64{}
	y := []float64{0, 0, 0, 0, 0}
	const expected = Unknown
	result, err := CompareFunctional(x, y, DefaultFunctionalErrRate)
	assert.Error(t, err)
	assert.Equal(t, ErrorVerdict, result.Verdict)
}

func TestCompareFunctional_GivenValidInputs_ReturnsCorrectResult(t *testing.T) {
	test := func(name string, x, y []float64, expectedErrRate float64, expected Verdict) {
		t.Run(name, func(t *testing.T) {
			result, err := CompareFunctional(x, y, expectedErrRate)
			assert.NoError(t, err)
			assert.Equal(t, expected, result.Verdict)
			if result.Verdict == Unknown {
				assert.LessOrEqual(t, result.PValue, result.HighThreshold)
				assert.Greater(t, result.PValue, result.LowThreshold)
			} else if result.Verdict == Same {
				assert.Greater(t, result.PValue, result.HighThreshold)
			} else if result.Verdict == Different {
				assert.LessOrEqual(t, result.PValue, result.LowThreshold)
			} else {
				t.Errorf("Obtained non-existent verdict %s", result.Verdict)
			}
		})
	}
	x := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	y := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	test("arrays are slightly different, return unknown", x, y, 0.5, Unknown)

	x = []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	y = []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	test("arrays are the same, return same", x, y, 1.0, Same)

	x = []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	y = []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	test("arrays are significantly different, return different", x, y, 1.0, Different)
}

func TestComparePerformance_GivenNoData_ReturnsNil(t *testing.T) {
	x := []float64{0, 0, 0, 0, 0}
	y := []float64{}
	const magnitude = 1.0
	const expected = Unknown
	result, err := ComparePerformance(x, y, magnitude, UnknownDir)
	assert.NoError(t, err)
	assert.Equal(t, NilVerdict, result.Verdict)
}

func TestComparePerformance_GivenValidInputs_ReturnsCorrectResult(t *testing.T) {
	test := func(name string, x, y []float64, magnitude float64, expected Verdict) {
		t.Run(name, func(t *testing.T) {
			result, err := ComparePerformance(x, y, magnitude, UnknownDir)
			assert.NoError(t, err)
			assert.Equal(t, expected, result.Verdict)
			switch result.Verdict {
			case Unknown:
				assert.LessOrEqual(t, result.PValue, result.HighThreshold)
				assert.Greater(t, result.PValue, result.LowThreshold)
			case Same:
				assert.Greater(t, result.PValue, result.HighThreshold)
			case Different:
				assert.LessOrEqual(t, result.PValue, result.LowThreshold)
			default:
				assert.Fail(t, "Unsupported verdict found %s", result.Verdict)
			}
		})
	}
	x := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	y := []float64{3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	mag := convertNormalizedToRawMagnitude(x, y, 0.5)
	test("arrays are slightly different, return unknown", x, y, mag, Unknown)

	x = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	y = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	test("arrays are the same, return same", x, y, 0.0, Same)

	x = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	y = []float64{7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	test("arrays are significantly different, return different", x, y, 0.0, Different)
}

func TestComparePerformance_GivenSmallDifference_ReturnsSame(t *testing.T) {
	x := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	y := []float64{7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	result, err := ComparePerformance(x, y, 1e6, UnknownDir)
	assert.NoError(t, err)
	assert.Equal(t, Same, result.Verdict)
	assert.Zero(t, result.PValue)
	assert.True(t, result.IsTooSmall)
}

func TestCompare_GivenImprovement_ReturnsSameAndNoPValue(t *testing.T) {
	test := func(name string, x, y []float64, dir ImprovementDir) {
		t.Run(name, func(t *testing.T) {
			result, err := compare(x, y, 0.0, 0.0, dir)
			require.NoError(t, err)
			assert.Equal(t, Same, result.Verdict)
			assert.Zero(t, result.PValue)
			switch dir {
			case Up:
				assert.Positive(t, result.MeanDiff)
			case Down:
				assert.Negative(t, result.MeanDiff)
			}
		})
	}
	x := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	y := []float64{7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	test("x < y, ImprovementDir = Up, return same", x, y, Up)

	x = []float64{7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	y = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	test("x > y, ImprovementDir = Down, return same", x, y, Down)
}

func TestCompare_GivenRegression_ReturnsPValue(t *testing.T) {
	test := func(name string, x, y []float64, dir ImprovementDir) {
		t.Run(name, func(t *testing.T) {
			result, err := compare(x, y, 0.0, 0.0, dir)
			require.NoError(t, err)
			assert.NotZero(t, result.PValue)
			switch dir {
			case Up:
				assert.Negative(t, result.MeanDiff)
			case Down:
				assert.Positive(t, result.MeanDiff)
			}
		})
	}
	x := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	y := []float64{7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	test("x < y, ImprovementDir = Down", x, y, Down)

	x = []float64{7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	y = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	test("x > y, ImprovementDir = Up", x, y, Up)
}

func TestComparePairwise_GivenImprovement_ReturnsSame(t *testing.T) {
	test := func(name string, x, y []float64, dir ImprovementDir) {
		t.Run(name, func(t *testing.T) {
			result, err := ComparePairwise(x, y, dir)
			require.NoError(t, err)
			require.LessOrEqual(t, result.PairwiseWilcoxonSignedRankedTestResult.PValue, thresholds.LowThreshold)
			assert.Equal(t, Same, result.Verdict)
			switch dir {
			case Up:
				assert.Positive(t, result.PairwiseWilcoxonSignedRankedTestResult.UpperCi)
			case Down:
				assert.Negative(t, result.PairwiseWilcoxonSignedRankedTestResult.LowerCi)
			}
		})
	}
	x := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	y := []float64{7, 8, 9, 10, 11, 12, 13, 14, 15}
	test("x < y, ImprovementDir = Up", x, y, Up)

	x = []float64{7, 8, 9, 10, 11, 12, 13, 14, 15}
	y = []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	test("x > y, ImprovementDir = Down", x, y, Down)
}

func TestComparePairwise_GivenRegression_ReturnsDifferent(t *testing.T) {
	test := func(name string, x, y []float64, dir ImprovementDir) {
		t.Run(name, func(t *testing.T) {
			result, err := ComparePairwise(x, y, dir)
			require.NoError(t, err)
			require.LessOrEqual(t, result.PairwiseWilcoxonSignedRankedTestResult.PValue, thresholds.LowThreshold)
			assert.Equal(t, Different, result.Verdict)
			assert.Positive(t, result.PairwiseWilcoxonSignedRankedTestResult.LowerCi*result.PairwiseWilcoxonSignedRankedTestResult.UpperCi)
			switch dir {
			case Up:
				assert.Negative(t, result.PairwiseWilcoxonSignedRankedTestResult.UpperCi)
			case Down:
				assert.Positive(t, result.PairwiseWilcoxonSignedRankedTestResult.LowerCi)
			}
		})
	}
	x := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	y := []float64{7, 8, 9, 10, 11, 12, 13, 14, 15}
	test("x < y, ImprovementDir = Down", x, y, Down)
	test("x < y, ImprovementDir = Unknown", x, y, UnknownDir)

	x = []float64{7, 8, 9, 10, 11, 12, 13, 14, 15}
	y = []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	test("x > y, ImprovementDir = Up", x, y, Up)
}

func TestComparePairwise_GivenNoRegression_ReturnsSame(t *testing.T) {
	test := func(name string, x, y []float64, dir ImprovementDir) {
		t.Run(name, func(t *testing.T) {
			result, err := ComparePairwise(x, y, dir)
			require.NoError(t, err)
			require.GreaterOrEqual(t, result.PairwiseWilcoxonSignedRankedTestResult.PValue, thresholds.LowThreshold)
			assert.Equal(t, Same, result.Verdict)
			assert.Negative(t, result.PairwiseWilcoxonSignedRankedTestResult.LowerCi*result.PairwiseWilcoxonSignedRankedTestResult.UpperCi)
		})
	}
	x := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	y := []float64{1.1, 1.9, 3.1, 3.9, 4.1, 4.9, 6.1, 7.9, 9.1}
	test("ImprovementDir = Down", x, y, Down)
	test("ImprovementDir = Unknown", x, y, UnknownDir)
}

func TestComparePairwise_GivenZeros_UsesNormalizedResult(t *testing.T) {
	x := []float64{1, 2, 0, 4, 5, 6, 7, 8, 9}
	y := []float64{1.1, 0, 3.1, 3.9, 4.1, 4.9, 6.1, 7.9, 9.1}

	result, err := ComparePairwise(x, y, Up)
	require.NoError(t, err)
	normalizedResult, err := stats.PairwiseWilcoxonSignedRankedTest(y, x, stats.TwoSided, stats.NormalizeResult)
	require.NoError(t, err)
	assert.Equal(t, normalizedResult, result.PairwiseWilcoxonSignedRankedTestResult)
}

func TestComparePairwise_GivenNonZeros_UsesLogTransform(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	y := []float64{1.1, 2.1, 3.1, 3.9, 4.1, 4.9, 6.1, 7.9, 9.1}

	result, err := ComparePairwise(x, y, Up)
	require.NoError(t, err)
	logTransformResult, err := stats.PairwiseWilcoxonSignedRankedTest(y, x, stats.TwoSided, stats.LogTransform)
	require.NoError(t, err)
	assert.Equal(t, logTransformResult, result.PairwiseWilcoxonSignedRankedTestResult)
}

func TestComparePairwise_GivenNegativeNumbers_ThrowsError(t *testing.T) {
	x := []float64{1, 2, 3, -4, 5, 6, 7, 8, 9}
	y := []float64{1.1, 2.1, 3.1, 3.9, 4.1, 4.9, 6.1, 7.9, 9.1}

	result, err := ComparePairwise(x, y, Up)
	require.Error(t, err)
	assert.Nil(t, result)
}
