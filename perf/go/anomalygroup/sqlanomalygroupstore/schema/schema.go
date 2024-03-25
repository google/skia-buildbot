package schema

import "time"

// AnomalyGroupSchemaSchema represents the SQL schema of the AnomalyGroups table.
type AnomalyGroupSchema struct {
	ID string `sql:"id UUID PRIMARY KEY DEFAULT gen_random_uuid()"`

	// The timestamp when the anomaly group is created.
	CreationTime time.Time `sql:"creation_time TIMESTAMPTZ DEFAULT now()"`

	// The LIST of metadata for each anomaly
	AnomalyIDs []string `sql:"anomaly_ids UUID ARRAY"`

	// The meta data from the first grouped anomaly.
	// Currently we should expect the followings:
	//   subscription_name;
	//   master_name;
	//   benchmark_name.
	GroupMetaData interface{} `sql:"group_meta_data JSONB"`

	// The overlapped reivision range among all the anomalies.
	// These two properties may change when new anomalies are added.
	// Keeping the up-to-date values here to avoid querying anomaly table.
	CommonRevStart int `sql:"common_rev_start INT"`
	CommonRevEnd   int `sql:"common_rev_end INT"`

	// An alerts.Alert.Action item which can be 'noaction', 'report'
	// or 'bisect'.
	Action string `sql:"action TEXT"`

	// The timestamp when the action takes place.
	ActionTime time.Time `sql:"action_time TIMESTAMPTZ"`

	// The ID of the bisection job if the action is 'bisect'
	BisectionID string `sql:"bisection_id TEXT"`
	// The ID of the filed issue if the action is 'report'
	// Notice that this is different from the issues filed at the
	// end of bisection.
	ReportedIssueID string `sql:"reported_issue_id TEXT"`

	// The list of culprits found related to this group
	CulpritIDs []string `sql:"culprit_ids UUID ARRAY"`

	// The timestamp of the last update
	LastModifiedTime time.Time `sql:"last_modified_time TIMESTAMPTZ"`
}
