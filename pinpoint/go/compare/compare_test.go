package compare

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareFunctional_GivenNoData_ReturnsError(t *testing.T) {
	x := []float64{}
	y := []float64{0, 0, 0, 0, 0}
	const expected = Unknown
	result, err := CompareFunctional(x, y, DefaultFunctionalErrRate)
	assert.NoError(t, err)
	assert.Equal(t, result.Verdict, expected)
	assert.Zero(t, result.PValue)
}

func TestCompareFunctional_GivenValidInputs_ReturnsCorrectResult(t *testing.T) {
	test := func(name string, x, y []float64, expectedErrRate float64, expected VerdictEnum) {
		t.Run(name, func(t *testing.T) {
			result, err := CompareFunctional(x, y, expectedErrRate)
			assert.NoError(t, err)
			assert.Equal(t, result.Verdict, expected)
			if result.Verdict == verdict(0) {
				// unknown
				assert.LessOrEqual(t, result.PValue, result.HighThreshold)
				assert.Greater(t, result.PValue, result.LowThreshold)
			} else if result.Verdict == verdict(1) {
				// same
				assert.Greater(t, result.PValue, result.HighThreshold)
			} else {
				// different
				assert.LessOrEqual(t, result.PValue, result.LowThreshold)
			}
		})
	}
	x := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	y := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
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
	result, err := ComparePerformance(x, y, magnitude)
	assert.NoError(t, err)
	assert.Equal(t, result.Verdict, expected)
	assert.Zero(t, result.PValue)
}

func TestComparePerformance_GivenValidInputs_ReturnsCorrectResult(t *testing.T) {
	test := func(name string, x, y []float64, magnitude float64, expected VerdictEnum) {
		t.Run(name, func(t *testing.T) {
			result, err := ComparePerformance(x, y, magnitude)
			assert.NoError(t, err)
			assert.Equal(t, result.Verdict, expected)
			if result.Verdict == verdict(0) {
				// unknown
				assert.LessOrEqual(t, result.PValue, result.HighThreshold)
				assert.Greater(t, result.PValue, result.LowThreshold)
			} else if result.Verdict == verdict(1) {
				// same
				assert.Greater(t, result.PValue, result.HighThreshold)
			} else {
				// different
				assert.LessOrEqual(t, result.PValue, result.LowThreshold)
			}
		})
	}
	x := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	y := []float64{3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	test("arrays are slightly different, return unknown", x, y, 0.5, Unknown)

	x = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	y = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	test("arrays are the same, return same", x, y, 1.0, Same)

	x = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	y = []float64{7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	test("arrays are significantly different, return different", x, y, 1.0, Different)
}
