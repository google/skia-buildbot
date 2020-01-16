package absolute

import (
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/stepfit"
)

// GetStepFitAtMid takes one []float32 trace and calculates and returns a
// StepFit.
//
// See StepFit for a description of the values being calculated.
//
// If percent is true then use delta as a percent difference, otherwise use
// delta as an absolute difference between to two means.
func GetStepFitAtMid(trace []float32, delta float32, percent bool) *stepfit.StepFit {
	stepSize := float32(0)

	i := len(trace) / 2
	y0 := vec32.Mean(trace[:i])
	y1 := vec32.Mean(trace[i:])

	if y0 != y1 {
		if percent {
			y := vec32.Mean(trace)
			stepSize = (y0 - y1) / (y)
		} else {
			stepSize = (y0 - y1)
		}
		sklog.Warningf("stepSize: %f", stepSize)
	}
	status := stepfit.UNINTERESTING
	if stepSize >= delta {
		status = stepfit.LOW
	} else if stepSize <= -delta {
		status = stepfit.HIGH
	}
	return &stepfit.StepFit{
		LeastSquares: 0,
		StepSize:     stepSize,
		TurningPoint: i,
		Regression:   stepSize,
		Status:       status,
	}
}
