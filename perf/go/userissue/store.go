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
	LastModified   int64
}

type SaveRequest struct {
	UserId         string
	TraceKey       string
	CommitPosition int64
	IssueId        int64
}

// Store is the interface used to persist user issues.
type Store interface {
	// Save inserts/updates a user issue into the db
	Save(ctx context.Context, req *SaveRequest) error

	// Delete deletes the user issue from the db.
	Delete(ctx context.Context, traceKey string, commitPosition int64) error

	// GetUserIssuesForTraceIds retrieves list of points with an associated issue id.
	GetUserIssuesForTraceIds(ctx context.Context, traceIds []string) ([]UserIssue, error)
}
