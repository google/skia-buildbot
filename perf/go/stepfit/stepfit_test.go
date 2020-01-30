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
			interesting: 20,
			expected:    &StepFit{TurningPoint: 2, StepSize: -2.0412414, Status: HIGH, Regression: -20.412415},
			message:     "Original - Simple Step Up",
			stepDet:     types.ORIGINAL_STEP,
		},
		{
			value:       []float32{1, 1, 0, 0, 0},
			interesting: 20,
			expected:    &StepFit{TurningPoint: 2, StepSize: 2.0412414, Status: LOW, Regression: 20.412415},
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
		{
			value:       []float32{1, 1, 1.5, 1.5},
			interesting: 1.0,
			expected:    &StepFit{TurningPoint: 2, StepSize: -0.5, Status: UNINTERESTING, Regression: -0.5},
			message:     "Absolute - no step - too small",
			stepDet:     types.ABSOLUTE_STEP,
		},
		{
			value:       []float32{},
			interesting: 1.0,
			expected:    &StepFit{TurningPoint: 0, StepSize: 0, Status: UNINTERESTING, Regression: 0},
			message:     "Absolute - empty",
			stepDet:     types.ABSOLUTE_STEP,
		},

		{
			value:       []float32{1, 2, 1, 2},
			interesting: 1.0,
			expected:    &StepFit{TurningPoint: 2, StepSize: 0, Status: UNINTERESTING, Regression: 0},
			message:     "Percent - No step",
			stepDet:     types.PERCENT_STEP,
		},
		{
			value:       []float32{1, 1, 2, 2},
			interesting: 1,
			expected:    &StepFit{TurningPoint: 2, StepSize: -1, Status: HIGH, Regression: -1},
			message:     "Percent - step - exact",
			stepDet:     types.PERCENT_STEP,
		},
		{
			value:       []float32{1, 1, 1.5, 1.5},
			interesting: 1.0,
			expected:    &StepFit{TurningPoint: 2, StepSize: -0.5, Status: UNINTERESTING, Regression: -0.5},
			message:     "Percent - no step - too small",
			stepDet:     types.PERCENT_STEP,
		},
		{
			value:       []float32{},
			interesting: 1.0,
			expected:    &StepFit{TurningPoint: 0, StepSize: 0, Status: UNINTERESTING, Regression: 0},
			message:     "Percent - empty",
			stepDet:     types.PERCENT_STEP,
		},
		{
			value:       []float32{1, 1.1, 0.9, 1.02, 1.12, 0.92},
			interesting: 0.2,
			expected:    &StepFit{TurningPoint: 3, StepSize: -0.1999998, Status: UNINTERESTING, Regression: -0.1999998},
			message:     "Cohen - no step - odd - small std dev",
			stepDet:     types.COHEN_STEP,
		},
		{
			value:       []float32{1, 1.1, 0.9, 1.02, 1.12, 0.92},
			interesting: 0.1,
			expected:    &StepFit{TurningPoint: 3, StepSize: -0.1999998, Status: HIGH, Regression: -0.1999998},
			message:     "Cohen - step - odd - small std dev",
			stepDet:     types.COHEN_STEP,
		},
		{
			value:       []float32{1, 1, 2, 2},
			interesting: 0.2,
			expected:    &StepFit{TurningPoint: 2, StepSize: -10, Status: HIGH, Regression: -10},
			message:     "Cohen - step - zero std dev",
			stepDet:     types.COHEN_STEP,
		},
		{
			value:       []float32{1, 2, 3, 3, 4},
			interesting: 0.2,
			expected:    &StepFit{TurningPoint: 2, StepSize: -2.8546433, Status: HIGH, Regression: -2.8546433},
			message:     "Cohen - step - odd - big std dev",
			stepDet:     types.COHEN_STEP,
		},
	}

	for _, tc := range testCases {
		got, want := GetStepFitAtMid(tc.value, 0.1, tc.interesting, tc.stepDet), tc.expected
		assert.Equal(t, want, got, tc.message)
	}
	// With a huge interesting value everything should be uninteresting.
	for _, tc := range testCases {
		got := GetStepFitAtMid(tc.value, 0.1, 500, tc.stepDet)
		if math.IsInf(float64(got.Regression), 1) && math.IsInf(float64(got.Regression), -1) && got.Status != UNINTERESTING {
			t.Errorf("Failed StepFit Got %#v Want %#v: %v Regression %g", got.Status, UNINTERESTING, tc.value, got.Regression)
		}
	}
}
