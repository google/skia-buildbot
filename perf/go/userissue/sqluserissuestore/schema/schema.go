package schema

import "time"

// UserIssueSchema represents the SQL schema of the UserIssues table.
type UserIssueSchema struct {
	// The user who associated the data point with the buganizer issue.
	// The user id will be their email as returned by uber-proxy auth.
	UserId string `sql:"user_id TEXT NOT NULL"`
	// The trace on which the bug_id was associated
	TraceKey string `sql:"trace_key TEXT NOT NULL"`
	// Commit position on which the bug_id was associated
	CommitPosition int `sql:"commit_position INT NOT NULL"`
	// The buganzier issue number
	IssueId int `sql:"issue_id INT NOT NULL"`
	// Stored as a Unix timestamp. Timestamp when this
	// database record was updated.
	LastModified time.Time `sql:"last_modified TIMESTAMPTZ DEFAULT now()"`
	// trace_key and commit_position are used to key a user issue.
	PrimaryKey struct{} `sql:"PRIMARY KEY(trace_key, commit_position)"`
}
