package userissue

import (
	"context"
)

// UserIssue is a struct that represents a user reported buganizer
// issue on a data point.
type UserIssue struct {
	UserId         string
	TraceKey       string
	CommitPosition int64
	IssueId        int64
}

// Store is the interface used to persist user issues.
type Store interface {
	// Save inserts a user issue into the db
	Save(ctx context.Context, req *UserIssue) error

	// Delete deletes the user issue from the db.
	Delete(ctx context.Context, traceKey string, commitPosition int64) error

	// GetUserIssuesForTraceKeys retrieves list of points with an associated issue id.
	GetUserIssuesForTraceKeys(ctx context.Context, traceKeys []string, startCommitPosition int64, endCommitPosition int64) ([]UserIssue, error)
}
