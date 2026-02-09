package schema

// AlertSchema represents the SQL schema of the Alerts table.
type AlertSchema struct {
	ID int `sql:"id INT PRIMARY KEY DEFAULT unique_rowid()"`

	// An alerts.Alert serialized as JSON.
	// TODO(jcgregorio) Update this to JSONB.
	Alert string `sql:"alert TEXT"`

	// The Alert.State which is an alerts.ConfigState value, which is converted
	// into an int.
	//
	// TODO(jcgregorio) Rationalize to either be an int or a string in both SQL
	// and in the code.
	ConfigState int `sql:"config_state INT DEFAULT 0"`

	// Stored as a Unit timestamp.
	LastModified int `sql:"last_modified INT"`

	// Name of the subscription this alert responds to.
	SubscriptionName string `sql:"sub_name STRING"`

	// Revision of the associated subscription. Used to query the Subscriptions table.
	SubscriptionRevision string `sql:"sub_revision STRING"`

	// Index used to query alerts by subscription name
	bySubNameIndex struct{} `sql:"INDEX idx_alerts_subname (sub_name)"`
}
