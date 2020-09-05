// Package regression provides for tracking Perf regressions.
package regression

import (
	"encoding/json"
	"errors"
	"sync"

	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
)

var ErrNoClusterFound = errors.New("No Cluster.")

// Status is used in TriageStatus.
type Status string

// Status constants.
const (
	// None means there is no regression.
	None Status = ""

	// Positive means this change in performance is OK/expected.
	Positive Status = "positive"

	// Negative means this regression is a bug.
	Negative Status = "negative"

	// Untriaged means the regression has not been triaged.
	Untriaged Status = "untriaged"
)

// AllStatus is a slice of all values of type Status.
var AllStatus = []Status{None, Positive, Negative, Untriaged}

// AllRegressionsForCommit is a map[alertid]Regression.
type AllRegressionsForCommit struct {
	ByAlertID map[string]*Regression `json:"by_query"`
	mutex     sync.Mutex
}

// TriageStatus is the status of a found regression.
type TriageStatus struct {
	Status  Status `json:"status"`
	Message string `json:"message"`
}

// Regression tracks the status of the Low and High regression clusters, if they
// exist for a given CommitID and alertid.
//
// Note that Low and High can be nil if no regression has been found in that
// direction.
//
// TODO(jcgregorio) Now that we can search for regressions using GroupBy it is possible
// that Frame will only be valid for Low or High. Fix by refactoring Regression.
type Regression struct {
	Low        *clustering2.ClusterSummary `json:"low"`   // Can be nil.
	High       *clustering2.ClusterSummary `json:"high"`  // Can be nil.
	Frame      *dataframe.FrameResponse    `json:"frame"` // Describes the Low and High ClusterSummary's.
	LowStatus  TriageStatus                `json:"low_status"`
	HighStatus TriageStatus                `json:"high_status"`
}

// NewRegression returns a new *Regression.
func NewRegression() *Regression {
	return &Regression{
		LowStatus: TriageStatus{
			Status: None,
		},
		HighStatus: TriageStatus{
			Status: None,
		},
	}
}

// New returns a new *Regressions.
func New() *AllRegressionsForCommit {
	return &AllRegressionsForCommit{
		ByAlertID: map[string]*Regression{},
	}
}

// Merge the results from rhs into this Regression.
func (r *Regression) Merge(rhs *Regression) *Regression {
	if rhs.Low != nil {
		if r.Low != nil && (rhs.Low.StepFit.Regression > r.Low.StepFit.Regression) {
			r.Low = rhs.Low
			r.LowStatus = rhs.LowStatus
			r.Frame = rhs.Frame
		} else {
			r.Low = rhs.Low
			r.LowStatus = rhs.LowStatus
			r.Frame = rhs.Frame
		}
	}
	if rhs.High != nil {
		if r.High != nil && (rhs.High.StepFit.Regression < r.High.StepFit.Regression) {
			r.High = rhs.High
			r.HighStatus = rhs.HighStatus
			r.Frame = rhs.Frame
		} else {
			r.High = rhs.High
			r.HighStatus = rhs.HighStatus
			r.Frame = rhs.Frame
		}
	}
	return r
}

// Triaged returns true if triaged.
func (r *Regression) Triaged() bool {
	ret := true
	ret = ret && (r.HighStatus.Status != Untriaged)
	ret = ret && (r.LowStatus.Status != Untriaged)
	return ret
}

// JSON returns the Regressions serialized as JSON. Use this instead of
// serializing Regression directly as it holds the mutex while serializing.
func (r *AllRegressionsForCommit) JSON() ([]byte, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return json.Marshal(r)
}
