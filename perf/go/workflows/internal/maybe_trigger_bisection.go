package internal

import (
	"go.skia.org/infra/perf/go/workflows"
	"go.temporal.io/sdk/workflow"
)

func MaybeTriggerBisectionWorkflow(ctx workflow.Context, input *workflows.MaybeTriggerBisectionParam) (*workflows.MaybeTriggerBisectionResult, error) {
	return &workflows.MaybeTriggerBisectionResult{}, nil
}
