package catapult

import (
	"fmt"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/internal"

	"go.temporal.io/sdk/workflow"

	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
)

const (
	BisectJobNameTemplate = "[Skia] Performance bisect on %s/%s"
)

// ConvertToCatapultResponseWorkflow converts raw data from a Skia bisection into a Catapult-supported format.
func ConvertToCatapultResponseWorkflow(ctx workflow.Context, p *workflows.BisectParams, be *internal.BisectExecution) (*pinpoint_proto.LegacyJobResponse, error) {
	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)

	// Updated is not set because it has auto_now_add=True
	// Non required fields that are not set:
	//   - StartedTime
	//   - Status: This property is derived from one of (failed, cancelled, completed or running), which
	//      are all computed properties.
	//   - Exception: TODO() Update this to also add exception details if we want to propagate jobs with
	//      issues back to the UI
	//   - CancelReason
	//   - BatchID: Unsupported field from Skia Bisect
	resp := &pinpoint_proto.LegacyJobResponse{
		JobId:                be.JobId,
		Configuration:        p.Request.GetConfiguration(),
		ImprovementDirection: parseImprovementDir(p.GetImprovementDirection()),
		BugId:                p.Request.GetBugId(),
		Project:              p.Request.GetProject(),
		ComparisonMode:       p.Request.GetComparisonMode(),
		Name:                 fmt.Sprintf(BisectJobNameTemplate, p.Request.GetConfiguration(), p.Request.GetBenchmark()),
		User:                 p.Request.GetUser(),
		Created:              be.CreateTime,
		DifferenceCount:      int32(len(be.Culprits)),
		Metric:               p.Request.GetChart(),
		Quests:               []string{"Build", "Test", "Get values"},
	}

	arguments, err := parseArguments(p.Request)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	resp.Arguments = arguments

	state, bots, err := parseRawDataToLegacyObject(ctx, be.Comparisons, be.RunData)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	resp.State = state
	resp.Bots = bots

	return resp, nil
}

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

	var resp *pinpoint_proto.LegacyJobResponse
	if err := workflow.ExecuteChildWorkflow(ctx, ConvertToCatapultResponseWorkflow, p, bisectExecution).Get(ctx, &resp); err != nil {
		return nil, skerr.Wrap(err)
	}

	// TODO(jeffyoon@) - integrate with skia bridge call

	return bisectExecution, nil
}
