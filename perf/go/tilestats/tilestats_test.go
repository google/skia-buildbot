package tilestats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalcStats(t *testing.T) {
	testcases := []struct {
		vec      []float64
		expected TraceStats
	}{
		{
			vec:      []float64{},
			expected: TraceStats{},
		},
		{
			vec: []float64{1.0},
			expected: TraceStats{
				Mean:   1.0,
				StdDev: 0.0,
				Q1:     0.0,
				Q2:     0.0,
				Q3:     0.0,
			},
		},
		{
			vec: []float64{1.0, 3.0},
			expected: TraceStats{
				Mean:   2.0,
				StdDev: 1.0,
				Q1:     2.0,
				Q2:     2.0,
				Q3:     2.0,
			},
		},
		{
			vec: []float64{1.0, 2.0, 3.0},
			expected: TraceStats{
				Mean:   2.0,
				StdDev: 0.8165,
				Q1:     1.0,
				Q2:     2.0,
				Q3:     3.0,
			},
		},
		{
			vec: []float64{7, 15, 36, 39, 40, 41},
			expected: TraceStats{
				Mean:   29.6666,
				StdDev: 13.4866,
				Q1:     15,
				Q2:     37.5,
				Q3:     40,
			},
		},
		{
			vec: []float64{6, 7, 15, 36, 39, 40, 41, 42, 43, 47, 49},
			expected: TraceStats{
				Mean:   33.1818,
				StdDev: 15.1346,
				Q1:     15,
				Q2:     40,
				Q3:     43,
			},
		},
	}

	for _, tc := range testcases {
		mean, stddev, q1, q2, q3 := calcTraceStats(tc.vec)
		assert.InDelta(t, mean, tc.expected.Mean, 0.001, "Mean: %#v", tc.vec)
		assert.InDelta(t, stddev, tc.expected.StdDev, 0.001, "StdDev : %#v", tc.vec)
		assert.InDelta(t, q1, tc.expected.Q1, 0.001, "Q1: %#v", tc.vec)
		assert.InDelta(t, q2, tc.expected.Q2, 0.001, "Q2: %#v", tc.vec)
		assert.InDelta(t, q3, tc.expected.Q3, 0.001, "Q3: %#v", tc.vec)
	}
}
