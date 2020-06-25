package stepfit

import (
	"math"

	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/types"
)

// StepFitStatus is the status of the StepFit.
type StepFitStatus string

const (
	// The possible values for StepFit.Status are:

	// LOW is a step down.
	LOW StepFitStatus = "Low"

	// HIGH is a step up.
	HIGH StepFitStatus = "High"

	// UNINTERESTING means no step occurred.
	UNINTERESTING StepFitStatus = "Uninteresting"

	// minTraceSize is the smallest trace length we can analyze.
	minTraceSize = 3
)

// AllStepFitStatus is the list of all StepFitStatus values.
var AllStepFitStatus = []StepFitStatus{LOW, HIGH, UNINTERESTING}

// StepFit stores information on the best Step Function fit on a trace.
//
// Used in ClusterSummary.
type StepFit struct {
	// LeastSquares is the Least Squares error for a step function curve fit to the trace.
	LeastSquares float32 `json:"least_squares"`

	// TurningPoint is the index where the Step Function changes value.
	TurningPoint int `json:"turning_point"`

	// StepSize is the size of the step in the step function. Negative values
	// indicate a step up, i.e. they look like a performance regression in the
	// trace, as opposed to positive values which look like performance
	// improvements.
	StepSize float32 `json:"step_size"`

	// The "Regression" value is calculated as Step Size / Least Squares Error.
	//
	// The better the fit the larger the number returned, because LSE
	// gets smaller with a better fit. The higher the Step Size the
	// larger the number returned.
	Regression float32 `json:"regression"`

	// Status of the cluster.
	//
	// Values can be "High", "Low", and "Uninteresting"
	Status StepFitStatus `json:"status"`
}

// NewStepFit creates an properly initialized StepFit struct.
func NewStepFit() *StepFit {
	return &StepFit{
		Status: UNINTERESTING,
	}
}

// GetStepFitAtMid takes one []float32 trace and calculates and returns a
// *StepFit.
//
// stddevThreshold is the minimum standard deviation allowed when normalizing
// traces to a standard deviation of 1.
//
// interesting is the threshold for a particular step to be flagged as a
// regression.
//
// stepDetection is the algorithm to use to test for a regression.
//
// See StepFit for a description of the values being calculated.
func GetStepFitAtMid(trace []float32, stddevThreshold float32, interesting float32, stepDetection types.StepDetection) *StepFit {
	ret := NewStepFit()
	if len(trace) < minTraceSize {
		return ret
	}
	// Only normalize the trace if doing ORIGINAL_STEP.
	if stepDetection == types.OriginalStep {
		trace = vec32.Dup(trace)
		vec32.Norm(trace, stddevThreshold)
	} else {
		// For all non-ORIGINAL_STEP regression types we use a symmetric (2*N)
		// trace, while in ORIGINAL_STEP uses the 2*N+1 length trace supplied.
		trace = trace[0 : len(trace)-1]
	}

	var lse float32
	var regression float32
	stepSize := float32(-1.0)
	i := len(trace) / 2

	// Now do different work based on stepDetection
	y0 := vec32.Mean(trace[:i])
	y1 := vec32.Mean(trace[i:])

	if stepDetection == types.OriginalStep {
		// This is the original recipe step detection as described at
		// https://bitworking.org/news/2014/11/detecting-benchmark-regressions
		lse := float32(math.MaxFloat32)
		if y0 != y1 {
			d := vec32.SSE(trace[:i], y0) + vec32.SSE(trace[i:], y1)
			if d < lse {
				lse = d
				stepSize = (y0 - y1)
			}
		}
		lse = float32(math.Sqrt(float64(lse))) / float32(len(trace))
		if lse < stddevThreshold {
			regression = stepSize / stddevThreshold
		} else {
			regression = stepSize / lse
		}
	} else if stepDetection == types.AbsoluteStep {
		// A simple check if the step size is greater than some absolute value.
		stepSize = (y0 - y1)
		regression = stepSize
	} else if stepDetection == types.PercentStep {
		// A simple check if the step size is greater than some percentage of
		// the mean of the first half of the trace.
		if len(trace) > 0 {
			stepSize = (y0 - y1) / (y0)
			regression = stepSize
		} else {
			stepSize = 0
			regression = stepSize
		}
	} else /* Cohen's d */ {
		// https://en.wikipedia.org/wiki/Effect_size#Cohen's_d
		if len(trace) < 4 {
			// The math for Cohen's d only makes sense for len(trace) >= 4.
			stepSize = 0
			regression = stepSize
		} else {
			s1 := vec32.StdDev(trace[:i], y0)
			s2 := vec32.StdDev(trace[i:], y1)
			s := (s1 + s2) / 2.0
			if math.IsNaN(float64(s)) || s < stddevThreshold {
				stepSize = (y0 - y1) / stddevThreshold
			} else {
				stepSize = (y0 - y1) / s
			}
			regression = stepSize
		}
	}
	status := UNINTERESTING
	if regression >= interesting {
		status = LOW
	} else if regression <= -interesting {
		status = HIGH
	}
	ret.Status = status
	ret.LeastSquares = lse
	ret.StepSize = stepSize
	ret.TurningPoint = i
	ret.Regression = regression
	return ret
}
