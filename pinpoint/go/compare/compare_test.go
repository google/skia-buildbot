package compare

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareNoData(t *testing.T) {
	for i, test := range []struct {
		name                 string
		performance_mode     bool
		x                    []float64
		y                    []float64
		sample_size          int
		normalized_magnitude float64
		expected             VerdictEnum
	}{
		{
			name:                 "functional no data",
			performance_mode:     false,
			x:                    []float64{},
			y:                    []float64{0, 0, 0, 0, 0},
			sample_size:          5,
			normalized_magnitude: 1.0,
			expected:             Unknown,
		},
		{
			name:                 "performance no data",
			performance_mode:     true,
			x:                    []float64{0, 0, 0, 0, 0},
			y:                    []float64{},
			sample_size:          5,
			normalized_magnitude: 1.0,
			expected:             Unknown,
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			var result *CompareResults
			var err error
			if test.performance_mode {
				result, err = ComparePerformance(test.x, test.y, test.sample_size, test.normalized_magnitude)
			} else {
				result, err = CompareFunctional(test.x, test.y, test.sample_size, test.normalized_magnitude)
			}
			assert.Nil(t, err)
			assert.Equal(t, result.Verdict, test.expected)
			assert.Zero(t, result.PValue)
		})
	}
}

func TestCompare(t *testing.T) {
	for i, test := range []struct {
		name                 string
		performance_mode     bool
		x                    []float64
		y                    []float64
		sample_size          int
		normalized_magnitude float64
		expected             VerdictEnum
	}{
		{
			name:                 "functional unknown",
			performance_mode:     false,
			x:                    []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			y:                    []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			sample_size:          10,
			normalized_magnitude: 0.5,
			expected:             Unknown,
		},
		{
			name:                 "functional same",
			performance_mode:     false,
			x:                    []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			y:                    []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			sample_size:          10,
			normalized_magnitude: 0.5,
			expected:             Same,
		},
		{
			name:                 "functional different",
			performance_mode:     false,
			x:                    []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			y:                    []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			sample_size:          10,
			normalized_magnitude: 0.5,
			expected:             Different,
		},
		{
			name:                 "performance different",
			performance_mode:     true,
			x:                    []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			y:                    []float64{7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			sample_size:          10,
			normalized_magnitude: 1,
			expected:             Different,
		},
		{
			name:                 "performance unknown",
			performance_mode:     true,
			x:                    []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			y:                    []float64{3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
			sample_size:          10,
			normalized_magnitude: 1,
			expected:             Unknown,
		},
		{
			name:                 "performance same",
			performance_mode:     true,
			x:                    []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			y:                    []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			sample_size:          10,
			normalized_magnitude: 1,
			expected:             Same,
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			var result *CompareResults
			var err error
			if test.performance_mode {
				result, err = ComparePerformance(test.x, test.y, test.sample_size, test.normalized_magnitude)
			} else {
				result, err = CompareFunctional(test.x, test.y, test.sample_size, test.normalized_magnitude)
			}
			assert.Nil(t, err)
			assert.Equal(t, result.Verdict, test.expected)
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
}
