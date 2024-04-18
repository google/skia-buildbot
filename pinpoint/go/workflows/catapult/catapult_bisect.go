package catapult

import (
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/internal"

	"go.temporal.io/sdk/workflow"

	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
)

// CatapultBisectWorkflow is a Skia-based bisect workflow that's backwards compatible to Catapult.
//
// By backwards compatible, it means that this workflow utilizes the Skia-based bisect, but writes
// the responses to Catapult via Skia-Bridge, such that the Catapult UI can display the results.
// Thus, the workflow method signature should be identical to internal.BisectWorkflow.
// This is written in its own package and in its own workflow so that it's self-contained.
func CatapultBisectWorkflow(ctx workflow.Context, p *workflows.BisectParams) (be *pinpoint_proto.BisectExecution, wkErr error) {
	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)

	var bisectExecution *pinpoint_proto.BisectExecution
	if err := workflow.ExecuteChildWorkflow(ctx, internal.BisectWorkflow, p).Get(ctx, &bisectExecution); err != nil {
		return nil, skerr.Wrap(err)
	}

	return bisectExecution, nil
}
