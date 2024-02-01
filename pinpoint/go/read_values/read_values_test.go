package read_values

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/cabe/go/perfresults"
)

var testData = map[string]perfresults.PerfResults{
	"rendering.desktop": {
		Histograms: []perfresults.Histogram{
			{
				Name:         "thread_total_rendering_cpu_time_per_frame",
				Unit:         "unitless_smallerIsBetter",
				SampleValues: []float64{12.9322},
			},
			{
				Name:         "tasks_per_frame_browser",
				Unit:         "unitless_smallerIsBetter",
				SampleValues: []float64{0.3917},
			},
			{
				Name:         "Compositing.Display.DrawToSwapUs",
				Unit:         "unitless_smallerIsBetter",
				SampleValues: []float64{169.9406, 169.9406, 206.3219, 654.8641},
			},
		},
	},
	// the possibility of two benchmarks in one results dataset
	// is very unlikely. We test for it out of an abundance of caution.
	"rendering.desktop.notracing": {
		Histograms: []perfresults.Histogram{
			{
				Name:         "thread_total_rendering_cpu_time_per_frame",
				Unit:         "unitless_smallerIsBetter",
				SampleValues: []float64{0.78},
			},
		},
	},
}

var testAggData = []float64{8, 2, -9, 15, 4}

func TestReadValues(t *testing.T) {
	for i, test := range []struct {
		name      string
		benchmark string
		chart     string
		expected  []float64
	}{
		{
			name:      "basic chart test",
			benchmark: "rendering.desktop",
			chart:     "thread_total_rendering_cpu_time_per_frame",
			expected:  []float64{12.9322},
		},
		{
			name:      "multiple value test",
			benchmark: "rendering.desktop",
			chart:     "Compositing.Display.DrawToSwapUs",
			expected:  []float64{169.9406, 169.9406, 206.3219, 654.8641},
		},
		{
			name:      "null case",
			benchmark: "fake benchmark",
			chart:     "fake chart",
			expected:  []float64{},
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			values := ReadChart(testData, test.benchmark, test.chart)
			assert.Equal(t, test.expected, values)
		})
	}
}

func TestAggData(t *testing.T) {
	for i, test := range []struct {
		name        string
		method      AggDataMethodEnum
		testData    []float64
		expected    float64
		expectedErr bool
	}{
		{
			name:     "count",
			method:   Count,
			testData: testAggData,
			expected: 5.0,
		},
		{
			name:     "mean",
			method:   Mean,
			testData: testAggData,
			expected: 4.0,
		},
		{
			name:     "max",
			method:   Max,
			testData: testAggData,
			expected: 15.0,
		},
		{
			name:     "min",
			method:   Min,
			testData: testAggData,
			expected: -9.0,
		},
		{
			name:     "std",
			method:   Std,
			testData: testAggData,
			expected: 8.80340843082,
		},
		{
			name:     "sum",
			method:   Sum,
			testData: testAggData,
			expected: 20.0,
		},
		{
			name:        "nil data",
			method:      Sum,
			testData:    nil,
			expectedErr: true,
		},
		{
			name:        "length 0 data",
			method:      Sum,
			testData:    []float64{},
			expectedErr: true,
		},
	} {
		t.Run(fmt.Sprintf("[%d] %s", i, test.name), func(t *testing.T) {
			ans, err := aggData(test.testData, test.method)
			if test.expectedErr {
				assert.Error(t, err)
			} else {
				assert.InDelta(t, test.expected, ans, 1e-6)
			}
		})
	}
}
