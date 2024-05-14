package internal

import (
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/workflows"
	"go.temporal.io/sdk/workflow"
)

// Handles processing of identified culprits.
// Stores culprit data in a persistant storage and notifies users accordingly.
func ProcessCulpritWorkflow(ctx workflow.Context, input *workflows.ProcessCulpritParam) (*workflows.ProcessCulpritResult, error) {
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
	var resp1 *pb.PersistCulpritResponse
	var resp2 *pb.NotifyUserOfCulpritResponse
	var err error
	var csa CulpritServiceActivity
	if err = workflow.ExecuteActivity(ctx, csa.PeristCulprit, input.CulpritServiceUrl, &pb.PersistCulpritRequest{
		Commits:        input.Commits,
		AnomalyGroupId: input.AnomalyGroupId,
	}).Get(ctx, &resp1); err != nil {
		return nil, err
	}
	if err = workflow.ExecuteActivity(ctx, csa.NotifyUserOfCulprit, input.CulpritServiceUrl, &pb.NotifyUserOfCulpritRequest{
		CulpritIds:     resp1.CulpritIds,
		AnomalyGroupId: input.AnomalyGroupId}).Get(ctx, &resp2); err != nil {
		return nil, err
	}
	return &workflows.ProcessCulpritResult{
		CulpritIds: resp1.CulpritIds,
		IssueIds:   resp2.IssueIds,
	}, nil
}
