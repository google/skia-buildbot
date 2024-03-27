package internal

import (
	"context"
	"time"

	backend "go.skia.org/infra/perf/go/backend/client"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/workflows"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var (
	// Default option for the regular activity.
	//
	// Activity usually communicates with the external services and is expected to complete
	// within a minute. RetryPolicy helps to recover from unexpected network errors or service
	// interruptions.
	// For activities that expect long running time and complex dependent services, a separate
	// option should be curated for individual activities.
	regularActivityOptions = workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 10,
		},
	}
)

type CulpritServiceActivity struct {
}

func (csa *CulpritServiceActivity) InvokePeristCulprit(ctx context.Context, culpritServiceUrl string, req *pb.PersistCulpritRequest) (*pb.PersistCulpritResponse, error) {
	client, err := backend.NewCulpritServiceClient(culpritServiceUrl)
	if err != nil {
		return nil, err
	}
	resp, err := client.PersistCulprit(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (csa *CulpritServiceActivity) InvokeNotifyUser(ctx context.Context, culpritServiceUrl string, req *pb.NotifyUserRequest) (*pb.NotifyUserResponse, error) {
	client, err := backend.NewCulpritServiceClient(culpritServiceUrl)
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
	err = workflow.ExecuteActivity(ctx, csa.InvokePeristCulprit, input.CulpritServiceUrl, &pb.PersistCulpritRequest{
		Commits:        input.Commits,
		AnomalyGroupId: input.AnomalyGroupId,
	}).Get(ctx, &resp1)
	if err != nil {
		return nil, err
	}
	err = workflow.ExecuteActivity(ctx, csa.InvokeNotifyUser, input.CulpritServiceUrl, &pb.NotifyUserRequest{
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
