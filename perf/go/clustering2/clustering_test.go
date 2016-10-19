package clustering2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/perf/go/ctrace2"
	"go.skia.org/infra/perf/go/kmeans"
)

func TestParamSummaries(t *testing.T) {
	obs := []kmeans.Clusterable{
		ctrace2.NewFullTrace(",arch=x86,config=8888,", []float32{1, 2}, 0.001),
		ctrace2.NewFullTrace(",arch=x86,config=565,", []float32{2, 3}, 0.001),
		ctrace2.NewFullTrace(",arch=x86,config=565,", []float32{3, 2}, 0.001),
	}
	expected := map[string][]ValueWeight{
		"arch": []ValueWeight{
			{"x86", 26},
		},
		"config": []ValueWeight{
			{"565", 21},
			{"8888", 16},
		},
	}
	assert.Equal(t, expected, getParamSummaries(obs))

	obs = []kmeans.Clusterable{}
	expected = map[string][]ValueWeight{}
	assert.Equal(t, expected, getParamSummaries(obs))
}

func TestStepFit(t *testing.T) {
	testCases := []struct {
		value    []float32
		expected *StepFit
		message  string
	}{
		{
			value:    []float32{0, 0, 1, 1, 1},
			expected: &StepFit{TurningPoint: 2, StepSize: -1, Status: "Low"},
			message:  "Simple Step Up",
		},
		{
			value:    []float32{1, 1, 1, 0, 0},
			expected: &StepFit{TurningPoint: 3, StepSize: 1, Status: "High"},
			message:  "Simple Step Down",
		},
		{
			value:    []float32{1, 1, 1, 1, 1},
			expected: &StepFit{TurningPoint: 0, StepSize: -1, Status: "Uninteresting"},
			message:  "No step",
		},
		{
			value:    []float32{},
			expected: &StepFit{TurningPoint: 0, StepSize: -1, Status: "Uninteresting"},
			message:  "Empty",
		},
	}

	for _, tc := range testCases {
		got, want := getStepFit(tc.value), tc.expected
		if got.StepSize != want.StepSize {
			t.Errorf("Failed StepFit Got %#v Want %#v: %s", got.StepSize, want.StepSize, tc.message)
		}
		if got.Status != want.Status {
			t.Errorf("Failed StepFit Got %#v Want %#v: %s", got.Status, want.Status, tc.message)
		}
		if got.TurningPoint != want.TurningPoint {
			t.Errorf("Failed StepFit Got %#v Want %#v: %s", got.TurningPoint, want.TurningPoint, tc.message)
		}
	}
}
