package types

import (
	"encoding/gob"
	"fmt"
	"time"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/config"
)

// PerfTrace represents all the values of a single floating point measurement.
// *PerfTrace implements Trace.
type PerfTrace struct {
	Values  []float64         `json:"values"`
	Params_ map[string]string `json:"params"`
}

func (t *PerfTrace) Params() map[string]string {
	return t.Params_
}

func (t *PerfTrace) Len() int {
	return len(t.Values)
}

func (t *PerfTrace) IsMissing(i int) bool {
	return t.Values[i] == config.MISSING_DATA_SENTINEL
}

func (t *PerfTrace) DeepCopy() tiling.Trace {
	n := len(t.Values)
	cp := &PerfTrace{
		Values:  make([]float64, n, n),
		Params_: make(map[string]string),
	}
	copy(cp.Values, t.Values)
	for k, v := range t.Params_ {
		cp.Params_[k] = v
	}
	return cp
}

func (t *PerfTrace) Merge(next tiling.Trace) tiling.Trace {
	nextPerf := next.(*PerfTrace)
	n := len(t.Values) + len(nextPerf.Values)
	n1 := len(t.Values)

	merged := NewPerfTraceN(n)
	merged.Params_ = t.Params_
	for k, v := range nextPerf.Params_ {
		merged.Params_[k] = v
	}
	for i, v := range t.Values {
		merged.Values[i] = v
	}
	for i, v := range nextPerf.Values {
		merged.Values[n1+i] = v
	}
	return merged
}

func (t *PerfTrace) Grow(n int, fill tiling.FillType) {
	if n < len(t.Values) {
		panic(fmt.Sprintf("Grow must take a value (%d) larger than the current Trace size: %d", n, len(t.Values)))
	}
	delta := n - len(t.Values)
	newValues := make([]float64, n)

	if fill == tiling.FILL_AFTER {
		copy(newValues, t.Values)
		for i := 0; i < delta; i++ {
			newValues[i+len(t.Values)] = config.MISSING_DATA_SENTINEL
		}
	} else {
		for i := 0; i < delta; i++ {
			newValues[i] = config.MISSING_DATA_SENTINEL
		}
		copy(newValues[delta:], t.Values)
	}
	t.Values = newValues
}

func (g *PerfTrace) Trim(begin, end int) error {
	if end < begin || end > g.Len() || begin < 0 {
		return fmt.Errorf("Invalid Trim range [%d, %d) of [0, %d]", begin, end, g.Len())
	}
	n := end - begin
	newValues := make([]float64, n)

	for i := 0; i < n; i++ {
		newValues[i] = g.Values[i+begin]
	}
	g.Values = newValues
	return nil
}

// NewPerfTrace allocates a new Trace set up for the given number of samples.
//
// The Trace Values are pre-filled in with the missing data sentinel since not
// all tests will be run on all commits.
func NewPerfTrace() *PerfTrace {
	return NewPerfTraceN(tiling.TILE_SIZE)
}

// NewPerfTraceN allocates a new Trace set up for the given number of samples.
//
// The Trace Values are pre-filled in with the missing data sentinel since not
// all tests will be run on all commits.
func NewPerfTraceN(n int) *PerfTrace {
	t := &PerfTrace{
		Values:  make([]float64, n, n),
		Params_: make(map[string]string),
	}
	for i, _ := range t.Values {
		t.Values[i] = config.MISSING_DATA_SENTINEL
	}
	return t
}

func init() {
	// Register *PerfTrace in gob so that it can be used as a
	// concrete type for Trace when writing and reading Tiles in gobs.
	gob.Register(&PerfTrace{})
}

type TryBotResults struct {
	// Map from Trace key to value.
	Values map[string]float64
}

func NewTryBotResults() *TryBotResults {
	return &TryBotResults{
		Values: map[string]float64{},
	}
}

// ValueWeight is a weight proportional to the number of times the parameter
// Value appears in a cluster. Used in ClusterSummary.
type ValueWeight struct {
	Value  string
	Weight int
}

// StepFit stores information on the best Step Function fit on a trace.
//
// Used in ClusterSummary.
type StepFit struct {
	// LeastSquares is the Least Squares error for a step function curve fit to the trace.
	LeastSquares float64

	// TurningPoint is the index where the Step Function changes value.
	TurningPoint int

	// StepSize is the size of the step in the step function. Negative values
	// indicate a step up, i.e. they look like a performance regression in the
	// trace, as opposed to positive values which look like performance
	// improvements.
	StepSize float64

	// The "Regression" value is calculated as Step Size / Least Squares Error.
	//
	// The better the fit the larger the number returned, because LSE
	// gets smaller with a better fit. The higher the Step Size the
	// larger the number returned.
	Regression float64

	// Status of the cluster.
	//
	// Values can be "High", "Low", and "Uninteresting"
	Status string
}

// ClusterSummary is a summary of a single cluster of traces.
type ClusterSummary struct {
	// Traces contains at most config.MAX_SAMPLE_TRACES_PER_CLUSTER sample
	// traces, the first is the centroid.
	Traces [][][]float64

	// Keys of all the members of the Cluster.
	Keys []string

	// ParamSummaries is a summary of all the parameters in the cluster.
	ParamSummaries [][]ValueWeight

	// StepFit is info on the fit of the centroid to a step function.
	StepFit *StepFit

	// Hash is the Git hash at the step point.
	Hash string

	// Timestamp is when this hash was committed.
	Timestamp int64

	// Status is the status, "New", "Ingore" or "Bug".
	Status string

	// A note about the Status.
	Message string

	// ID is the identifier for this summary in the datastore.
	ID int64

	// Bugs is a list of IDs of bugs in the codesite issue tracker.
	Bugs []int64
}

// ValidStatusValues are the valid values of ClusterSummary.Status when the
// ClusterSummary is used as an alert.
var ValidStatusValues = []string{"New", "Ignore", "Bug"}

func NewClusterSummary(numKeys, numTraces int) *ClusterSummary {
	return &ClusterSummary{
		Keys:           make([]string, numKeys),
		Traces:         make([][][]float64, numTraces),
		ParamSummaries: [][]ValueWeight{},
		StepFit:        &StepFit{},
		Hash:           "",
		Timestamp:      0,
		Status:         "",
		Message:        "",
		ID:             -1,
	}
}

// Merge adds in new info from the passed in ClusterSummary.
func (c *ClusterSummary) Merge(from *ClusterSummary) {
	for _, k := range from.Keys {
		if !util.In(k, c.Keys) {
			c.Keys = append(c.Keys, k)
		}
	}
}

// Activity stores information on one user action activity. This corresponds to
// one record in the activity database table. See DESIGN.md for details.
type Activity struct {
	ID     int
	TS     int64
	UserID string
	Action string
	URL    string
}

// Date returns an RFC3339 string for the Activity's TS.
func (a *Activity) Date() string {
	return time.Unix(a.TS, 0).Format(time.RFC3339)
}
