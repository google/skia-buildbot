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

	// Indicates if the regression is migrated to the regression2 table.
	Migrated bool `sql:"migrated BOOL"`

	// Id for the regression. This is only used to migrate data into the new schema.
	RegressionId string `sql:"regression_id TEXT"`

	compoundKey struct{} `sql:"PRIMARY KEY (commit_number, alert_id)"`
}
