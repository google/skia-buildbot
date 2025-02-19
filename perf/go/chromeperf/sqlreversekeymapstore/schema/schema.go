package schema

// b/383913153
// The test paths on Chromeperf are updated when they are uploaded to Perf.
// The ‘invalid’ characters in the paths are placed by underscores. It causes
// issues when we try to use the test paths on Perf to query for anomalies in
// Chromeperf. As we don’t know which ‘invalid’ characters are replaced, we
// have no way to do the query correctly.
// This ReverseKeyMap is used to keep track of all the replacements. So that we
// have a way to track back to the original value.
// Considering the numbers of test paths are stable, the size of the total
// records should be stable after all tests are uploaded once.
type ReverseKeyMapSchema struct {
	// The updated param value in Perf after some characters are replaced.
	ModifiedValue string `sql:"modified_value TEXT"`
	// The param key of the value.
	ParamKey string `sql:"param_key TEXT"`
	// The original value of the param value in Chromeperf.
	OriginalValue string `sql:"original_value TEXT"`
	// The primary key is the combination of param key and the updated param value.
	// It should point to a unique original value.
	PrimaryKey struct{} `sql:"PRIMARY KEY(modified_value, param_key)"`
}
