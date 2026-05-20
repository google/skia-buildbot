package stepfit

import (
	"math"

	"github.com/aclements/go-moremath/stats"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/alerts"
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

	// MinTraceSize is the smallest trace length we can analyze.
	MinTraceSize = 3
)

// AllStepFitStatus is the list of all StepFitStatus values.
var AllStepFitStatus = []StepFitStatus{LOW, HIGH, UNINTERESTING}

// AnomalyResult represents the evaluation of a single algorithm against a threshold.
type AnomalyResult struct {
	AlgoName      string        `json:"algo"`
	Value         float32       `json:"value"`
	Threshold     float32       `json:"threshold"`
	IsAnomaly     bool          `json:"is_anomaly"`
	StepSize      float32       `json:"step_size"`
	LeastSquares  float32       `json:"least_squares"`
	Status        StepFitStatus `json:"status"`
	RawRegression float32       `json:"raw_regression"`
	TurningPoint  int           `json:"turning_point"`
}

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

	// RuleEvaluations is the flat list of evaluated anomaly rules, sorted by anomaly status and algorithm priority.
	RuleEvaluations []AnomalyResult `json:"rule_evaluation,omitempty"`
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

// CalculateStepFitValues calculates the core values for a step fit, returning an AnomalyResult.
func CalculateStepFitValues(trace []float32, stddevThreshold float32, simpleRule *alerts.AlgorithmCheck) (bool, AnomalyResult) {
	i := len(trace) / 2
	y0 := vec32.Mean(trace[:i])
	y1 := vec32.Mean(trace[i:])

	var lse float32 = InvalidLeastSquaresError
	var regression float32
	stepSize := float32(-1.0)
	status := UNINTERESTING

	s1 := vec32.StdDev(trace[:i], y0)
	s2 := vec32.StdDev(trace[i:], y1)
	n1 := i
	n2 := len(trace) - i
	stepDetection := simpleRule.Step
	interesting := simpleRule.Threshold

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
	} else if stepDetection == types.Stepiness {
		sse1 := vec32.SSE(trace[:i], y0)
		sse2 := vec32.SSE(trace[i:], y1)
		meanTotal := vec32.Mean(trace)
		sseTotal := vec32.SSE(trace, meanTotal)
		stepSize, regression = CalcStepinessStep(y0, y1, sse1, sse2, sseTotal)
	} else /* types.MannWhitneyU  */ {
		var err error
		sample1 := vec32.ToFloat64(trace[:i])
		sample2 := vec32.ToFloat64(trace[i:])
		stepSize, regression, lse, err = CalcMannWhitneyStep(y0, y1, sample1, sample2)
		if err != nil {
			return false, AnomalyResult{}
		}
	}

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

	actualValue := float32(math.Abs(float64(regression)))
	isTriggered := status != UNINTERESTING

	result := AnomalyResult{
		AlgoName:      string(simpleRule.Step),
		Value:         actualValue,
		Threshold:     simpleRule.Threshold,
		IsAnomaly:     isTriggered,
		StepSize:      stepSize,
		LeastSquares:  lse,
		Status:        status,
		RawRegression: regression,
		TurningPoint:  i,
	}

	return true, result
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
	stepSize := float32(-1.0)
	regression := val
	if val < 0 {
		regression = -val
	}
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

// CalcValidCohenStep calculates step size and regression for Cohen's d effect size.
// It uses the pooled standard deviation.
// https://en.wikipedia.org/wiki/Effect_size#Cohen's_d
func CalcValidCohenStep(y0, y1, s1, s2 float32, n1, n2 int, stddevThreshold float32) (float32, float32) {
	if n1+n2 < 3 {
		return 0, 0
	}

	var s_p float32
	denominator := float32(n1 + n2 - 2)
	if denominator > 0 {
		s_p = float32(math.Sqrt(float64(((float32(n1-1)*s1*s1 + float32(n2-1)*s2*s2) / denominator))))
	} else {
		s_p = (s1 + s2) / 2.0
	}

	var stepSize float32
	if math.IsNaN(float64(s_p)) || s_p < stddevThreshold {
		stepSize = (y0 - y1) / stddevThreshold
	} else {
		stepSize = (y0 - y1) / s_p
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

// CalcStepinessStep calculates step size and regression for Stepiness detection.
// It returns a number between 0 and 1 that indicates how step-like the trace is.
//
// This is mathematically equivalent to the legacy Python catapult implementation:
//
//	normalized = (values - mean) / stddev
//	step_values = [mean(normalized[:i])..., mean(normalized[i:])...]
//	RMSD = sqrt(sum((normalized - step_values)^2) / N)
//	steppiness = 1.0 - RMSD
//
// Proof of equivalence:
//
//	RMSD = sqrt( (SSE_left / stddev^2 + SSE_right / stddev^2) / N )
//	RMSD = sqrt( (SSE_left + SSE_right) / (stddev^2 * N) )
//
// Since stddev^2 * N = SSE_total:
//
//	RMSD = sqrt( (SSE_left + SSE_right) / SSE_total )
func CalcStepinessStep(y0, y1, sse1, sse2, sseTotal float32) (float32, float32) {
	stepSize := y0 - y1
	var steppiness float32
	if sseTotal > 0 {
		steppiness = 1.0 - float32(math.Sqrt(float64((sse1+sse2)/sseTotal)))
	}

	// Regression needs a sign to indicate direction (step up vs step down)
	// to be compatible with CalculateStepFitValues threshold logic.
	regression := steppiness
	if stepSize < 0 {
		regression = -steppiness
	}
	return stepSize, regression
}
