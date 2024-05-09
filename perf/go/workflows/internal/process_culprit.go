package internal

import (
	"context"

	backend "go.skia.org/infra/perf/go/backend/client"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/workflows"
	"go.temporal.io/sdk/workflow"
)

type CulpritServiceActivity struct {
	insecure_conn bool
}

func (csa *CulpritServiceActivity) PeristCulprit(ctx context.Context, culpritServiceUrl string, req *pb.PersistCulpritRequest) (*pb.PersistCulpritResponse, error) {
	client, err := backend.NewCulpritServiceClient(culpritServiceUrl, csa.insecure_conn)
	if err != nil {
		return nil, err
	}
	resp, err := client.PersistCulprit(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (csa *CulpritServiceActivity) NotifyUser(ctx context.Context, culpritServiceUrl string, req *pb.NotifyUserRequest) (*pb.NotifyUserResponse, error) {
	client, err := backend.NewCulpritServiceClient(culpritServiceUrl, csa.insecure_conn)
	if err != nil {
		return nil, err
	}
	resp, err := client.NotifyUser(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Handles processing of identified culprits.
// Stores culprit data in a persistant storage and notifies users accordingly.
func ProcessCulpritWorkflow(ctx workflow.Context, input *workflows.ProcessCulpritParam) (*workflows.ProcessCulpritResult, error) {
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
	var resp1 *pb.PersistCulpritResponse
	var resp2 *pb.NotifyUserResponse
	var err error
	var csa CulpritServiceActivity
	err = workflow.ExecuteActivity(ctx, csa.PeristCulprit, input.CulpritServiceUrl, &pb.PersistCulpritRequest{
		Commits:        input.Commits,
		AnomalyGroupId: input.AnomalyGroupId,
	}).Get(ctx, &resp1)
	if err != nil {
		return nil, err
	}
	err = workflow.ExecuteActivity(ctx, csa.NotifyUser, input.CulpritServiceUrl, &pb.NotifyUserRequest{
		CulpritIds:     resp1.CulpritIds,
		AnomalyGroupId: input.AnomalyGroupId}).Get(ctx, &resp2)
	if err != nil {
		return nil, err
	}
	return &workflows.ProcessCulpritResult{
		CulpritIds: resp1.CulpritIds,
		IssueIds:   resp2.IssueIds,
	}, nil
}
