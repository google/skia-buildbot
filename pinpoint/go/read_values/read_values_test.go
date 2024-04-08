package read_values

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/cabe/go/perfresults"
)

var testData = map[string]perfresults.PerfResults{
	"rendering.desktop": {
		Histograms: map[string]perfresults.Histogram{
			"thread_total_rendering_cpu_time_per_frame": {
				Name:         "thread_total_rendering_cpu_time_per_frame",
				Unit:         "unitless_smallerIsBetter",
				SampleValues: []float64{12.9322},
			},
			"tasks_per_frame_browser": {
				Name:         "tasks_per_frame_browser",
				Unit:         "unitless_smallerIsBetter",
				SampleValues: []float64{0.3917},
			},
			"Compositing.Display.DrawToSwapUs": {
				Name:         "Compositing.Display.DrawToSwapUs",
				Unit:         "unitless_smallerIsBetter",
				SampleValues: []float64{169.9406, 169.9406, 206.3219, 654.8641},
			},
		},
	},
	// the possibility of two benchmarks in one results dataset
	// is very unlikely. We test for it out of an abundance of caution.
	"rendering.desktop.notracing": {
		Histograms: map[string]perfresults.Histogram{
			"thread_total_rendering_cpu_time_per_frame": {
				Name:         "thread_total_rendering_cpu_time_per_frame",
				Unit:         "unitless_smallerIsBetter",
				SampleValues: []float64{0.78},
			},
		},
	},
}

var testAggData = []float64{8, 2, -9, 15, 4}

func TestReadChart_ChartData_ReadValues(t *testing.T) {
	test := func(name, benchmark, chart string, expected ...float64) {
		t.Run(name, func(t *testing.T) {
			values := ReadChart(testData, benchmark, chart)
			// We need to consolidate variadic arg expected into []float64
			// to make assert.Equal work. The value returned from the ReadChart
			// function is of type []float64
			exp := append([]float64{}, expected...)
			assert.Equal(t, exp, values)
		})
	}

	test("basic chart test", "rendering.desktop", "thread_total_rendering_cpu_time_per_frame", 12.9322)
	test("multiple value test", "rendering.desktop", "Compositing.Display.DrawToSwapUs", 169.9406, 169.9406, 206.3219, 654.8641)
	test("null case", "fake benchmark", "fake chart")
}

func TestAggData_NonBlankData_AggData(t *testing.T) {
	test := func(name string, testData []float64, method AggDataMethodEnum, expected float64) {
		t.Run(name, func(t *testing.T) {
			ans, err := aggData(testData, method)
			assert.NoError(t, err)
			assert.InDelta(t, expected, ans, 1e-6)
		})
	}

	test("count", testAggData, Count, 5.0)
	test("mean", testAggData, Mean, 4.0)
	test("max", testAggData, Max, 15.0)
	test("min", testAggData, Min, -9.0)
	test("std", testAggData, Std, 8.803408)
	test("sum", testAggData, Sum, 20.0)
}
