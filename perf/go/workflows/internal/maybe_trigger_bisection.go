package internal

import (
	"errors"
	"slices"
	"strings"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	ag_pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	b_pb "go.skia.org/infra/perf/go/autobisection/proto/v1"
	c_pb "go.skia.org/infra/perf/go/culprit/proto/v1"

	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/workflows"
	"go.skia.org/infra/pinpoint/go/pinpoint"

	"go.temporal.io/sdk/workflow"
)

const (
	_WAIT_TIME_FOR_ANOMALIES = 30 * time.Minute
)

func agsaToken() *AnomalyGroupServiceActivity {
	return &AnomalyGroupServiceActivity{}
}

func gsaToken() *GerritServiceActivity {
	return &GerritServiceActivity{}
}

func csaToken() *CulpritServiceActivity {
	return &CulpritServiceActivity{}
}

func bsaToken() *AutobisectionServiceActivity {
	return &AutobisectionServiceActivity{}
}

// MaybeTriggerBisectionWorkflow is the entry point for the workflow which handles anomaly group
// processing. It is responsible for triggering a bisection if the anomalygroup's
// group action = BISECT. If group action = REPORT, files a bug notifying user of the anomalies.
func MaybeTriggerBisectionWorkflow(
	ctx workflow.Context,
	input *workflows.MaybeTriggerBisectionParam,
) (*workflows.MaybeTriggerBisectionResult, error) {

	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)

	if err := waitForAnomalyClusteringWindow(ctx); err != nil {
		return nil, skerr.Wrap(err)
	}

	anomalyGroupResponse, err := loadAnomalyGroupByID(
		ctx,
		input.AnomalyGroupServiceUrl,
		input.AnomalyGroupId,
	)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	workflow.GetLogger(ctx).Info(
		"MaybeTriggerBisectionWorkflow",
		"WorkflowID",
		workflow.GetInfo(ctx).WorkflowExecution.ID,
		"AnomalyGroup",
		input.AnomalyGroupId,
		"GroupAction",
		anomalyGroupResponse.AnomalyGroup.GroupAction,
		"AnomalyGroupServiceUrl",
		input.AnomalyGroupServiceUrl,
	)

	switch anomalyGroupResponse.AnomalyGroup.GroupAction {
	case ag_pb.GroupActionType_BISECT:
		bisectionAllowed, err := isBisectionAllowed(ctx)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if bisectionAllowed {
			res, err := processAnomaliesAsBisection(ctx, input)
			if err == nil {
				return res, nil
			}
			workflow.GetLogger(ctx).
				Error("Bisection failed, falling back to reporting.", "error", err)
		}
		// Fallback: bisection not allowed or failed.
		return processAnomaliesAsReporting(ctx, input)
	case ag_pb.GroupActionType_REPORT:
		return processAnomaliesAsReporting(ctx, input)
	case ag_pb.GroupActionType_NOACTION:
		metrics2.GetCounter("anomalygroup_ignored").Inc(1)
		return nil, nil
	default:
		return nil, skerr.Fmt(
			"Unhandled GroupAction type %s",
			anomalyGroupResponse.AnomalyGroup.GroupAction,
		)
	}
}

func processAnomaliesAsBisection(
	ctx workflow.Context,
	input *workflows.MaybeTriggerBisectionParam,
) (*workflows.MaybeTriggerBisectionResult, error) {
	anomaliesCount := 1
	topAnomaliesResponse, err := findTopAnomalies(
		ctx,
		input.AnomalyGroupServiceUrl,
		input.AnomalyGroupId,
		anomaliesCount,
	)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	topAnomaly := topAnomaliesResponse.Anomalies[0]
	startHash, endHash, err := getCommitHashes(
		ctx,
		topAnomaly.StartCommit,
		topAnomaly.EndCommit,
	)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	jobId, err := createBisectJob(
		ctx,
		topAnomaly,
		startHash,
		endHash,
	)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	workflow.GetLogger(ctx).Info("Pinpoint Job created", "jobId", jobId)

	// Update the anomaly group with the bisection id.
	updateRequest := ag_pb.UpdateAnomalyGroupRequest{
		AnomalyGroupId: input.AnomalyGroupId,
		BisectionId:    jobId,
	}
	if err = updateAnomalyGroup(ctx, input.AnomalyGroupServiceUrl, &updateRequest); err != nil {
		return nil, skerr.Wrap(err)
	}

	jobState, err := waitPinpointJobCompletion(ctx, jobId)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	if err := processBisectJobResults(ctx, jobState, topAnomaly, input.AnomalyGroupId, input.AutobisectionServiceUrl); err != nil {
		return nil, skerr.Wrap(err)
	}

	metrics2.GetCounter("anomalygroup_bisected").Inc(1)
	return &workflows.MaybeTriggerBisectionResult{
		JobId: jobId,
	}, nil
}

func processAnomaliesAsReporting(
	ctx workflow.Context,
	input *workflows.MaybeTriggerBisectionParam,
) (*workflows.MaybeTriggerBisectionResult, error) {
	// Load Anomalies data
	anomaliesCount := 10
	topAnomaliesResponse, err := findTopAnomalies(
		ctx,
		input.AnomalyGroupServiceUrl,
		input.AnomalyGroupId,
		anomaliesCount,
	)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	topAnomalies := convertToCulpritAnomalies(topAnomaliesResponse.Anomalies)

	notifyUserOfAnomalyResponse, err := notifyUserOfAnomalies(
		ctx,
		topAnomalies,
		input.CulpritServiceUrl,
		input.AnomalyGroupId,
	)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Update the anomaly group with the reported issue id.
	if notifyUserOfAnomalyResponse != nil && notifyUserOfAnomalyResponse.IssueId != "" {
		updateRequest := ag_pb.UpdateAnomalyGroupRequest{
			AnomalyGroupId: input.AnomalyGroupId,
			IssueId:        notifyUserOfAnomalyResponse.IssueId,
		}
		if err = updateAnomalyGroup(ctx, input.AnomalyGroupServiceUrl, &updateRequest); err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	metrics2.GetCounter("anomalygroup_reported").Inc(1)
	return &workflows.MaybeTriggerBisectionResult{}, nil
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

// waitForAnomalyClusteringWindow waits for some time so that more anomalies can
// be detected and grouped.
func waitForAnomalyClusteringWindow(ctx workflow.Context) error {
	if err := workflow.Sleep(ctx, _WAIT_TIME_FOR_ANOMALIES); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func loadAnomalyGroupByID(
	ctx workflow.Context,
	url string,
	anomalyGroupID string,
) (*ag_pb.LoadAnomalyGroupByIDResponse, error) {
	var anomalyGroupResponse *ag_pb.LoadAnomalyGroupByIDResponse
	err := workflow.ExecuteActivity(ctx, agsaToken().LoadAnomalyGroupByID, url,
		&ag_pb.LoadAnomalyGroupByIDRequest{
			AnomalyGroupId: anomalyGroupID,
		}).
		Get(ctx, &anomalyGroupResponse)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return anomalyGroupResponse, nil
}

func isBisectionAllowed(ctx workflow.Context) (bool, error) {
	var bisectionAllowed bool
	err := workflow.ExecuteActivity(ctx, agsaToken().CheckBisectionAllowed).
		Get(ctx, &bisectionAllowed)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	workflow.GetLogger(ctx).Info(
		"MaybeTriggerBisectionWorkflow",
		"Bisection allowed",
		bisectionAllowed,
	)
	return bisectionAllowed, nil
}

func findTopAnomalies(
	ctx workflow.Context,
	url string,
	anomalyGroupID string,
	limit int,
) (*ag_pb.FindTopAnomaliesResponse, error) {
	var topAnomaliesResponse *ag_pb.FindTopAnomaliesResponse
	if err := workflow.ExecuteActivity(ctx, agsaToken().FindTopAnomalies, url, &ag_pb.FindTopAnomaliesRequest{
		AnomalyGroupId: anomalyGroupID,
		Limit:          int64(limit),
	}).Get(ctx, &topAnomaliesResponse); err != nil {
		return nil, skerr.Wrap(err)
	}
	if topAnomaliesResponse == nil || len(topAnomaliesResponse.Anomalies) == 0 {
		return nil, skerr.Fmt("No anomalies found for anomalygroup %s", anomalyGroupID)
	}
	return topAnomaliesResponse, nil
}

// Currently the protos in culprit service and anomaly service are having two identical
// copies of definition on Anomaly. We should merge them into one.
func convertToCulpritAnomalies(anomalies []*ag_pb.Anomaly) []*c_pb.Anomaly {
	result := make([]*c_pb.Anomaly, len(anomalies))
	for i, anomaly := range anomalies {
		result[i] = &c_pb.Anomaly{
			StartCommit:          anomaly.StartCommit,
			EndCommit:            anomaly.EndCommit,
			Paramset:             anomaly.Paramset,
			ImprovementDirection: anomaly.ImprovementDirection,
			MedianBefore:         anomaly.MedianBefore,
			MedianAfter:          anomaly.MedianAfter,
		}
	}
	return result
}

// getCommitHashes converts start and end commit postions to commit hash.
func getCommitHashes(
	ctx workflow.Context,
	startCommit int64,
	endCommit int64,
) (string, string, error) {
	var startHash, endHash string
	if err := workflow.ExecuteActivity(ctx, gsaToken().GetCommitRevision, startCommit).
		Get(ctx, &startHash); err != nil {
		return "", "", skerr.Wrap(err)
	}
	if err := workflow.ExecuteActivity(ctx, gsaToken().GetCommitRevision, endCommit).
		Get(ctx, &endHash); err != nil {
		return "", "", skerr.Wrap(err)
	}
	return startHash, endHash, nil
}

func createBisectJob(
	ctx workflow.Context,
	anomaly *ag_pb.Anomaly,
	startHash, endHash string,
) (string, error) {
	story, chart, stat := parseStoryChartStat(anomaly)
	req := pinpoint.BisectJobCreateRequest{
		ComparisonMode: "performance",
		StartGitHash:   startHash,
		EndGitHash:     endHash,
		Configuration:  anomaly.Paramset["bot"],
		Benchmark:      anomaly.Paramset["benchmark"],
		Story:          story,
		Chart:          chart,
		Statistic:      stat,
		TestPath:       anomaly.Paramset["test_path"],
	}
	resp, err := executeBisectJobActivity(ctx, req)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	if resp.JobID == "" {
		return "", skerr.Wrap(errors.New("Chromeperf failed to create a new job"))
	}
	return resp.JobID, nil
}

func executeBisectJobActivity(
	ctx workflow.Context,
	req pinpoint.BisectJobCreateRequest,
) (resp *pinpoint.CreatePinpointResponse, err error) {
	var ppc *pinpoint.Client
	activity := workflow.ExecuteActivity(
		ctx,
		ppc.CreateBisect,
		&req,
		true, // isNewAnomaly is set to true to avoid updating chromeperf anomalies.
	)
	if err = activity.Get(ctx, &resp); err != nil {
		return nil, skerr.Wrap(err)
	}
	return resp, nil
}

func updateAnomalyGroup(
	ctx workflow.Context,
	url string,
	req *ag_pb.UpdateAnomalyGroupRequest,
) error {
	var updateAnomalyGroupResponse *ag_pb.UpdateAnomalyGroupResponse
	future := workflow.ExecuteActivity(ctx, agsaToken().UpdateAnomalyGroup, url, req)
	if err := future.Get(ctx, &updateAnomalyGroupResponse); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func notifyUserOfAnomalies(
	ctx workflow.Context,
	anomalies []*c_pb.Anomaly,
	culpritServiceUrl, anomalyGroupId string,
) (*c_pb.NotifyUserOfAnomalyResponse, error) {
	var notifyUserOfAnomalyResponse *c_pb.NotifyUserOfAnomalyResponse
	request := c_pb.NotifyUserOfAnomalyRequest{
		AnomalyGroupId: anomalyGroupId,
		Anomaly:        anomalies,
	}
	future := workflow.ExecuteActivity(
		ctx,
		csaToken().NotifyUserOfAnomaly,
		culpritServiceUrl,
		&request,
	)
	if err := future.Get(ctx, &notifyUserOfAnomalyResponse); err != nil {
		return nil, skerr.Wrap(err)
	}
	return notifyUserOfAnomalyResponse, nil
}

func parseStoryChartStat(anomaly *ag_pb.Anomaly) (string, string, string) {
	chart, stat := parseStatisticNameFromChart(anomaly.Paramset["measurement"])

	story := anomaly.Paramset["story"]
	if benchmarkStoriesNeedUpdate(anomaly.Paramset["benchmark"]) {
		story = updateStoryDescriptorName(story)
	}
	return story, chart, stat
}

// waitPinpointJobCompletion waits while a pinpoint jobs is in progress by
// polling it's status.
func waitPinpointJobCompletion(
	ctx workflow.Context,
	jobId string,
) (*pinpoint.FetchJobStateResponse, error) {
	// In case of increasing the timeout, keep in mind the workflow runner timeout
	// settings in perf/go/anomalygroup/utils/anomalygrouputils.go
	timeout := 10 * time.Hour
	interval := 30 * time.Minute
	startTime := workflow.Now(ctx)
	for {
		if err := workflow.Sleep(ctx, interval); err != nil {
			return nil, skerr.Wrap(err)
		}

		resp, err := executeFetchJobStateActivity(ctx, jobId)
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		doneStatuses := []string{"completed", "failed", "cancelled"}
		if slices.Contains(doneStatuses, strings.ToLower(resp.Status)) {
			return resp, nil
		}

		if workflow.Now(ctx).Sub(startTime) > timeout {
			return nil, skerr.Fmt("Pinpoint job timeout: %s", jobId)
		}
	}
}

func executeFetchJobStateActivity(
	ctx workflow.Context,
	jobId string,
) (resp *pinpoint.FetchJobStateResponse, err error) {
	var ppc *pinpoint.Client
	activity := workflow.ExecuteActivity(
		ctx,
		ppc.FetchJobState,
		pinpoint.FetchJobStateRequest{JobID: jobId},
	)
	err = activity.Get(ctx, &resp)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return resp, nil
}

func processBisectJobResults(
	ctx workflow.Context,
	jobState *pinpoint.FetchJobStateResponse,
	anomaly *ag_pb.Anomaly,
	anomalyGroupId string,
	autobisectionServiceUrl string,
) error {
	culprits, err := extractCulprits(jobState)
	if err != nil {
		return skerr.Wrap(err)
	}

	autobisectionReq := &b_pb.SaveAutobisectionRequest{
		JobId:            jobState.JobID,
		AnomalyGroupId:   anomalyGroupId,
		AnomalyId:        anomaly.Id,
		IsRealRegression: isRealRegression(jobState),
	}

	workflow.GetLogger(ctx).Info(
		"processBisectJobResults",
		"WorkflowID",
		workflow.GetInfo(ctx).WorkflowExecution.ID,
		"Bisect job",
		jobState.JobID,
		"Job status",
		jobState.Status,
		"Culprits",
		culprits,
		"Real regression",
		autobisectionReq.IsRealRegression,
	)

	// Save the autobisection results to the database.
	if err := workflow.ExecuteActivity(ctx, bsaToken().SaveAutobisection, autobisectionServiceUrl, autobisectionReq).Get(ctx, nil); err != nil {
		return skerr.Wrap(err)
	}

	return nil
}

func extractCulprits(jobState *pinpoint.FetchJobStateResponse) (culprits []string, err error) {
	for _, stateItem := range jobState.State {
		if value, ok := stateItem.Comparisons["prev"]; !ok || value != "different" {
			continue
		}

		for _, commit := range stateItem.Change.Commits {
			if commit.Repository == "chromium" {
				culprits = append(culprits, commit.GitHash)
			}
		}
	}

	return culprits, nil
}

func isRealRegression(jobState *pinpoint.FetchJobStateResponse) bool {
	// If the initial sandwich verification run shows no statistical significant
	// difference, no further bisection is done. In this case, the job has only 2
	// start commit and end commit states. If the initial sandwich verification
	// run shows that there is a real regression, bisection starts. In that case,
	// there are more states than 2 states.
	// If there are more than 2 states but no culprit found, that means the
	// regression is real but pinpoint failed to find the culprit commit.
	hasCulprit := jobState.DifferenceCount != nil && *jobState.DifferenceCount > 0
	return hasCulprit || len(jobState.State) > 2
}
