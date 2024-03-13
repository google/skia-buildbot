package compare

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKolmogorovSmirnov_Samples_ReturnsExpectedOutput(t *testing.T) {
	test := func(name string, xData, yData []float64, expected float64, exact bool) {
		t.Run(name, func(t *testing.T) {
			result, err := KolmogorovSmirnov(xData, yData)
			require.NoError(t, err)
			if exact {
				assert.Equal(t, result, expected)
			} else {
				// almostExpected criteria is numerical difference
				// rounded to the 5th difference
				assert.InDelta(t, expected, result, 1e-5)
			}
		})
	}

	// The expected value for these unit tests are taken from:
	// https://source.chromium.org/chromium/chromium/src/+/main:third_party/catapult/dashboard/dashboard/pinpoint/models/compare/kolmogorov_smirnov_test.py;drc=960c656c2fe229173bf8b0a10c45f28447d594f5;l=14
	x := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	y := []float64{20, 21, 22, 23, 24, 25, 26, 27, 28, 29}
	test("basic test 1", x, y, 1.8879794e-05, false)

	x = []float64{0, 1, 2, 3, 4}
	y = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	test("basic test 2", x, y, 0.26680, false)

	x = []float64{0, 0, 0, 0, 0}
	y = []float64{1, 1, 1, 1, 1}
	test("duplicate values", x, y, 0.00378, false)

	x = []float64{0}
	y = []float64{1}
	test("small samples", x, y, 0.28904, false)

	x = []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	y = []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	test("identical values", x, y, 1.0, true)
}

func TestKolmogorovSmirnov_EmptyData_ReturnsError(t *testing.T) {
	_, err := KolmogorovSmirnov([]float64{}, []float64{})
	assert.Error(t, err)

	_, err = KolmogorovSmirnov(nil, nil)
	assert.Error(t, err)
}
