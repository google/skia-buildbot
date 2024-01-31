package thresholds

import (
	"fmt"
	"testing"
)

func TestLowThreshold(t *testing.T) {
	if LowThreshold != 0.01 {
		t.Errorf("Expected Low Threshold to be 0.01 but got %v", LowThreshold)
	}
}

func TestHighThresholds(t *testing.T) {
	for i, test := range []struct {
		name                 string
		performance_mode     bool
		normalized_magnitude float64
		sample_size          int
		// test if result < expected
		// this replicates assertLessEqual
		less_equal bool
		expected   float64
	}{
		{
			name:                 "basic functional test",
			performance_mode:     false,
			normalized_magnitude: 0.5,
			sample_size:          20,
			less_equal:           false,
			expected:             0.0195,
		},
		{
			name:                 "basic performance test",
			performance_mode:     true,
			normalized_magnitude: 10,
			sample_size:          5,
			less_equal:           false,
			expected:             0.0122,
		},
		{
			name:                 "performance test low comparison magnitude",
			performance_mode:     true,
			normalized_magnitude: 0.1,
			sample_size:          20,
			less_equal:           true,
			expected:             0.99,
		},
		{
			name:                 "performance test low > high threshold",
			performance_mode:     true,
			normalized_magnitude: 1.5,
			sample_size:          20,
			less_equal:           true,
			expected:             LowThreshold,
		},
		{
			name:                 "performance test high sample size",
			performance_mode:     true,
			normalized_magnitude: 1.5,
			sample_size:          50,
			less_equal:           true,
			expected:             LowThreshold,
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			var result float64
			if test.performance_mode {
				result, _ = HighThresholdPerformance(test.normalized_magnitude, test.sample_size)
			} else {
				result, _ = HighThresholdFunctional(test.normalized_magnitude, test.sample_size)
			}
			if test.less_equal {
				if result > test.expected {
					t.Errorf("Expected %v to be less than %v", test.expected, result)
				}
			} else {
				if result != test.expected {
					t.Errorf("Expected %v, but got %v", test.expected, result)
				}
			}
		})
	}
}
