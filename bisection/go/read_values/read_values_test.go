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
