package db

import (
	"context"
	"time"

	"go.skia.org/infra/autogardener/go/types"
)

type AutoGardenerDB interface {
	// GetTaskSummary retrieves the Summary for the given task ID, if it exists.
	// If not, it returns nil with no error.
	GetTaskSummary(ctx context.Context, taskID string) (*types.TaskSummary, error)

	// PutTaskSummary sets the Summary for the given task ID, replacing any
	// existing entry.
	PutTaskSummary(ctx context.Context, taskID string, summary *types.TaskSummary) error

	// GetUnclassifiedTaskSummaries retrieves any TaskSummary which has not been
	// classified, keyed by task ID.
	GetUnclassifiedTaskSummaries(ctx context.Context, limit int) (map[string]*types.TaskSummary, error)

	// GetReport retrieves the latest Report for the given repo and branch, if
	// it exists. If not, it returns nil with no error.
	GetReport(ctx context.Context, repo, branch string) (*types.Report, error)

	// PutReport sets the latest Report for the given repo and branch, replacing
	// any existing entry.
	PutReport(ctx context.Context, repo, branch string, report *types.Report) error

	// GetFailureClass retrieves the FailureClass for the given ID, if it exists.
	// If not, it returns nil with no error.
	GetFailureClass(ctx context.Context, id string) (*types.FailureClass, error)

	// PutFailureClass sets the FailureClass for the given ID, replacing any
	// existing entry.
	PutFailureClass(ctx context.Context, fc *types.FailureClass) error

	// GetRecentFailureClasses retrieves FailureClasses seen after the given
	// timestamp, up to the specified limit.
	GetRecentFailureClasses(ctx context.Context, repo string, since time.Time, limit int) ([]*types.FailureClass, error)
}
