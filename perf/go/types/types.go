package types

import (
	"fmt"

	"go.skia.org/infra/go/vec32"
)

// Trace is just a slice of float32s.
type Trace []float32

// NewTrace returns a Trace of length 'traceLen' initialized to vec32.MISSING_DATA_SENTINEL.
func NewTrace(traceLen int) Trace {
	return Trace(vec32.New(traceLen))
}

// TraceSet is a set of Trace's, keyed by trace id.
type TraceSet map[string]Trace

// Progress is a func that is called to update the progress on a computation.
type Progress func(step, totalSteps int)

type ClusterAlgo string

// ClusterAlgo constants.
//
// Update algo-select-sk if this enum is changed.
const (
	KMEANS_ALGO  ClusterAlgo = "kmeans"  // Cluster traces using k-means clustering on their shapes.
	STEPFIT_ALGO ClusterAlgo = "stepfit" // Look at each trace individually and determine if it steps up or down.
)

// StepDetection are the different ways we can look at an individual trace, or a
// cluster centroid (which is also a single trace), and detect if a step has
// occurred.
type StepDetection string

const (
	// ORIGINAL_STEP is the original type of step detection. Note we leave as
	// empty string so we pick up the right default from old alerts.
	ORIGINAL_STEP StepDetection = ""

	// ABSOLUTE_STEP is a step detection that looks for an absolute magnitude
	// change.
	ABSOLUTE_STEP StepDetection = "absolute"

	// PERCENT_STEP is a simple check if the step size is greater than some
	// percentage of the mean of the first half of the trace.
	PERCENT_STEP StepDetection = "percent"

	// COHEN_STEP uses Cohen's d method to detect a change. https://en.wikipedia.org/wiki/Effect_size#Cohen's_d
	COHEN_STEP StepDetection = "cohen"
)

var (
	AllClusterAlgos = []ClusterAlgo{
		KMEANS_ALGO,
		STEPFIT_ALGO,
	}

	AllStepDetections = []StepDetection{
		ORIGINAL_STEP,
		ABSOLUTE_STEP,
		PERCENT_STEP,
		COHEN_STEP,
	}
)

func ToClusterAlgo(s string) (ClusterAlgo, error) {
	ret := ClusterAlgo(s)
	for _, c := range AllClusterAlgos {
		if c == ret {
			return ret, nil
		}
	}
	return ret, fmt.Errorf("%q is not a valid ClusterAlgo, must be a value in %v", s, AllClusterAlgos)
}

func ToStepDetection(s string) (StepDetection, error) {
	ret := StepDetection(s)
	for _, c := range AllStepDetections {
		if c == ret {
			return ret, nil
		}
	}
	return ret, fmt.Errorf("%q is not a valid StepDetection, must be a value is %v", s, AllStepDetections)
}
