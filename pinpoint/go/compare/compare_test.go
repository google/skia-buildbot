package compare

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.NoError(t, err)
	assert.Equal(t, expected, result.Verdict)
	assert.Zero(t, result.PValue)
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

func TestComparePerformance_GivenNoData_ReturnsError(t *testing.T) {
	x := []float64{0, 0, 0, 0, 0}
	y := []float64{}
	const magnitude = 1.0
	const expected = Unknown
	result, err := ComparePerformance(x, y, magnitude, UnknownDir)
	assert.NoError(t, err)
	assert.Equal(t, expected, result.Verdict)
	assert.Zero(t, result.PValue)
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
	var x = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	var y = []float64{7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
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
	var x = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	var y = []float64{7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	test("x < y, ImprovementDir = Down", x, y, Down)

	x = []float64{7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	y = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	test("x > y, ImprovementDir = Up", x, y, Up)
}
