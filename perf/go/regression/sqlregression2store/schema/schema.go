package schema

import (
	"time"

	"go.skia.org/infra/perf/go/types"
)

// RegressionSchema is the SQL schema for storing regression.Regression's.
type Regression2Schema struct {
	// The id for the regression.
	// Changed from UUID to Text in https://b.corp.google.com/issues/492077374
	ID string `sql:"id TEXT PRIMARY KEY DEFAULT spanner.generate_uuid()"`

	// The commit_number where the regression occurred.
	CommitNumber types.CommitNumber `sql:"commit_number INT"`

	// The commit_number before the commit where the regression occurred.
	PrevCommitNumber types.CommitNumber `sql:"prev_commit_number INT"`

	// The commit_number to be displayed for the regression in the graph.
	// This point will always be within the range (PrevCommitNumber, CommitNumber].
	// In case of using anomaly localization, it will be the point with the biggest
	// regression in the range. If not, it will be equal to CommitNumber.
	// The nudging process will allow to move this point, but only within the
	// (PrevCommitNumber, CommitNumber] range.
	DisplayCommitNumber types.CommitNumber `sql:"display_commit_number INT"`

	// The id of an Alert, i.e. the id from the Alerts table.
	AlertID int `sql:"alert_id INT"`

	// Subscription name associated with the regression.
	SubName string `sql:"sub_name TEXT"`

	// Id of the bug created by manual triage.
	BugID int `sql:"bug_id INT"`

	// The timestamp when the anomaly group is created.
	CreationTime time.Time `sql:"creation_time TIMESTAMPTZ DEFAULT now()"`

	// Median of the data frame before the regression.
	MedianBefore float32 `sql:"median_before REAL"`

	// Median of the data frame after the regression.
	MedianAfter float32 `sql:"median_after REAL"`

	// Whether the regression represents an improvement in the metrics.
	IsImprovement bool `sql:"is_improvement BOOL"`

	// The cluster type for the regression.
	ClusterType string `sql:"cluster_type TEXT"`

	// A clustering2.ClusterSummary serialized as json.
	ClusterSummary interface{} `sql:"cluster_summary JSONB"`

	// A frame.FrameResponse serialized as json.
	Frame interface{} `sql:"frame JSONB"`

	// Chromeperf ID in case this anomaly is a migrated one. Otherwise 0.
	LegacyKey string `sql:"legacy_key TEXT"`

	// Id of the trace this regression has been detected on.
	// Equal to ID of frame->'dataframe'->'traceset' (for new anomalies with individual detection).
	TraceID []byte `sql:"trace_id BYTES"`

	// Triage status for the regression.
	TriageStatus string `sql:"triage_status TEXT"`

	// Triage message for the regression.
	TriageMessage string `sql:"triage_message TEXT"`

	// Index used to query regressions based on alert id
	byAlertIdIndex struct{} `sql:"INDEX by_alert_id (alert_id)"`

	// Index used to query regressions based on subscription name and creation time.
	bySubNameAndCreationTime struct{} `sql:"INDEX by_sub_name_creation_time (sub_name, creation_time DESC)"`

	// Index used to query untriaged regressions
	bySubNameTriageStatusCreationTimeAsc struct{} `sql:"INDEX by_sub_name_triage_status_creation_time_asc (sub_name, triage_status, creation_time ASC)"`

	// Index used to query regressions by commit and alert ids
	byCommitAndAlertIndex struct{} `sql:"INDEX by_commit_alert (commit_number, alert_id)"`

	// Index used to query regressions by revision number. Tailored for GetByRevision query.
	byCommitAndPrevCommitIndex struct{} `sql:"INDEX by_commit_and_prev_commit (commit_number, prev_commit_number)"`

	// Tailored for ReadRangeFiltered - by TraceId.
	byTraceIdAndCommit struct{} `sql:"INDEX by_trace_id_and_commit (trace_id, commit_number)"`

	// Matching against legacy keys (in particular, to speedup legacy SID)
	byLegacyKey struct{} `sql:"INDEX by_legacy_key (legacy_key)"`
}
