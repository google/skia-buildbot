package compare

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMannWhitneyU_Samples_ReturnsExpectedOutput(t *testing.T) {
	test := func(name string, xData, yData []float64, expected float64, exact bool) {
		t.Run(name, func(t *testing.T) {
			result := MannWhitneyU(xData, yData)
			if exact {
				assert.Equal(t, result, expected)
			} else {
				// almostExpected criteria is numerical difference
				// rounded to the 7th difference
				assert.InDelta(t, expected, result, 1e-7)
			}
		})
	}

	x := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	y := []float64{20, 21, 22, 23, 24, 25, 26, 27, 28, 29}
	// The expected values are calculated as explained here:
	// https://en.wikipedia.org/wiki/Mann%E2%80%93Whitney_U_test
	test("basic test 1", x, y, 0.0001827, false)

	x = []float64{0, 1, 2, 3, 4}
	y = []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	test("basic test 2", x, y, 0.1398636, false)

	x = []float64{0, 0, 0, 0, 0}
	y = []float64{1, 1, 1, 1, 1}
	test("duplicate values", x, y, 0.0039768, false)

	// For the following cases, it is expected that the outputs should exactly
	// match the expected values.
	// See https://source.corp.google.com/h/skia/buildbot/+/main:pinpoint/go/compare/mann_whitney_u.go;drc=fffea2a3556ac9ab9d2bf1ee62526c81aa328f62;l=50
	x = []float64{0}
	y = []float64{1}
	test("small samples", x, y, 1.0, true)

	x = []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	y = []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	test("identical values", x, y, 1.0, true)
}
