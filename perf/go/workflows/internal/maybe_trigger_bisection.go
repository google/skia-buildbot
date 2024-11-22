package internal

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"go.skia.org/infra/go/skerr"
	ag_pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	c_pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/types"
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
		if topAnomaliesResponse != nil && len(topAnomaliesResponse.Anomalies) == 0 {
			return nil, skerr.Fmt("No anomalies found for anomalygroup %s", input.AnomalyGroupId)
		}
		topAnomaly := topAnomaliesResponse.Anomalies[0]

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

		chart, stat := parseStatisticNameFromChart(topAnomaly.Paramset["measurement"])

		benchmark := topAnomaly.Paramset["benchmark"]
		story := topAnomaly.Paramset["story"]
		if benchmarkStoriesNeedUpdate(benchmark) {
			story = updateStoryDescriptorName(story)
		}
		find_culprit_wf := workflow.ExecuteChildWorkflow(c_ctx, pinpoint.CulpritFinderWorkflow,
			&pinpoint.CulpritFinderParams{
				Request: &pp_pb.ScheduleCulpritFinderRequest{
					StartGitHash:         startHash,
					EndGitHash:           endHash,
					Configuration:        topAnomaly.Paramset["bot"],
					Benchmark:            benchmark,
					Story:                story,
					Chart:                chart,
					Statistic:            stat,
					ImprovementDirection: topAnomaly.ImprovementDirection,
				},
				CallbackParams: &pp_pb.CulpritProcessingCallbackParams{
					AnomalyGroupId:        input.AnomalyGroupId,
					CulpritServiceUrl:     input.CulpritServiceUrl,
					TemporalTaskQueueName: input.GroupingTaskQueue,
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
		// Step 3. Load Anomalies data
		var topAnomaliesResponse *ag_pb.FindTopAnomaliesResponse
		if err = workflow.ExecuteActivity(ctx, agsa.FindTopAnomalies, input.AnomalyGroupServiceUrl, &ag_pb.FindTopAnomaliesRequest{
			AnomalyGroupId: input.AnomalyGroupId,
			Limit:          10,
		}).Get(ctx, &topAnomaliesResponse); err != nil {
			return nil, skerr.Wrap(err)
		}
		if topAnomaliesResponse != nil && len(topAnomaliesResponse.Anomalies) == 0 {
			return nil, skerr.Fmt("No anomalies found for anomalygroup %s", input.AnomalyGroupId)
		}
		topAnomalies := make([]*c_pb.Anomaly, len(topAnomaliesResponse.Anomalies))
		// Currently the protos in culprit service and anomaly service are having two identical
		// copies of definition on Anomaly. We should merge them into one.
		for i, anomaly := range topAnomaliesResponse.Anomalies {
			topAnomalies[i] = &c_pb.Anomaly{
				StartCommit:          anomaly.StartCommit,
				EndCommit:            anomaly.EndCommit,
				Paramset:             anomaly.Paramset,
				ImprovementDirection: anomaly.ImprovementDirection,
			}
		}
		// Step 4. Notify the user of the top anomalies
		var notifyUserOfAnomalyResponse *ag_pb.UpdateAnomalyGroupResponse
		if err = workflow.ExecuteActivity(ctx, csa.NotifyUserOfAnomaly, input.CulpritServiceUrl, &c_pb.NotifyUserOfAnomalyRequest{
			AnomalyGroupId: input.AnomalyGroupId,
			Anomaly:        topAnomalies,
		}).Get(ctx, &notifyUserOfAnomalyResponse); err != nil {
			return nil, err
		}
		return &workflows.MaybeTriggerBisectionResult{}, nil
	}

	return nil, skerr.Fmt("Unhandled GroupAction type %s", anomalyGroupResponse.AnomalyGroup.GroupAction)
}

// Mimic the story name update in the legacy descriptor logic.
// The original source in catapult/dashboard/dashboard/common/descriptor.py
func benchmarkStoriesNeedUpdate(b string) bool {
	system_health_benchmark_prefix := "system_health"
	legacy_complex_cases_benchmarks := []string{
		"tab_switching.typical_25",
		"v8.browsing_desktop",
		"v8.browsing_desktop-future",
		"v8.browsing_mobile",
		"v8.browsing_mobile-future",
		"heap_profiling.mobile.disabled",
	}
	if strings.HasPrefix(b, system_health_benchmark_prefix) {
		return true
	}
	for _, benchmark := range legacy_complex_cases_benchmarks {
		if benchmark == b {
			return true
		}
	}
	return false
}

func updateStoryDescriptorName(s string) string {
	return strings.Replace(s, "_", ":", -1)
}

func parseStatisticNameFromChart(chart_name string) (string, string) {
	parts := strings.Split(chart_name, "_")
	part_count := len(parts)
	if part_count < 1 {
		return chart_name, ""
	}
	for _, stat := range types.AllMeasurementStats {
		if parts[part_count-1] == stat {
			return strings.Join(parts[:part_count-1], "_"), parts[part_count-1]
		}
	}
	return chart_name, ""
}
