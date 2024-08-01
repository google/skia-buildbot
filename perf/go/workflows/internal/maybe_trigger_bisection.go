package internal

import (
	"time"

	"go.skia.org/infra/go/skerr"
	ag_pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	c_pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/workflows"
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
		return &workflows.MaybeTriggerBisectionResult{
			JobId: "Unknown yet",
		}, nil
		// TODO(wenbinzhang): invoke the bisection when we can have the
		// bisection job id without waiting for its result.
		// The following invoking logic will be commeted out for now.
		// ======== start of bisection related steps ========
		// // Step 5. Invoke Bisection conditionally
		// var be *pp_pb.BisectExecution
		// if err := workflow.ExecuteChildWorkflow(ctx, pinpoint.CatapultBisect,
		// 	&pinpoint.BisectParams{
		// 		Request: &pp_pb.ScheduleBisectRequest{
		// 			ComparisonMode:       "performance",
		// 			StartGitHash:         startHash,
		// 			EndGitHash:           endHash,
		// 			Configuration:        topAnomaly.Paramset["bot"],
		// 			Benchmark:            topAnomaly.Paramset["benchmark"],
		// 			Story:                topAnomaly.Paramset["story"],
		// 			Chart:                topAnomaly.Paramset["measurement"],
		// 			AggregationMethod:    topAnomaly.Paramset["stat"],
		// 			ImprovementDirection: topAnomaly.ImprovementDirection,
		// 		},
		// 	}).Get(ctx, &be); err != nil {
		// 	return nil, skerr.Wrap(err)
		// }
		// // Step 6. Update the anomaly group with the bisection id.
		// var updateAnomalyGroupResponse *ag_pb.UpdateAnomalyGroupResponse
		// if err = workflow.ExecuteActivity(ctx, agsa.UpdateAnomalyGroup, input.AnomalyGroupServiceUrl, &ag_pb.UpdateAnomalyGroupRequest{
		// 	AnomalyGroupId: input.AnomalyGroupId,
		// 	BisectionId:    be.JobId,
		// }).Get(ctx, &updateAnomalyGroupResponse); err != nil {
		// 	return nil, skerr.Wrap(err)
		// }
		// return &workflows.MaybeTriggerBisectionResult{
		// 	JobId: be.JobId,
		// }, nil
		// ======== end of bisection related steps ========
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
