package types

import (
	"fmt"
	"time"

	"go.skia.org/infra/go/vec32"
)

// CommitNumber is the offset of any commit from the first commit in a repo.
// That is, the first commit is 0. The presumes that all commits are linearly
// ordered, i.e. no tricky branch merging.
type CommitNumber int32

// BadCommitNumber is an invalid CommitNumber.
const BadCommitNumber CommitNumber = -1

// CommitNumberSlice is a utility class for sorting CommitNumbers.
type CommitNumberSlice []CommitNumber

func (p CommitNumberSlice) Len() int           { return len(p) }
func (p CommitNumberSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p CommitNumberSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// TileNumber is the number of a Tile in the TraceStore. The first tile is
// always 0. The number of commits per Tile is configured per TraceStore.
type TileNumber int32

// BadTileNumber is an invalid TileNumber.
const BadTileNumber TileNumber = -1

// Prev returns the number of the previous tile.
//
// May return a BadTileNumber.
func (t TileNumber) Prev() TileNumber {
	t = t - 1
	if t < 0 {
		return BadTileNumber
	}
	return t
}

// TileNumberFromCommitNumber converts a CommitNumber into a TileNumber given
// the tileSize.
func TileNumberFromCommitNumber(commitNumber CommitNumber, tileSize int32) TileNumber {
	if tileSize <= 0 {
		return BadTileNumber
	}
	return TileNumber(int32(commitNumber) / tileSize)
}

// TileCommitRangeForTileNumber returns the first and last CommitNumbers that
// would appear in a tile of size tileSize.
func TileCommitRangeForTileNumber(tileNumber TileNumber, tileSize int32) (CommitNumber, CommitNumber) {
	return CommitNumber(int32(tileNumber) * tileSize), CommitNumber((int32(tileNumber)+1)*tileSize - 1)
}

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

// RegressionDetectionGrouping is how traces are grouped when regression detection is done.
type RegressionDetectionGrouping string

// RegressionDetectionGrouping constants.
//
// Update algo-select-sk if this enum is changed.
const (
	KMeansGrouping  RegressionDetectionGrouping = "kmeans"  // Cluster traces using k-means clustering on their shapes.
	StepFitGrouping RegressionDetectionGrouping = "stepfit" // Look at each trace individually and determine if it steps up or down.
)

// StepDetection are the different ways we can look at an individual trace, or a
// cluster centroid (which is also a single trace), and detect if a step has
// occurred.
type StepDetection string

const (
	// OriginalStep is the original type of step detection. Note we leave as
	// empty string so we pick up the right default from old alerts.
	OriginalStep StepDetection = ""

	// AbsoluteStep is a step detection that looks for an absolute magnitude
	// change.
	AbsoluteStep StepDetection = "absolute"

	// PercentStep is a simple check if the step size is greater than some
	// percentage of the mean of the first half of the trace.
	PercentStep StepDetection = "percent"

	// CohenStep uses Cohen's d method to detect a change. https://en.wikipedia.org/wiki/Effect_size#Cohen's_d
	CohenStep StepDetection = "cohen"
)

var (
	// AllClusterAlgos is a list of all valid RegressionDetectionGroupings.
	AllClusterAlgos = []RegressionDetectionGrouping{
		KMeansGrouping,
		StepFitGrouping,
	}

	// AllStepDetections is a list of all valid StepDetections.
	AllStepDetections = []StepDetection{
		OriginalStep,
		AbsoluteStep,
		PercentStep,
		CohenStep,
	}
)

// ToClusterAlgo converts a string to a RegressionDetectionGrouping
func ToClusterAlgo(s string) (RegressionDetectionGrouping, error) {
	ret := RegressionDetectionGrouping(s)
	for _, c := range AllClusterAlgos {
		if c == ret {
			return ret, nil
		}
	}
	return ret, fmt.Errorf("%q is not a valid ClusterAlgo, must be a value in %v", s, AllClusterAlgos)
}

// ToStepDetection converts a string to a StepDetection.
func ToStepDetection(s string) (StepDetection, error) {
	ret := StepDetection(s)
	for _, c := range AllStepDetections {
		if c == ret {
			return ret, nil
		}
	}
	return ret, fmt.Errorf("%q is not a valid StepDetection, must be a value is %v", s, AllStepDetections)
}

// Domain represents the range of commits over which to do some work, such as
// searching for regressions.
type Domain struct {
	// N is the number of commits.
	N int32 `json:"n"`

	// End is the time when our range of N commits should end.
	End time.Time `json:"end"`

	// Offset is the exact commit we are interested in. If non-zero then ignore
	// both N and End.
	Offset int32 `json:"offset"`
}

// ProgressCallback if a func that's called to return information on a currently running process.
type ProgressCallback func(message string)

// CL is the identifier for a change list, or pull request in GitHub
// lingo.
type CL string
