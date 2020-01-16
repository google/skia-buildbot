package stepfit

import (
	"math"

	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/types"
)

const (
	// The possible values for StepFit.Status are:

	LOW           = "Low"
	HIGH          = "High"
	UNINTERESTING = "Uninteresting"

	// MIN_SSE is the minimum sum squares error we'll accept.
	MIN_SSE = 10e-6
)

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
	Status string `json:"status"`
}

// GetStepFitAtMid takes one []float32 trace and calculates and returns a StepFit.
//
// See StepFit for a description of the values being calculated.
func GetStepFitAtMid(trace []float32, stddevThreshold float32, interesting float32, stepDetection types.StepDetection) *StepFit {
	var lse float32
	var regression float32
	stepSize := float32(-1.0)
	i := len(trace) / 2

	// Only normalize the trace if doing ORIGINAL_STEP.
	if stepDetection == types.ORIGINAL_STEP {
		trace = vec32.Dup(trace)
		vec32.Norm(trace, stddevThreshold)
	}

	// Now do different work based on stepDetection
	y0 := vec32.Mean(trace[:i])
	y1 := vec32.Mean(trace[i:])

	if stepDetection == types.ORIGINAL_STEP {
		lse := float32(math.MaxFloat32)
		if y0 != y1 {
			d := vec32.SSE(trace[:i], y0) + vec32.SSE(trace[i:], y1)
			if d < lse {
				lse = d
				stepSize = (y0 - y1)
			}
		}
		lse = float32(math.Sqrt(float64(lse))) / float32(len(trace))
		if lse < MIN_SSE {
			regression = stepSize / MIN_SSE
		} else {
			regression = stepSize / lse
		}
	} else if stepDetection == types.ABSOLUTE_STEP {
		stepSize = (y0 - y1)
		regression = stepSize
	} else if stepDetection == types.PERCENT_STEP {
		y := vec32.Mean(trace)
		stepSize = (y0 - y1) / (y)
		regression = stepSize
	} else /* Cohen's d */ {

	}
	status := UNINTERESTING
	if regression >= interesting {
		status = LOW
	} else if regression <= -interesting {
		status = HIGH
	}
	return &StepFit{
		LeastSquares: lse,
		StepSize:     stepSize,
		TurningPoint: i,
		Regression:   regression,
		Status:       status,
	}
}
