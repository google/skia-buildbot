package backends

import (
	"context"

	"go.skia.org/infra/go/sklog"

	"google.golang.org/api/workflowexecutions/v1"
)

// DialCloudWorkflowsExecutionService returns a google cloud workflows 'Executions' service client.
func DialCloudWorkflowsExecutionService(ctx context.Context) (*workflowexecutions.Service, error) {
	workflowexecutionsService, err := workflowexecutions.NewService(ctx)
	if err != nil {
		sklog.Infof("cloud workflow service dial failed: %v", err)

		return nil, err
	}

	return workflowexecutionsService, nil
}
