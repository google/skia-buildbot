package stepfit

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/types"
)

func TestStepFit(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		value       []float32
		interesting float32
		expected    *StepFit
		message     string
		stepDet     types.StepDetection
	}{
		{
			value:       []float32{0, 0, 1, 1, 1},
			interesting: 50,
			expected:    &StepFit{TurningPoint: 2, StepSize: -2.0412414, Status: HIGH, Regression: -204124.14},
			message:     "Original - Simple Step Up",
			stepDet:     types.ORIGINAL_STEP,
		},
		{
			value:       []float32{1, 1, 0, 0, 0},
			interesting: 50,
			expected:    &StepFit{TurningPoint: 2, StepSize: 2.0412414, Status: LOW, Regression: 204124.14},
			message:     "Original - Simple Step Down",
			stepDet:     types.ORIGINAL_STEP,
		},
		{
			value:       []float32{1, 1, 1, 1, 1},
			interesting: 50,
			expected:    &StepFit{TurningPoint: 2, StepSize: -1, Status: UNINTERESTING, Regression: -2.7105057e-19},
			message:     "Original - No step",
			stepDet:     types.ORIGINAL_STEP,
		},
		{
			value:       []float32{},
			interesting: 50,
			expected:    &StepFit{TurningPoint: 0, StepSize: -1, Status: UNINTERESTING, Regression: 0},
			message:     "Original - Empty",
			stepDet:     types.ORIGINAL_STEP,
		},
		{
			value:       []float32{1, 2, 1, 2},
			interesting: 1.0,
			expected:    &StepFit{TurningPoint: 2, StepSize: 0, Status: UNINTERESTING, Regression: 0},
			message:     "Absolute - No step",
			stepDet:     types.ABSOLUTE_STEP,
		},
		{
			value:       []float32{1, 1, 2, 2},
			interesting: 1.0,
			expected:    &StepFit{TurningPoint: 2, StepSize: -1, Status: HIGH, Regression: -1},
			message:     "Absolute - step - exact",
			stepDet:     types.ABSOLUTE_STEP,
		},
	}

	for _, tc := range testCases {
		got, want := GetStepFitAtMid(tc.value, 0.1, tc.interesting, tc.stepDet), tc.expected
		assert.Equal(t, got, want, tc.message)
	}
	// With a huge interesting value everything should be uninteresting.
	for _, tc := range testCases {
		got := GetStepFitAtMid(tc.value, 0.1, 500, tc.stepDet)
		if math.IsInf(float64(got.Regression), 1) && math.IsInf(float64(got.Regression), -1) && got.Status != UNINTERESTING {
			t.Errorf("Failed StepFit Got %#v Want %#v: %v Regression %g", got.Status, UNINTERESTING, tc.value, got.Regression)
		}
	}
}
