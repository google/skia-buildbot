package refiner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/ui/frame"
)

func TestFindPeaks_ReturnsStructuredPeaks(t *testing.T) {
	// Helper to create a response with a specific regression value
	r := func(val float64) *regression.RegressionDetectionResponse {
		return &regression.RegressionDetectionResponse{
			Frame: &frame.FrameResponse{DataFrame: &dataframe.DataFrame{}},
			Summary: &clustering2.ClusterSummaries{
				Clusters: []*clustering2.ClusterSummary{
					{
						StepFit: &stepfit.StepFit{
							Regression: float32(val),
							Status:     stepfit.LOW,
						},
						StepPoint: &dataframe.ColumnHeader{Offset: 100},
						Keys:      []string{"k"},
					},
				},
			},
		}
	}

	tests := []struct {
		name     string
		input    []*regression.RegressionDetectionResponse
		expected PeakIndexes
	}{
		{
			name:  "Single Peak",
			input: []*regression.RegressionDetectionResponse{r(1), r(3), r(1)},
			expected: PeakIndexes{
				LeftIndex: 1, RightIndex: 1, MaxIndex: 1,
			},
		},
		{
			name:  "Flat Peak",
			input: []*regression.RegressionDetectionResponse{r(1), r(3), r(3), r(1)},
			expected: PeakIndexes{
				LeftIndex: 1, RightIndex: 2, MaxIndex: 2, // (1+2+1)/2 = 2
			},
		},
		{
			name:  "Multiple Peaks - Global Max Middle",
			input: []*regression.RegressionDetectionResponse{r(1), r(5), r(1), r(20), r(1), r(5), r(1)},
			expected: PeakIndexes{
				LeftIndex: 1, RightIndex: 5, MaxIndex: 3, // Left=1(first peak), Right=5(last peak), Max=3(val 20)
			},
		},
		{
			name:  "Multiple Peaks - Flat Global Max",
			input: []*regression.RegressionDetectionResponse{r(1), r(5), r(1), r(20), r(20), r(1), r(5), r(1)},
			expected: PeakIndexes{
				LeftIndex: 1, RightIndex: 6, MaxIndex: 4, // Max=4 (center of 3,4)
			},
		},
		{
			name:  "Edge Peak Left",
			input: []*regression.RegressionDetectionResponse{r(5), r(1)},
			expected: PeakIndexes{
				LeftIndex: 0, RightIndex: 0, MaxIndex: 0,
			},
		},
		{
			name:  "One Item",
			input: []*regression.RegressionDetectionResponse{r(-5)},
			expected: PeakIndexes{
				LeftIndex: 0, RightIndex: 0, MaxIndex: 0,
			},
		},
		{
			name:  "Edge Peak Right",
			input: []*regression.RegressionDetectionResponse{r(1), r(5)},
			expected: PeakIndexes{
				LeftIndex: 1, RightIndex: 1, MaxIndex: 1,
			},
		},
		{
			name:  "No Peaks - Flat Region",
			input: []*regression.RegressionDetectionResponse{r(1), r(1), r(1)},
			expected: PeakIndexes{
				LeftIndex: 0, RightIndex: 2, MaxIndex: 1, // Fallback: 0, n-1, n/2
			},
		},
		{
			name:  "No Peaks - Flat Region v2",
			input: []*regression.RegressionDetectionResponse{r(10), r(10), r(10), r(10)},
			expected: PeakIndexes{
				LeftIndex: 0, RightIndex: 3, MaxIndex: 2, // Fallback: 0, n-1, n/2
			},
		},
		{
			name:  "No Data",
			input: []*regression.RegressionDetectionResponse{},
			expected: PeakIndexes{
				LeftIndex: 0, RightIndex: 0, MaxIndex: 0,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, findPeaks(tc.input))
		})
	}
}
