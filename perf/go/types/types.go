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
	KMEANS_ALGO   ClusterAlgo = "kmeans"   // Cluster traces using k-means clustering on their shapes.
	STEPFIT_ALGO  ClusterAlgo = "stepfit"  // Look at each trace individually and determine if it steps up or down.
	ABSOLUTE_ALGO ClusterAlgo = "absolute" // Look at each trace individually and determine if it steps up or down by some value.
	PERCENT_ALGO  ClusterAlgo = "percent"  // Look at each trace individually and determine if it steps up or down by a certain percentage.

)

var (
	AllClusterAlgos = []ClusterAlgo{KMEANS_ALGO, STEPFIT_ALGO, ABSOLUTE_ALGO, PERCENT_ALGO}
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
