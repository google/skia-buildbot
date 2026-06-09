package schema

// AutobisectionSchema represents the SQL schema of the Autobisections table.
type AutobisectionSchema struct {
	// The Pinpoint job ID.
	JobID string `sql:"job_id TEXT PRIMARY KEY"`

	// Id of the workflow that executed this autobisection.
	WorkflowID string `sql:"workflow_id TEXT"`

	// Link to the Anomaly Group / Regression.
	AnomalyGroupID string `sql:"anomaly_group_id TEXT"`

	// ID of the anomaly bisect operates on.
	AnomalyId string `sql:"anomaly_id TEXT"`

	// insignificant / no culprit / found culprits
	RegressionStatus string `sql:"regression_status TEXT"`
}
