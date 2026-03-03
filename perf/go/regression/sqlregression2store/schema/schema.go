package schema

import (
	"time"

	"go.skia.org/infra/perf/go/types"
)

// RegressionSchema is the SQL schema for storing regression.Regression's.
type Regression2Schema struct {
	// The id for the regression.
	ID string `sql:"id UUID PRIMARY KEY DEFAULT gen_random_uuid()"`

	// The commit_number where the regression occurred.
	CommitNumber types.CommitNumber `sql:"commit_number INT"`

	// The commit_number before the commit where the regression occurred.
	PrevCommitNumber types.CommitNumber `sql:"prev_commit_number INT"`

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

	// Triage status for the regression.
	TriageStatus string `sql:"triage_status TEXT"`

	// Triage message for the regression.
	TriageMessage string `sql:"triage_message TEXT"`

	// Index used to query regressions based on alert id
	byAlertIdIndex struct{} `sql:"INDEX by_alert_id (alert_id)"`

	// Index used to query regressions based on subscription name and creation time.
	bySubNameAndCreationTime struct{} `sql:"INDEX by_sub_name_creation_time (sub_name, creation_time DESC)"`

	// Index used to query regressions by commit and alert ids
	byCommitAndAlertIndex struct{} `sql:"INDEX by_commit_alert (commit_number, alert_id)"`

	// Index used to query regressions by revision number. Tailored for GetByRevision query.
	byCommitAndPrevCommitIndex struct{} `sql:"INDEX by_commit_and_prev_commit (commit_number, prev_commit_number)"`
}
