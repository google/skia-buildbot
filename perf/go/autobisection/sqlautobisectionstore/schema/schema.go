package schema

// AutobisectionSchema represents the SQL schema of the Autobisections table.
type AutobisectionSchema struct {
	// The Pinpoint job ID.
	JobID string `sql:"job_id TEXT PRIMARY KEY"`

	// Link to the Anomaly Group / Regression.
	AnomalyGroupID string `sql:"anomaly_group_id TEXT"`

	// ID of the anomaly bisect operates on.
	AnomalyId string `sql:"anomaly_id TEXT"`

	// Whether a culprit or regression was verified.
	IsRealRegression bool `sql:"is_real_regression BOOL"`
}
