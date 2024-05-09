package internal

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/skerr"
	pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	backend "go.skia.org/infra/perf/go/backend/client"
	"go.skia.org/infra/perf/go/workflows"
	"go.skia.org/infra/pinpoint/go/backends"
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
type GerritServiceActivity struct {
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

func (gsa *GerritServiceActivity) GetCommitRevision(ctx context.Context, commitPostion int64) (string, error) {
	client, err := backends.NewCrrevClient(ctx)
	if err != nil {
		return "", err
	}
	resp, err := client.GetCommitInfo(ctx, fmt.Sprint(commitPostion))
	if err != nil {
		return "", err
	}
	return resp.GitHash, nil
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
	var gsa GerritServiceActivity

	// Step 1. wait for some time so that more anomalies can be detected and grouped.
	if err = workflow.Sleep(ctx, _WAIT_TIME_FOR_ANOMALIES); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Step 2. Load Anomalygroup data
	if err = workflow.ExecuteActivity(ctx, agsa.LoadAnomalyGroupByID, input.AnomalyGroupServiceUrl, &pb.LoadAnomalyGroupByIDRequest{
		AnomalyGroupId: input.AnomalyGroupId,
	}).Get(ctx, &anomalyGroupResponse); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Step 3. Load Anomaly data
	var topAnomaliesResponse *pb.FindTopAnomaliesResponse
	if err = workflow.ExecuteActivity(ctx, agsa.FindTopAnomalies, input.AnomalyGroupServiceUrl, &pb.FindTopAnomaliesRequest{
		AnomalyGroupId: input.AnomalyGroupId,
		Limit:          10,
	}).Get(ctx, &topAnomaliesResponse); err != nil {
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
		// Step 4. Convert start and end commit postions to commit hash
		var startHash, endHash string
		if err = workflow.ExecuteActivity(ctx, gsa.GetCommitRevision, topAnomaly.StartCommit).Get(ctx, &startHash); err != nil {
			return nil, skerr.Wrap(err)
		}
		if err = workflow.ExecuteActivity(ctx, gsa.GetCommitRevision, topAnomaly.EndCommit).Get(ctx, &endHash); err != nil {
			return nil, skerr.Wrap(err)
		}
		// Step 5. Invoke Bisection conditionally
		if err := workflow.ExecuteChildWorkflow(ctx, pinpoint.CatapultBisect,
			&pinpoint.BisectParams{
				Request: &pinpoint_proto.ScheduleBisectRequest{
					ComparisonMode:       "performance",
					StartGitHash:         startHash,
					EndGitHash:           endHash,
					Configuration:        topAnomaly.Paramset["bot"],
					Benchmark:            topAnomaly.Paramset["benchmark"],
					Story:                topAnomaly.Paramset["story"],
					Chart:                topAnomaly.Paramset["measurement"],
					AggregationMethod:    topAnomaly.Paramset["stat"],
					ImprovementDirection: topAnomaly.ImprovementDirection,
				},
			}).Get(ctx, &be); err != nil {
			return nil, skerr.Wrap(err)
		}
		var updateAnomalyGroupResponse *pb.UpdateAnomalyGroupResponse
		if err = workflow.ExecuteActivity(ctx, agsa.UpdateAnomalyGroup, input.AnomalyGroupServiceUrl, &pb.UpdateAnomalyGroupRequest{
			AnomalyGroupId: input.AnomalyGroupId,
			BisectionId:    be.JobId,
		}).Get(ctx, &updateAnomalyGroupResponse); err != nil {
			return nil, skerr.Wrap(err)
		}
		return &workflows.MaybeTriggerBisectionResult{}, nil
	} else if anomalyGroupResponse.AnomalyGroup.GroupAction == pb.GroupActionType_REPORT {
		// TODO(b/321081776): Add logic to invoke buganizer api
		return nil, skerr.Fmt("Unimplemented error")
	}

	return nil, skerr.Fmt("Unhandled GroupAction type %s", anomalyGroupResponse.AnomalyGroup.GroupAction)
}
