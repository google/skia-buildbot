package types

import (
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/vec32"
)

// CommitNumber is the offset of any commit from the first commit in a repo.
// That is, the first commit is 0. The presumes that all commits are linearly
// ordered, i.e. no tricky branch merging.
type CommitNumber int32

// BadCommitNumber is an invalid CommitNumber.
const BadCommitNumber CommitNumber = -1

// Add an offset to a CommitNumber and return the resulting CommitNumber.
func (c CommitNumber) Add(offset int32) CommitNumber {
	ret := c + CommitNumber(offset)
	if ret < 0 {
		return BadCommitNumber
	}
	return ret
}

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

// TraceCommitLink contains data for a link to show for a trace data point.
type TraceCommitLink struct {
	// Text is the display text for the link.
	Text string

	// Href is the target url for the link.
	Href string
}

// TraceMetadata is a struct to define metadata for a trace.
type TraceMetadata struct {
	// TraceID is the string id of the trace.
	TraceID string `json:"traceid"`

	// CommitLinks is a map where the key is a commit number and the value is
	// a map containing commit links.
	CommitLinks map[CommitNumber]map[string]TraceCommitLink `json:"commitLinks"`
}

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

	// RatioStep is exactly like OriginalStep except it uses RMSE as the 
	// fitness function to calculate the regression ratio.
	RatioStep StepDetection = "ratio"

	// AbsoluteStep is a step detection that looks for an absolute magnitude
	// change.
	AbsoluteStep StepDetection = "absolute"

	// Const is a step detection that detects if the absolute value of the trace
	// value exceeds some constant.
	Const StepDetection = "const"

	// PercentStep is a simple check if the step size is greater than some
	// percentage of the mean of the first half of the trace.
	PercentStep StepDetection = "percent"

	// CohenStep uses Cohen's d method to detect a change. https://en.wikipedia.org/wiki/Effect_size#Cohen's_d
	CohenStep StepDetection = "cohen"

	// MannWhitneyU uses the Mann-Whitney U test to detect a change. https://en.wikipedia.org/wiki/Mann%E2%80%93Whitney_U_test
	MannWhitneyU StepDetection = "mannwhitneyu"
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
		Const,
		PercentStep,
		CohenStep,
		MannWhitneyU,
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

// AlertAction defines the action to trigger.
type AlertAction string

const (
	// NoAction means no action is needed for anomalies detected by this alert.
	NoAction AlertAction = "noaction"

	// Report means the anomalies detected by this alert should only create a
	// new issue or update an existing one.
	FileIssue AlertAction = "report"

	// Bisect means the anomalies detected by this alert should trigger a
	// bisection job to drill down to the culprit.
	Bisection AlertAction = "bisect"
)

var (
	// AllStepDetections is a list of all valid StepDetections.
	AllAlertActions = []AlertAction{
		NoAction,
		FileIssue,
		Bisection,
	}
)

// All valid stat suffix from perf measurements.
var (
	AllMeasurementStats = []string{
		"avg", "count", "max", "min", "std", "sum",
	}
)

type AnomalyDetectionNotifyType string

const (
	// IssueTracker means send Markdown formatted notifications to the
	// issue tracker.
	IssueNotify AnomalyDetectionNotifyType = "issuetracker"
	// None means do not send any notification.
	NoneNotify AnomalyDetectionNotifyType = "none"
)

// AllAnomalyDetectionNotifyTypes is the list of all valid AnomalyDetectionNotifyTypes.
var AllAnomalyDetectionNotifyTypes []AnomalyDetectionNotifyType = []AnomalyDetectionNotifyType{IssueNotify, NoneNotify}

// ProjectId defines the action to trigger.
type ProjectId string

var (
	// AllProjectIds is a list of all project ids.
	AllProjectIds = []ProjectId{
		"chromium",
	}
)

// TraceSourceInfo provides a struct to abstract sourceInfo data and operations
// for traces extracted from database.
type TraceSourceInfo struct {
	// Map where the key is a commit number and the value is a source file id.
	sourceMap map[CommitNumber]int64

	// Mutex to synchronize operations on the map.
	mutex sync.RWMutex
}

// NewTraceSourceInfo returns a new TraceSourceInfo object.
func NewTraceSourceInfo() *TraceSourceInfo {
	return &TraceSourceInfo{
		sourceMap: map[CommitNumber]int64{},
		mutex:     sync.RWMutex{},
	}
}

// Add adds a new source file id for a commit number to the info.
func (ts *TraceSourceInfo) Add(commitNumber CommitNumber, sourceFileID int64) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	if ts.sourceMap == nil {
		ts.sourceMap = map[CommitNumber]int64{}
	}
	ts.sourceMap[commitNumber] = sourceFileID
}

// Get retrieves the source file id for a commit number.
// Returns a false value when commit number is not present.
func (ts *TraceSourceInfo) Get(commitNumber CommitNumber) (int64, bool) {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()
	if ts.sourceMap == nil {
		ts.sourceMap = map[CommitNumber]int64{}
	}
	sourceFileID, ok := ts.sourceMap[commitNumber]
	return sourceFileID, ok
}

// CopyFrom copies the data from the provided TraceSourceInfo object
// into the current one.
func (ts *TraceSourceInfo) CopyFrom(other *TraceSourceInfo) {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()
	if ts.sourceMap == nil {
		ts.sourceMap = map[CommitNumber]int64{}
	}
	// We need to lock the 'other' object as well to safely read its map.
	other.mutex.RLock()
	defer other.mutex.RUnlock()
	for commitNumber, sourceFileID := range other.sourceMap {
		ts.sourceMap[commitNumber] = sourceFileID
	}
}

// GetAllSourceFileIds returns all the sourceFileIds added to the object.
func (ts *TraceSourceInfo) GetAllSourceFileIds() []int64 {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()
	ret := []int64{}
	if ts.sourceMap != nil {
		for _, sourceFileID := range ts.sourceMap {
			ret = append(ret, sourceFileID)
		}
	}
	return ret
}

// GetAllCommitNumbers returns all the commit numbers added to the object.
func (ts *TraceSourceInfo) GetAllCommitNumbers() []CommitNumber {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()
	ret := []CommitNumber{}
	if ts.sourceMap != nil {
		for commitNumber := range ts.sourceMap {
			ret = append(ret, commitNumber)
		}
	}
	return ret
}

type BugType string

const (
	ManualTriage = "manual"
	AutoTriage   = "auto-triage"
	AutoBisect   = "auto-bisect"
)

// RegressionBug is a type that binds bug id and it's source together.
// In other words, it allows us to determine which sheriff action created this association.
type RegressionBug struct {
	BugId string  `json:"bug_id"`
	Type  BugType `json:"bug_type"`
}
