package regression

import (
	"testing"

	"go.skia.org/infra/go/testutils"
)

func TestFilterTail(t *testing.T) {
	testutils.SmallTest(t)
	testCases := []struct {
		trace    []float32
		quantile float64
		mult     float64
		slack    float64
		expected float32
	}{
		{
			trace:    []float32{0, 0, 0, 0, -1},
			quantile: 0.1,
			mult:     1.5,
			slack:    0,
			expected: -1,
		},
		{
			trace:    []float32{0, 0, 0, 0, 1},
			quantile: 0.1,
			mult:     1.5,
			slack:    0,
			expected: 1,
		},
		{
			trace:    []float32{1, 1, 1, 1, 1},
			quantile: 0.1,
			mult:     1.5,
			slack:    0,
			expected: 0,
		},
		{
			trace:    []float32{},
			quantile: 0.1,
			mult:     1.5,
			slack:    0,
			expected: 0,
		},
		{
			trace:    []float32{0.1, 0, 0, 0.12},
			quantile: 0.1,
			mult:     1.5,
			slack:    0,
			expected: 0,
		},
		{
			trace:    []float32{0.1, 0, 0, 0.2},
			quantile: 0.1,
			mult:     1.5,
			slack:    0,
			expected: 0.2,
		},
		{
			trace:    []float32{0.1, -0.1, 0, -0.2},
			quantile: 0.1,
			mult:     1.5,
			slack:    0,
			expected: -0.2,
		},
		{
			trace:    []float32{0.1, -0.1, 0, -0.2},
			quantile: 0.1,
			mult:     1.5,
			slack:    0.1,
			expected: 0,
		},
		{
			trace:    []float32{0.1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.1},
			quantile: 0.1,
			mult:     1.5,
			slack:    0,
			expected: 0.1,
		},
	}

	for _, tc := range testCases {
		got, want := FilterTail(tc.trace, tc.quantile, tc.mult, tc.slack), tc.expected
		if got != want {
			t.Errorf("Failed FilterTail Got %#v Want %#v. Trace: %v", got, want, tc.trace)
		}
	}
}
