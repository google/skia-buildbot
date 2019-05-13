package stepfit

import (
	"math"
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
)

func TestStepFit(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		value    []float32
		expected *StepFit
		message  string
	}{
		{
			value:    []float32{0, 0, 1, 1, 1},
			expected: &StepFit{TurningPoint: 2, StepSize: -1, Status: HIGH, Regression: -100000},
			message:  "Simple Step Up",
		},
		{
			value:    []float32{1, 1, 0, 0, 0},
			expected: &StepFit{TurningPoint: 2, StepSize: 1, Status: LOW, Regression: 100000},
			message:  "Simple Step Down",
		},
		{
			value:    []float32{1, 1, 1, 1, 1},
			expected: &StepFit{TurningPoint: 0, StepSize: -1, Status: UNINTERESTING, Regression: -2.7105057e-19},
			message:  "No step",
		},
		{
			value:    []float32{},
			expected: &StepFit{TurningPoint: 0, StepSize: -1, Status: UNINTERESTING, Regression: 0},
			message:  "Empty",
		},
	}

	for _, tc := range testCases {
		got, want := GetStepFitAtMid(tc.value, 50), tc.expected
		if got.StepSize != want.StepSize {
			t.Errorf("Failed StepFit Got %#v Want %#v: %s", got.StepSize, want.StepSize, tc.message)
		}
		if got.Status != want.Status {
			t.Errorf("Failed StepFit Got %#v Want %#v: %s", got.Status, want.Status, tc.message)
		}
		if got.TurningPoint != want.TurningPoint {
			t.Errorf("Failed StepFit Got %#v Want %#v: %s", got.TurningPoint, want.TurningPoint, tc.message)
		}
		if got.Regression != want.Regression {
			t.Errorf("Failed StepFit Got %#v Want %#v: %s", got.Regression, want.Regression, tc.message)
		}
	}
	// With a huge interesting value everything should be uninteresting.
	for _, tc := range testCases {
		got := GetStepFitAtMid(tc.value, 500)
		if math.IsInf(float64(got.Regression), 1) && math.IsInf(float64(got.Regression), -1) && got.Status != UNINTERESTING {
			t.Errorf("Failed StepFit Got %#v Want %#v: %v Regression %g", got.Status, UNINTERESTING, tc.value, got.Regression)
		}
	}
}
