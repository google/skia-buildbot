package compare

import (
	"fmt"
	"math"
	"testing"
)

func TestMannWhitneyU(t *testing.T) {
	for i, test := range []struct {
		name     string
		xdata    []float64
		ydata    []float64
		expected float64
		// if True, use almostEquals criteria, which is numerical difference
		// rounded to the 7th difference
		// https://docs.python.org/3/library/unittest.html#unittest.TestCase.assertAlmostEqual
		// else, use exact equals
		almostEquals bool
	}{
		{
			name:         "small samples",
			xdata:        []float64{0},
			ydata:        []float64{1},
			expected:     1.0,
			almostEquals: false,
		},
		{
			name:         "basic test 1",
			xdata:        []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			ydata:        []float64{20, 21, 22, 23, 24, 25, 26, 27, 28, 29},
			expected:     0.00018267179110955002,
			almostEquals: true,
		},
		{
			name:         "basic test 2",
			xdata:        []float64{0, 1, 2, 3, 4},
			ydata:        []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			expected:     0.13986357686781267,
			almostEquals: true,
		},
		{
			name:         "duplicate values",
			xdata:        []float64{0, 0, 0, 0, 0},
			ydata:        []float64{1, 1, 1, 1, 1},
			expected:     0.0039767517097886512,
			almostEquals: true,
		},
		{
			name:         "identical values",
			xdata:        []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			ydata:        []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			expected:     1.0,
			almostEquals: false,
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			result := MannWhitneyU(test.xdata, test.ydata)
			if test.almostEquals {
				if math.Abs(result-test.expected) > 1e-7 {
					t.Errorf("Expected %v, but got %v", test.expected, result)
				}
			} else {
				if result != test.expected {
					t.Errorf("Expected %v, but got %v", test.expected, result)
				}
			}
		})
	}
}
