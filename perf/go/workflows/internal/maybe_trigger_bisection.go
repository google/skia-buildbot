package internal

import (
	"time"

	"github.com/google/uuid"
	"go.skia.org/infra/go/skerr"
	ag_pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	c_pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/workflows"
	pinpoint "go.skia.org/infra/pinpoint/go/workflows"
	pp_pb "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/workflow"
)

const (
	_WAIT_TIME_FOR_ANOMALIES = 30 * time.Minute
)

// MaybeTriggerBisectionWorkflow is the entry point for the workflow which handles anomaly group
// processing. It is responsible for triggering a bisection if the anomalygroup's
// group action = BISECT. If group action = REPORT, files a bug notifying user of the anomalies.
func MaybeTriggerBisectionWorkflow(ctx workflow.Context, input *workflows.MaybeTriggerBisectionParam) (*workflows.MaybeTriggerBisectionResult, error) {
	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
	var anomalyGroupResponse *ag_pb.LoadAnomalyGroupByIDResponse
	var err error
	var agsa AnomalyGroupServiceActivity
	var gsa GerritServiceActivity
	var csa CulpritServiceActivity

	// Step 1. wait for some time so that more anomalies can be detected and grouped.
	if err = workflow.Sleep(ctx, _WAIT_TIME_FOR_ANOMALIES); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Step 2. Load Anomalygroup data
	if err = workflow.ExecuteActivity(ctx, agsa.LoadAnomalyGroupByID, input.AnomalyGroupServiceUrl, &ag_pb.LoadAnomalyGroupByIDRequest{
		AnomalyGroupId: input.AnomalyGroupId,
	}).Get(ctx, &anomalyGroupResponse); err != nil {
		return nil, skerr.Wrap(err)
	}

	if anomalyGroupResponse.AnomalyGroup.GroupAction == ag_pb.GroupActionType_BISECT {
		// Step 3. Load Anomaly data
		var topAnomaliesResponse *ag_pb.FindTopAnomaliesResponse
		if err = workflow.ExecuteActivity(ctx, agsa.FindTopAnomalies, input.AnomalyGroupServiceUrl, &ag_pb.FindTopAnomaliesRequest{
			AnomalyGroupId: input.AnomalyGroupId,
			Limit:          1,
		}).Get(ctx, &topAnomaliesResponse); err != nil {
			return nil, skerr.Wrap(err)
		}
		var topAnomaly *ag_pb.Anomaly
		if len(topAnomaliesResponse.Anomalies) <= 0 {
			return nil, skerr.Fmt("No anomalies found for anomalygroup %s", input.AnomalyGroupId)
		} else {
			topAnomaly = topAnomaliesResponse.Anomalies[0]
		}
		// Step 4. Convert start and end commit postions to commit hash
		var startHash, endHash string
		if err = workflow.ExecuteActivity(ctx, gsa.GetCommitRevision, topAnomaly.StartCommit).Get(ctx, &startHash); err != nil {
			return nil, skerr.Wrap(err)
		}
		if err = workflow.ExecuteActivity(ctx, gsa.GetCommitRevision, topAnomaly.EndCommit).Get(ctx, &endHash); err != nil {
			return nil, skerr.Wrap(err)
		}
		// Step 5. Invoke Bisection conditionally
		child_wf_id := uuid.New().String()
		// Childworkflow options includes:
		//   WorkflowID: 		The UUID to be used as the Pinpoint job id. We pre-assigne it
		//				 		here to avoid extra calls to get it from the spawned workflow.
		//	 TaskQueue:  		Assign the cihld workflow to the correct task queue. If this is
		//				 		empty, it will be assigned to the current grouping queue.
		//   ParentClosePolicy: Using _ABANDON option to ensure the child workflow will
		//    					continue even if the parent workflow exits.
		child_wf_options := workflow.ChildWorkflowOptions{
			WorkflowID:        child_wf_id,
			TaskQueue:         input.PinpointTaskQueue,
			ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON,
		}
		c_ctx := workflow.WithChildOptions(ctx, child_wf_options)

		var aggregationMethod string
		// "value" is not a valid aggregation method in pinpoint.
		if topAnomaly.Paramset["stat"] == "value" {
			aggregationMethod = "mean"
		} else {
			aggregationMethod = topAnomaly.Paramset["stat"]
		}
		find_culprit_wf := workflow.ExecuteChildWorkflow(c_ctx, pinpoint.CulpritFinderWorkflow,
			&pinpoint.CulpritFinderParams{
				Request: &pp_pb.ScheduleCulpritFinderRequest{
					StartGitHash:         startHash,
					EndGitHash:           endHash,
					Configuration:        topAnomaly.Paramset["bot"],
					Benchmark:            topAnomaly.Paramset["benchmark"],
					Story:                topAnomaly.Paramset["story"],
					Chart:                topAnomaly.Paramset["measurement"],
					AggregationMethod:    aggregationMethod,
					ImprovementDirection: topAnomaly.ImprovementDirection,
				},
			})
		// This Get() call will wait for the child workflow to start.
		if err = find_culprit_wf.GetChildWorkflowExecution().Get(ctx, nil); err != nil {
			return nil, skerr.Wrapf(err, "Child workflow failed to start.")
		}

		// Step 6. Update the anomaly group with the bisection id.
		var updateAnomalyGroupResponse *ag_pb.UpdateAnomalyGroupResponse
		if err = workflow.ExecuteActivity(ctx, agsa.UpdateAnomalyGroup, input.AnomalyGroupServiceUrl, &ag_pb.UpdateAnomalyGroupRequest{
			AnomalyGroupId: input.AnomalyGroupId,
			BisectionId:    child_wf_id,
		}).Get(ctx, &updateAnomalyGroupResponse); err != nil {
			return nil, skerr.Wrap(err)
		}
		return &workflows.MaybeTriggerBisectionResult{
			JobId: child_wf_id,
		}, nil
	} else if anomalyGroupResponse.AnomalyGroup.GroupAction == ag_pb.GroupActionType_REPORT {
		var notifyUserOfAnomalyResponse *ag_pb.UpdateAnomalyGroupResponse
		if err = workflow.ExecuteActivity(ctx, csa.NotifyUserOfAnomaly, input.CulpritServiceUrl, &c_pb.NotifyUserOfAnomalyRequest{
			AnomalyGroupId: input.AnomalyGroupId,
		}).Get(ctx, &notifyUserOfAnomalyResponse); err != nil {
			return nil, err
		}
		return &workflows.MaybeTriggerBisectionResult{}, nil
	}

	return nil, skerr.Fmt("Unhandled GroupAction type %s", anomalyGroupResponse.AnomalyGroup.GroupAction)
}
