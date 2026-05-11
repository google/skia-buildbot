package db

import (
	"context"

	"go.skia.org/infra/autogardener/go/types"
)

type AutoGardenerDB interface {
	// GetTaskSummary retrieves the Summary for the given task ID, if it exists.
	// If not, it returns nil with no error.
	GetTaskSummary(ctx context.Context, taskID string) (*types.TaskSummary, error)

	// PutTaskSummary sets the Summary for the given task ID, replacing any
	// existing entry.
	PutTaskSummary(ctx context.Context, taskID string, summary *types.TaskSummary) error
}
