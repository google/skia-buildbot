package regression

import (
	"context"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/progress"
	pb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

// AnomalyCommitRange encapsulates the commit numbers associated with an anomaly.
type AnomalyCommitRange struct {
	CommitNumber        types.CommitNumber
	PrevCommitNumber    types.CommitNumber
	DisplayCommitNumber types.CommitNumber
}

// Store persists Regressions.
type Store interface {
	// Range returns a map from types.CommitNumber to *Regressions that exist in the
	// given range of commits. Note that if begin==end that results
	// will be returned for begin.
	Range(ctx context.Context, begin, end types.CommitNumber) (map[types.CommitNumber]*AllRegressionsForCommit, error)

	// RangeFiltered gets all regressions in the given commit range and trace names.
	RangeFiltered(ctx context.Context, begin, end types.CommitNumber, traceNames []string) ([]*Regression, error)

	// SetHigh sets the ClusterSummary for a high regression at the given commit and alertID.
	SetHigh(ctx context.Context, commitRange AnomalyCommitRange, alertID string, df *frame.FrameResponse, high *clustering2.ClusterSummary) (bool, string, error)

	// SetLow sets the ClusterSummary for a low regression at the given commit and alertID.
	SetLow(ctx context.Context, commitRange AnomalyCommitRange, alertID string, df *frame.FrameResponse, low *clustering2.ClusterSummary) (bool, string, error)

	// TriageLow sets the triage status for the low cluster at the given commit and alertID.
	TriageLow(ctx context.Context, commitNumber types.CommitNumber, alertID string, tr TriageStatus) error

	// TriageHigh sets the triage status for the high cluster at the given commit and alertID.
	TriageHigh(ctx context.Context, commitNumber types.CommitNumber, alertID string, tr TriageStatus) error

	// Write the Regressions to the store. The provided 'regressions' maps from
	// types.CommitNumber to all the regressions for that commit.
	Write(ctx context.Context, regressions map[types.CommitNumber]*AllRegressionsForCommit) error

	// Given the subscription name GetRegressionsBySubName gets all the regressions against
	// the specified subscription. The response will be paginated according to the provided
	// limit and offset.
	GetRegressionsBySubName(ctx context.Context, req GetAnomalyListRequest, limit int) ([]*Regression, error)

	// Given a list of regression IDs (only in the regression2store),
	// return a list of regressions.
	GetByIDs(ctx context.Context, ids []string) ([]*Regression, error)

	// GetIdsByManualTriageBugID returns a list of distinct regression ids with given manual triage bug id.
	GetIdsByManualTriageBugID(ctx context.Context, bugID int) ([]string, error)

	// Return a list of regressions satisfying: previous_commit < rev <= commit.
	GetByRevision(ctx context.Context, rev string) ([]*Regression, error)

	// GetOldestCommit returns the commit with the lowest commit number
	GetOldestCommit(ctx context.Context) (*types.CommitNumber, error)

	// GetRegression returns the regression info at the given commit for specific alert.
	GetRegression(ctx context.Context, commitNumber types.CommitNumber, alertID string) (*Regression, error)

	// GetRegressionsBefore returns up to limit regressions for the given trace before or at the commit.
	GetRegressionsBefore(ctx context.Context, traceName string, subName string, commit types.CommitNumber, limit int) ([]*Regression, error)

	// DeleteByCommit deletes a regression from the Regression table via the CommitNumber.
	// Use with caution.
	DeleteByCommit(ctx context.Context, commitNumber types.CommitNumber, tx pgx.Tx) error

	// SetBugID associates a set of regressions, identified by their IDs, with a bug ID.
	SetBugID(ctx context.Context, regressionIDs []string, bugID int) error

	// IgnoreAnomalies sets the triage status to Ignored and message to IgnoredMessage for the given regressions.
	IgnoreAnomalies(ctx context.Context, regressionIDs []string) error

	// ResetAnomalies sets the triage status to Untriaged, message to ResetMessage, and bugID to 0 for the given regressions.
	ResetAnomalies(ctx context.Context, regressionIDs []string) error

	// NudgeAndResetAnomalies updates the commit number and previous commit number for the given regressions,
	// and also sets the triage status to Untriaged, message to NudgedMessage, and bugID to 0.
	NudgeAndResetAnomalies(ctx context.Context, regressionIDs []string, displayCommitNumber types.CommitNumber) error

	// GetBugIdsForRegressions queries all bugs from regressions2, culprits and anomalygroups for given regressions.
	GetBugIdsForRegressions(ctx context.Context, regressions []*Regression) ([]*Regression, error)

	// GetSubscriptionsForRegressions returns a subset of subscription fields for given regressions, together with regression and alert ids.
	GetSubscriptionsForRegressions(ctx context.Context, regressionIDs []string) ([]string, []int64, []*pb.Subscription, error)
}

// FullSummary describes a single regression.
type FullSummary struct {
	Summary clustering2.ClusterSummary `json:"summary"`
	Triage  TriageStatus               `json:"triage"`
	Frame   frame.FrameResponse        `json:"frame"`
}

// Request object for the request from the anomaly table UI.
type GetAnomalyListRequest struct {
	SubName             string `json:"sheriff"`
	IncludeTriaged      bool   `json:"triaged"`
	IncludeImprovements bool   `json:"improvements"`
	QueryCursor         string `json:"anomaly_cursor"`
	Host                string `json:"host"`
	PaginationOffset    int    `json:"pagination_offset,omitempty"`
}

// BackfillRequest is the request for the backfill endpoint or topic.
type BackfillRequest struct {
	// Unique ID for tracking the request in logs.
	RequestID string `json:"request_id"`

	// ID of the alert configuration to use.
	AlertID int64 `json:"alert_id"`

	// The end timestamp (Unix seconds). We load the latest N commits (e.g., 50)
	// ending at or before this timestamp and run anomaly detection. This acts
	// as the "processing date". We don't need a start timestamp because commit
	// density varies and we just load the last N commits relative to this end date.
	End int64 `json:"end"`

	// Whether to send notifications if regressions are found.
	SendNotifications bool `json:"send_notifications"`

	// Whether to load all traces in a single data frame (true) or in chunks (false).
	LoadAllTracesTogether bool `json:"load_all_traces_together"`

	// Specific trace IDs to process. If provided, only these traces will be processed.
	TraceIDs []string `json:"trace_ids,omitempty"`
}

// RegressionDetectionRequest is all the info needed to start a clustering run,
// an Alert and the Domain over which to run that Alert.
type RegressionDetectionRequest struct {
	Alert  *alerts.Alert `json:"alert"`
	Domain types.Domain  `json:"domain"`

	// query is the exact query being run. It may be more specific than the one
	// in the Alert if the Alert has a non-empty GroupBy.
	query string

	// Step/TotalQueries is the current percent of all the queries that have been processed.
	Step int `json:"step"`

	// TotalQueries is the number of sub-queries to be processed based on the
	// GroupBy setting in the Alert.
	TotalQueries int `json:"total_queries"`

	// Progress of the detection request.
	Progress progress.Progress `json:"-"`
}

// Query returns the query that the RegressionDetectionRequest process is
// running.
//
// Note that it may be more specific than the Alert.Query if the Alert has a
// non-empty GroupBy value.
func (r *RegressionDetectionRequest) Query() string {
	if r.query != "" {
		return r.query
	}
	if r.Alert != nil {
		return r.Alert.Query
	}
	return ""
}

// SetQuery sets a more refined query for the RegressionDetectionRequest.
func (r *RegressionDetectionRequest) SetQuery(q string) {
	r.query = q
}

// NewRegressionDetectionRequest returns a new RegressionDetectionRequest.
func NewRegressionDetectionRequest() *RegressionDetectionRequest {
	return &RegressionDetectionRequest{
		Progress: progress.New(),
	}
}

type RegressionDetectionResponse struct {
	Summary *clustering2.ClusterSummaries `json:"summary"`
	Frame   *frame.FrameResponse          `json:"frame"`

	// Message contains context about the detection for this specific response,
	// such as trace filtering statistics.
	Message string `json:"-"` // Using json:"-" prevents it from being serialized by default.

	TraceName string `json:"-"`
}

// ConfirmedRegression represents a regression that has been validated and approved
// for saving or alerting. It includes the explicit commit boundary where it was found.
type ConfirmedRegression struct {
	Summary             *clustering2.ClusterSummaries `json:"summary"`
	RightMostSummary    *clustering2.ClusterSummaries `json:"-"`
	Frame               *frame.FrameResponse          `json:"frame"`
	RightMostFrame      *frame.FrameResponse          `json:"-"`
	Message             string                        `json:"-"`
	PrevCommitNumber    types.CommitNumber            `json:"prev_commit_number"`
	CommitNumber        types.CommitNumber            `json:"commit_number"`
	DisplayCommitNumber types.CommitNumber            `json:"display_commit_number"`
}

// RegressionRefiner defines an interface for modules that process a complete
// set of regression detection results before they are sent for storage.
type RegressionRefiner interface {
	// Process takes a slice of RegressionDetectionResponse (the raw results of
	// regression detection). It returns a processed slice of ConfirmedRegression,
	// which contains the anomalies (e.g. high or low status when exceeding a threshold)
	// that we want to save into the database, send notifications and etc.
	// ConfirmedRegression is an alias for RegressionDetectionResponse used by the RegressionRefiner
	// and ConfirmedRegressionHandler to represent regressions that have been validated and approved
	// for saving or alerting.
	Process(ctx context.Context, cfg *alerts.Alert, responses []*RegressionDetectionResponse) ([]*ConfirmedRegression, error)
}

type dryRunKey struct{}

// WithDryRun returns a new context with the dry run flag set to true.
func WithDryRun(ctx context.Context) context.Context {
	return context.WithValue(ctx, dryRunKey{}, true)
}

// IsDryRun returns true if the dry run flag is set to true in the context.
func IsDryRun(ctx context.Context) bool {
	v, _ := ctx.Value(dryRunKey{}).(bool)
	return v
}
