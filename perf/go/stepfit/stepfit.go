package stepfit

import (
	"math"

	"github.com/aclements/go-moremath/stats"
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
	// LeastSquares is the Least Squares error for a step function curve fit to
	// the trace. Will be set to InvalidLeastSquaresError if LSE isn't
	// calculated for a given algorithm.
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

// InvalidLeastSquaresError signals that the value of StepFit.LeastSquares is
// invalid, i.e. it is not calculated for the given algorithm.
const InvalidLeastSquaresError = -1

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

	var lse float32 = InvalidLeastSquaresError

	var regression float32
	stepSize := float32(-1.0)
	i := len(trace) / 2

	y0 := vec32.Mean(trace[:i])
	y1 := vec32.Mean(trace[i:])
	s1 := vec32.StdDev(trace[:i], y0)
	s2 := vec32.StdDev(trace[i:], y1)
	n1 := i
	n2 := len(trace) - i

	// Now do different work based on stepDetection
	if stepDetection == types.OriginalStep {
		sse1 := vec32.SSE(trace[:i], y0)
		sse2 := vec32.SSE(trace[i:], y1)
		stepSize, regression, lse = CalcOriginalStep(y0, y1, sse1, sse2, len(trace), stddevThreshold)
	} else if stepDetection == types.AbsoluteStep {
		stepSize, regression = CalcAbsoluteStep(y0, y1)
	} else if stepDetection == types.Const {
		// Const uses the value at the turning point.
		val := trace[i]
		stepSize, regression = CalcConstStep(val, interesting)
	} else if stepDetection == types.PercentStep {
		stepSize, regression = CalcPercentStep(y0, y1)
	} else if stepDetection == types.CohenStep {
		stepSize, regression = CalcCohenStep(y0, y1, s1, s2, n1, n2, stddevThreshold)
	} else /* types.MannWhitneyU  */ {
		var err error
		sample1 := vec32.ToFloat64(trace[:i])
		sample2 := vec32.ToFloat64(trace[i:])
		stepSize, regression, lse, err = CalcMannWhitneyStep(y0, y1, sample1, sample2)
		if err != nil {
			return ret
		}
	}

	status := UNINTERESTING
	if stepDetection == types.MannWhitneyU {
		// There is a different interpretation of regression for MannWhitneyU,
		// where regression = p. That is, when doing a hypothesis test we want
		// to see if p < 0.05, for example. So that only tells us if a
		// regression has occurred, i.e. we rejected the null hypothesis, so we
		// need to use the sign of stepSize to determine the direction (status).
		if regression <= interesting {
			if stepSize < 0 {
				status = HIGH
				regression *= -1
			} else {
				status = LOW
			}
		}
	} else {
		if regression >= interesting {
			status = LOW
		} else if regression <= -interesting {
			status = HIGH
		}
	}
	ret.Status = status
	ret.LeastSquares = lse
	ret.StepSize = stepSize
	ret.TurningPoint = i
	ret.Regression = regression
	return ret
}

// CalcOriginalStep calculates step size, regression, and LSE for OriginalStep detection.
// sse1 and sse2 are the sum squared errors of the left and right sides respectively.
// totalN is the total length of the trace (used for LSE normalization).
// This is the original recipe step detection as described at
// https://bitworking.org/news/2014/11/detecting-benchmark-regressions
func CalcOriginalStep(y0, y1, sse1, sse2 float32, totalN int, stddevThreshold float32) (float32, float32, float32) {
	lse := float32(math.MaxFloat32)
	stepSize := float32(-1.0)

	if y0 != y1 {
		d := sse1 + sse2
		if d < lse {
			lse = d
			stepSize = (y0 - y1)
		}
	}
	// The next line of code should actually be math.Sqrt(lse/len(trace))
	// instead it is math.Sqrt(lse)/len(trace), which does not give the stddev.
	lse = float32(math.Sqrt(float64(lse))) / float32(totalN)

	var regression float32
	if lse < stddevThreshold {
		regression = stepSize / stddevThreshold
	} else {
		regression = stepSize / lse
	}
	return stepSize, regression, lse
}

// CalcAbsoluteStep calculates step size and regression for AbsoluteStep detection.
// A simple check if the step size is greater than some absolute value.
func CalcAbsoluteStep(y0, y1 float32) (float32, float32) {
	stepSize := (y0 - y1)
	return stepSize, stepSize
}

// CalcConstStep calculates step size and regression for Const detection.
// A simple check if the absolute value of the trace is greater than some constant value.
func CalcConstStep(val float32, interesting float32) (float32, float32) {
	absTraceValue := float32(math.Abs(float64(val)))
	stepSize := absTraceValue - interesting
	regression := -1 * absTraceValue // * -1 So that regressions get flagged as HIGH.
	return stepSize, regression
}

// CalcPercentStep calculates step size and regression for PercentStep detection.
// It checks the percentage difference between two means (y0 and y1) relative to y0.
func CalcPercentStep(y0, y1 float32) (float32, float32) {
	stepSize := (y0 - y1) / y0 // The division can produce +/-Inf or NaN.
	if math.IsInf(float64(stepSize), 0) {
		stepSize = math.MaxFloat32
		if y0 < y1 {
			stepSize *= -1
		}
	}
	if math.IsNaN(float64(stepSize)) {
		stepSize = 0
	}
	return stepSize, stepSize
}

// CalcCohenStep calculates step size and regression for CohenStep detection.
// https://en.wikipedia.org/wiki/Effect_size#Cohen's_d
func CalcCohenStep(y0, y1, s1, s2 float32, n1, n2 int, stddevThreshold float32) (float32, float32) {
	if n1+n2 < 4 {
		return 0, 0
	}
	s := (s1 + s2) / 2.0

	var stepSize float32
	if math.IsNaN(float64(s)) || s < stddevThreshold {
		stepSize = (y0 - y1) / stddevThreshold
	} else {
		stepSize = (y0 - y1) / s
	}
	return stepSize, stepSize
}

// CalcMannWhitneyStep calculates step size, regression (p-value), and LSE (U-statistic) for MannWhitneyU detection.
func CalcMannWhitneyStep(y0, y1 float32, sample1, sample2 []float64) (float32, float32, float32, error) {
	mwResults, err := stats.MannWhitneyUTest(sample1, sample2, stats.LocationDiffers)
	if err != nil {
		return 0, 0, 0, err
	}
	stepSize := (y0 - y1)
	regression := float32(mwResults.P)
	lse := float32(mwResults.U)
	return stepSize, regression, lse, nil
}
