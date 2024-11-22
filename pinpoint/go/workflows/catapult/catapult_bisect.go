package catapult

import (
	"fmt"

	"github.com/google/uuid"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/internal"

	"go.temporal.io/sdk/workflow"

	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

const (
	BisectJobNameTemplate            = "[Skia] Performance bisect on %s/%s"
	ProdPerfInternalWorkflowTemplate = "https://temporal-ui.skia.org/namespaces/perf-internal/workflows/%s"
	DevPerfInternalWorkflowTemplate  = "https://temporal-ui-dev.corp.goog/namespaces/perf-internal/workflows/%s"
)

// updateStatesWithComparisons goes through each state and appends legacy comparisons
func updateStatesWithComparisons(states []*pinpoint_proto.LegacyJobResponse_State, magnitude float64, direction compare.ImprovementDir) error {
	if len(states) < 2 {
		return skerr.Fmt("cannot create comparisons when there are less than 2 objects")
	}

	// handling idx 0.
	perfResult, err := compare.ComparePerformance(states[0].Values, states[1].Values, magnitude, direction)
	if err != nil {
		return skerr.Wrap(err)
	}
	states[0].Comparisons = &pinpoint_proto.LegacyJobResponse_State_Comparison{
		Next: string(perfResult.Verdict),
	}

	// everything else inbetween should have prev and next set.
	for idx := 1; idx < len(states)-1; idx++ {
		currState := states[idx]
		perfResult, err := compare.ComparePerformance(currState.Values, states[idx+1].Values, magnitude, direction)
		if err != nil {
			return skerr.Wrap(err)
		}
		currState.Comparisons = &pinpoint_proto.LegacyJobResponse_State_Comparison{
			Prev: states[idx-1].Comparisons.Next,
			Next: string(perfResult.Verdict),
		}
	}

	// on the last idx, only prev is set.
	states[len(states)-1].Comparisons = &pinpoint_proto.LegacyJobResponse_State_Comparison{
		Prev: states[len(states)-2].Comparisons.Next,
	}
	return nil
}

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
	user := p.Request.GetUser()
	if user == "" {
		// default account for autobisects
		user = "chromeperf@appspot.gserviceaccount.com"
	}
	resp := &pinpoint_proto.LegacyJobResponse{
		JobId:                be.JobId,
		Configuration:        p.Request.GetConfiguration(),
		ImprovementDirection: parseImprovementDir(p.GetImprovementDirection()),
		BugId:                p.Request.GetBugId(),
		Project:              p.Request.GetProject(),
		ComparisonMode:       p.Request.GetComparisonMode(),
		Name:                 fmt.Sprintf(BisectJobNameTemplate, p.Request.GetConfiguration(), p.Request.GetBenchmark()),
		User:                 user,
		Created:              be.CreateTime,
		DifferenceCount:      int32(len(be.Culprits)),
		Metric:               p.Request.GetChart(),
		Quests:               []string{"Build", "Test", "Get values"},
	}

	if p.Production {
		resp.SkiaWorkflowUrl = fmt.Sprintf(ProdPerfInternalWorkflowTemplate, be.JobId)
	} else {
		resp.SkiaWorkflowUrl = fmt.Sprintf(DevPerfInternalWorkflowTemplate, be.JobId)
	}

	arguments, err := parseArguments(p.Request)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	resp.Arguments = arguments

	state, bots, err := parseRawDataToLegacyObject(ctx, be.Comparisons, be.RunData, p.Request.GetChart())
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	resp.State = state
	resp.Bots = bots

	// Comparisons (prev, next) are calculated by comparing each one in order of events
	// so if we have commits (A, B, C), A is compared with B to determine next. B is compared with C
	// and the comparison values at B would be (prev: {next value from A}, next: {current calculated value}).
	// C would only have prev set, which would be equivalent to B's next value.
	err = updateStatesWithComparisons(state, p.GetMagnitude(), p.GetImprovementDirection())
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	resp.Updated = timestamppb.Now()

	return resp, nil
}

// CatapultBisectWorkflow is a Skia-based bisect workflow that's backwards compatible to Catapult.
//
// By backwards compatible, it means that this workflow utilizes the Skia-based bisect, but writes
// the responses to Catapult via Skia-Bridge, such that the Catapult UI can display the results.
// Thus, the workflow method signature should be identical to internal.BisectWorkflow.
// This is written in its own package and in its own workflow so that it's self-contained.
func CatapultBisectWorkflow(ctx workflow.Context, p *workflows.BisectParams) (*pinpoint_proto.BisectExecution, error) {
	logger := workflow.GetLogger(ctx)

	// We want to specify the exact job id it'll be using instead of a randomly generated one
	// so that users from Pinpoint can route back to it.
	workflowID := p.JobID
	if workflowID != "" {
		// Reuse the JobID as the workflowID if given.
	} else if err := workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} {
		return uuid.New().String()
	}).Get(&workflowID); err != nil {
		return nil, skerr.Wrap(err)
	}

	bisectOptions := childWorkflowOptions
	bisectOptions.WorkflowID = workflowID
	bisectCtx := workflow.WithChildOptions(ctx, bisectOptions)
	bisectCtx = workflow.WithActivityOptions(bisectCtx, regularActivityOptions)

	// The workflow options above will ensure that it runs with that UUID. Setting it in
	// BisectParams will ensure that the respoonse JobID is also set to this such that
	// when it's converted to the Legacy response it's propagated accordingly.
	p.JobID = workflowID
	var bisectExecution *internal.BisectExecution
	if err := workflow.ExecuteChildWorkflow(bisectCtx, internal.BisectWorkflow, p).Get(bisectCtx, &bisectExecution); err != nil {
		return nil, skerr.Wrap(err)
	}

	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, catapultBisectActivityOptions)
	var resp *pinpoint_proto.LegacyJobResponse
	if err := workflow.ExecuteChildWorkflow(ctx, ConvertToCatapultResponseWorkflow, p, bisectExecution).Get(ctx, &resp); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Note, if running locally, you may need to disable this activity.
	// See WriteBisectToCatapultActivity in README.md.
	var dsResp *DatastoreResponse
	if err := workflow.ExecuteActivity(ctx, WriteBisectToCatapultActivity, &resp, p.Production).Get(ctx, &dsResp); err != nil {
		return nil, skerr.Wrap(err)
	}

	logger.Info(fmt.Sprintf("Datastore information for this job: %v", dsResp))

	return &pinpoint_proto.BisectExecution{
		JobId:            bisectExecution.JobId,
		Culprits:         bisectExecution.Culprits,
		DetailedCulprits: bisectExecution.DetailedCulprits,
	}, nil
}
