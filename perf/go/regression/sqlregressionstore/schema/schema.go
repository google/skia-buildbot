package schema

import "go.skia.org/infra/perf/go/types"

// RegressionSchema is the SQL schema for storing regression.Regression's.
type RegressionSchema struct {
	// The commit_number where the regression occurred.
	CommitNumber types.CommitNumber `sql:"commit_number INT"`

	// The id of an Alert, i.e. the id from the Alerts table.
	AlertID int `sql:"alert_id INT"`

	// A regression.Regression serialized as JSON.
	Regression string `sql:"regression TEXT"`

	compoundKey struct{} `sql:"PRIMARY KEY (commit_number, alert_id)"`
}
