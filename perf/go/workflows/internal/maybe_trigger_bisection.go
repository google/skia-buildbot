package internal

import (
	"context"
	"time"

	"go.skia.org/infra/go/skerr"
	pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	backend "go.skia.org/infra/perf/go/backend/client"
	"go.skia.org/infra/perf/go/workflows"
	pinpoint "go.skia.org/infra/pinpoint/go/workflows"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/workflow"
)

const (
	_WAIT_TIME_FOR_ANOMALIES = 30 * time.Minute
)

type AnomalyGroupServiceActivity struct {
	insecure_conn bool
}

func (agsa *AnomalyGroupServiceActivity) LoadAnomalyGroupByID(ctx context.Context, anomalygroupServiceUrl string, req *pb.LoadAnomalyGroupByIDRequest) (*pb.LoadAnomalyGroupByIDResponse, error) {
	client, err := backend.NewAnomalyGroupServiceClient(anomalygroupServiceUrl, agsa.insecure_conn)
	if err != nil {
		return nil, err
	}
	resp, err := client.LoadAnomalyGroupByID(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (agsa *AnomalyGroupServiceActivity) FindTopAnomalies(ctx context.Context, anomalygroupServiceUrl string, req *pb.FindTopAnomaliesRequest) (*pb.FindTopAnomaliesResponse, error) {
	client, err := backend.NewAnomalyGroupServiceClient(anomalygroupServiceUrl, agsa.insecure_conn)
	if err != nil {
		return nil, err
	}
	resp, err := client.FindTopAnomalies(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (agsa *AnomalyGroupServiceActivity) UpdateAnomalyGroup(ctx context.Context, anomalygroupServiceUrl string, req *pb.UpdateAnomalyGroupRequest) (*pb.UpdateAnomalyGroupResponse, error) {
	client, err := backend.NewAnomalyGroupServiceClient(anomalygroupServiceUrl, agsa.insecure_conn)
	if err != nil {
		return nil, err
	}
	resp, err := client.UpdateAnomalyGroup(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// MaybeTriggerBisectionWorkflow is the entry point for the workflow which handles anomaly group
// processing. It is responsible for triggering a bisection if the anomalygroup's
// group action = BISECT. If group action = REPORT, files a bug notifying user of the anomalies.
func MaybeTriggerBisectionWorkflow(ctx workflow.Context, input *workflows.MaybeTriggerBisectionParam) (*workflows.MaybeTriggerBisectionResult, error) {
	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
	var anomalyGroupResponse *pb.LoadAnomalyGroupByIDResponse
	var err error
	var agsa AnomalyGroupServiceActivity

	// Step 1. wait for some time so that more anomalies can be detected and grouped.
	err = workflow.Sleep(ctx, _WAIT_TIME_FOR_ANOMALIES)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Step 2. Load Anomalygroup data
	err = workflow.ExecuteActivity(ctx, agsa.LoadAnomalyGroupByID, input.AnomalyGroupServiceUrl, &pb.LoadAnomalyGroupByIDRequest{
		AnomalyGroupId: input.AnomalyGroupId,
	}).Get(ctx, &anomalyGroupResponse)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Step 3. Load Anomaly data
	var topAnomaliesResponse *pb.FindTopAnomaliesResponse
	err = workflow.ExecuteActivity(ctx, agsa.FindTopAnomalies, input.AnomalyGroupServiceUrl, &pb.FindTopAnomaliesRequest{
		AnomalyGroupId: input.AnomalyGroupId,
		Limit:          10,
	}).Get(ctx, &topAnomaliesResponse)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var topAnomaly *pb.Anomaly
	if len(topAnomaliesResponse.Anomalies) <= 0 {
		return nil, skerr.Fmt("No anomalies found for anomalygroup %s", input.AnomalyGroupId)
	} else {
		topAnomaly = topAnomaliesResponse.Anomalies[0]
	}

	var be *pinpoint_proto.BisectExecution
	if anomalyGroupResponse.AnomalyGroup.GroupAction == pb.GroupActionType_BISECT {
		// Step 4. Convert commit postions to commit hash
		// TODO(b/321081776): Implement conversion from commit positions to commit hash.
		startCommit := "test"
		endCommit := "test"

		// Step 5. Invoke Bisection conditionally
		err := workflow.ExecuteChildWorkflow(ctx, pinpoint.CatapultBisect,
			&pinpoint.BisectParams{
				Request: &pinpoint_proto.ScheduleBisectRequest{
					ComparisonMode:       "performance",
					StartGitHash:         startCommit,
					EndGitHash:           endCommit,
					Configuration:        topAnomaly.Paramset["bot"],
					Benchmark:            topAnomaly.Paramset["benchmark"],
					Story:                topAnomaly.Paramset["story"],
					Chart:                topAnomaly.Paramset["measurement"],
					AggregationMethod:    topAnomaly.Paramset["stat"],
					ImprovementDirection: topAnomaly.ImprovementDirection,
				},
			}).Get(ctx, &be)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		var updateAnomalyGroupResponse *pb.UpdateAnomalyGroupResponse
		err = workflow.ExecuteActivity(ctx, agsa.UpdateAnomalyGroup, input.AnomalyGroupServiceUrl, &pb.UpdateAnomalyGroupRequest{
			AnomalyGroupId: input.AnomalyGroupId,
			BisectionId:    be.JobId,
		}).Get(ctx, &updateAnomalyGroupResponse)
		if err != nil {
			return nil, err
		}
		return &workflows.MaybeTriggerBisectionResult{}, nil
	} else if anomalyGroupResponse.AnomalyGroup.GroupAction == pb.GroupActionType_REPORT {
		// TODO(b/321081776): Add logic to invoke buganizer api
		return nil, skerr.Fmt("Unimplemented error")
	}

	return nil, skerr.Fmt("Unhandled GroupAction type %s", anomalyGroupResponse.AnomalyGroup.GroupAction)
}
