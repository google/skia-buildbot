// Package regression provides for tracking Perf regressions.
package regression

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

var ErrNoClusterFound = errors.New("No Cluster.")

// Status is used in TriageStatus.
type Status string

// ClusterType is used to denote type of cluster in regression2 schema.
type ClusterType string

// To learn more about statuses allowed in Regressions2 anomalies,
// see go/perf-chrome-anom-mig [1], section "Regression states".
// For a more detailed explanation behind those, see go/perf-ag-statuses [2].

// Status constants used by both (Legacy) Regressions and Regressions2.
const (
	// Ignored means the regression has been ignored by sheriff,
	// or that autobisection thinks the anomaly is insignificant and sheriff doesn't want to
	// investigate. By default, the latter will end up in the Untriaged state, and we will implement
	// "auto-ignore" only on demand. But too large volume of such anomalies likely means that
	// our detection algos are too sensitive, and we can save some resources by investigating.
	Ignored Status = "ignored"
)

// Regressions2 Status constants.
const (
	// Anomaly hasn't yet been grouped, group still can grow, or autobisection is in progress.
	Pending Status = "pending"

	// Untriaged means that Sheriff should triage it.
	// This corresponds to "Sheriff's attention required" section in [1].
	// There are a few cases:
	// - anomaly grouping is not enabled,
	// - action: IGNORE defined in sheriff config,
	// - autobisection failed (i.e. timeout or no bots),
	// - autobisection finished with INSIGNIFICANT status - by default, we do not ignore them,
	// but rather let Sheriffs change thresholds so fewer bisect jobs are kicked off. We need to mark
	// such regressions differently, let's use Triage Message: autobisection - insignificant.
	Untriaged Status = "untriaged"

	// Triaged means one of the following:
	// - Action: IGNORE - sheriff manually filed a bug against this anomaly (or assigned)
	// - Action: REPORT - a bug has been created automatically
	// - Action: BISECT - autobisection found a culprit (and filed a bug automatically), or no culprit
	// was found and a bug has been filed against sheriff to investigate - regression is there, still.
	// We mark the latter with Triage Message: autobisection - no culprit found.
	Triaged Status = "triaged"
)

// (Legacy) Regressions Status constants
const (
	// None means there is no regression. This seems to be unused outside initialization,
	// and perhaps should be removed.
	// TODO(b/481616822) remove
	None Status = ""

	// Positive means this change in performance is OK/expected. Used only by Regressions store.
	// Unused by Regressions2 - we use is_improvement field instead.
	Positive Status = "positive"

	// Negative means this regression is a bug. Used only by Regressions store.
	// Unused by Regressions2 - we use is_improvement field instead.
	Negative Status = "negative"
)

// Cluster types
const (
	// Available cluster types in regression2
	HighClusterType ClusterType = "high"
	LowClusterType  ClusterType = "low"
	NoneClusterType ClusterType = "none"
)

// Triage messages
const (
	// IgnoredMessage is the message used when a regression is ignored via the triage menu.
	IgnoredMessage = "Ignored via Triage Menu"
	// NudgedMessage is the message used when a regression is nudged.
	NudgedMessage = "Nudged"
	// ResetMessage is the message used when a regression triage status is reset.
	ResetMessage = ""
	// ABInsignificantMessage is the message used when a regression was found
	// insignificant by autobisection.
	ABInsignificantMessage = "Autobisection - insignificant"
	// ABNoCulpritFoundMessage is the message used when autobisection found no culprit against this
	// regression.
	ABNoCulpritFoundMessage = "Autobisection - no culprit found"
)

// AllStatus is a slice of all values of type Status.
var AllStatus = []Status{None, Positive, Negative, Untriaged, Ignored, Pending, Triaged}

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
	Id                  string                `json:"id"`
	CommitNumber        types.CommitNumber    `json:"commit_number"`
	PrevCommitNumber    types.CommitNumber    `json:"prev_commit_number"`
	DisplayCommitNumber types.CommitNumber    `json:"display_commit_number"`
	AlertId             int64                 `json:"alert_id"`
	Bugs                []types.RegressionBug `json:"bugs"`
	AllBugsFetched      bool                  `json:"all_bugs_fetched"`
	CreationTime        time.Time             `json:"creation_time"`
	MedianBefore        float32               `json:"median_before"`
	MedianAfter         float32               `json:"median_after"`
	IsImprovement       bool                  `json:"is_improvement"`
	ClusterType         string                `json:"cluster_type"`
	LegacyKey           string                `json:"legacy_key"`
	SubscriptionName    string                `json:"sub_name"`
	TraceID             string                `json:"trace_id"`
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

// DetermineIsImprovement returns true if the regression is an improvement.
// It uses the improvement_direction in the paramset and the step fit status.
// If improvement_direction is not present (or Frame is missing), it falls back to fallbackDirection.
func (r *Regression) DetermineIsImprovement(fallbackDirection string) bool {
	_, clusterSummary, _ := r.GetClusterTypeAndSummaryAndTriageStatus()
	if clusterSummary == nil || clusterSummary.StepFit == nil {
		return false
	}
	var paramset map[string][]string
	if r.Frame != nil && r.Frame.DataFrame != nil {
		paramset = r.Frame.DataFrame.ParamSet
	}
	return IsRegressionImprovement(paramset, clusterSummary.StepFit.Status, fallbackDirection)
}

// IsRegressionImprovement returns true if the metric has moved towards the improvement direction.
// If paramset is nil or improvement_direction is not present, it falls back to fallbackDirection.
func IsRegressionImprovement(paramset map[string][]string, stepFitStatus stepfit.StepFitStatus, fallbackDirection string) bool {
	if paramset != nil {
		if _, ok := paramset["improvement_direction"]; ok {
			improvementDirection := paramset["improvement_direction"]
			if len(improvementDirection) > 0 {
				return improvementDirection[0] == "down" && stepFitStatus == stepfit.LOW || improvementDirection[0] == "up" && stepFitStatus == stepfit.HIGH
			}
		}
	}

	if fallbackDirection != "" {
		if stepFitStatus == stepfit.LOW {
			return fallbackDirection == string(alerts.DOWN)
		}
		if stepFitStatus == stepfit.HIGH {
			return fallbackDirection == string(alerts.UP)
		}
	}

	return false
}
