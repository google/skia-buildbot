// Package regression provides for tracking Perf regressions.
package regression

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

var ErrNoClusterFound = errors.New("No Cluster.")

// Status is used in TriageStatus.
type Status string

// ClusterType is used to denote type of cluster in regression2 schema.
type ClusterType string

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

	// Ignored means the regression has been ignored.
	Ignored Status = "ignored"

	// Available cluster types in regression2
	HighClusterType ClusterType = "high"
	LowClusterType  ClusterType = "low"
	NoneClusterType ClusterType = "none"

	// IgnoredMessage is the message used when a regression is ignored via the triage menu.
	IgnoredMessage = "Ignored via Triage Menu"
	// NudgedMessage is the message used when a regression is nudged.
	NudgedMessage = "Nudged"
	// ResetMessage is the message used when a regression triage status is reset.
	ResetMessage = ""
)

// AllStatus is a slice of all values of type Status.
var AllStatus = []Status{None, Positive, Negative, Untriaged, Ignored}

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
	Frame      *frame.FrameResponse        `json:"frame"` // Describes the Low and High ClusterSummary's.
	LowStatus  TriageStatus                `json:"low_status"`
	HighStatus TriageStatus                `json:"high_status"`

	// The fields below are only to be used with the regression2 schema.
	Id               string                `json:"id"`
	CommitNumber     types.CommitNumber    `json:"commit_number"`
	PrevCommitNumber types.CommitNumber    `json:"prev_commit_number"`
	AlertId          int64                 `json:"alert_id"`
	Bugs             []types.RegressionBug `json:"bugs"`
	AllBugsFetched   bool                  `json:"all_bugs_fetched"`
	CreationTime     time.Time             `json:"creation_time"`
	MedianBefore     float32               `json:"median_before"`
	MedianAfter      float32               `json:"median_after"`
	IsImprovement    bool                  `json:"is_improvement"`
	ClusterType      string                `json:"cluster_type"`
	SubscriptionName string                `json:"sub_name"`
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
		Id: uuid.NewString(),
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

// GetClusterTypeAndSummaryAndTriageStatus returns the cluster type, cluster summary
// and triage status objects for the regression.
func (r *Regression) GetClusterTypeAndSummaryAndTriageStatus() (ClusterType, *clustering2.ClusterSummary, TriageStatus) {
	if r.High != nil {
		return HighClusterType, r.High, r.HighStatus
	} else if r.Low != nil {
		return LowClusterType, r.Low, r.LowStatus
	} else {
		return NoneClusterType, nil, TriageStatus{}
	}
}
