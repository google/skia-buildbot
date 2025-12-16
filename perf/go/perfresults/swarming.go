package perfresults

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.chromium.org/luci/common/retry"
	"go.chromium.org/luci/grpc/prpc"
	swarmingv2 "go.chromium.org/luci/swarming/proto/api_v2"
	swarmingv2grpc "go.chromium.org/luci/swarming/proto/api_v2/grpcpb"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2/google"
)

// swarmingClient wraps swarmingv2.TasksClient to provide convenient functions
type swarmingClient struct {
	swarmingv2grpc.TasksClient
}

func newSwarmingClient(ctx context.Context, swarmingHost string, client *http.Client) (*swarmingClient, error) {
	if client == nil {
		ts, err := google.DefaultTokenSource(ctx)
		if err != nil {
			return nil, skerr.Wrapf(err, "unable to fetch token source")
		}

		client = httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	}

	prpc := &prpc.Client{
		C:    client,
		Host: swarmingHost,
		Options: &prpc.Options{
			Retry: func() retry.Iterator {
				return &retry.ExponentialBackoff{
					MaxDelay: time.Minute,
					Limited: retry.Limited{
						Delay:   time.Second,
						Retries: 1,
					},
				}
			},
			PerRPCTimeout: 90 * time.Second,
		},
	}
	return &swarmingClient{
		TasksClient: swarmingv2grpc.NewTasksClient(prpc),
	}, nil
}

func (client *swarmingClient) findTaskCASOutputs(ctx context.Context, taskIDs ...string) ([]*swarmingv2.CASReference, error) {
	all_cas := make([]*swarmingv2.CASReference, len(taskIDs))
	for i, t := range taskIDs {
		resp, err := client.GetResult(ctx, &swarmingv2.TaskIdWithPerfRequest{
			TaskId: t,
		})

		// We short-cut on any errors occured in the middle, expecting this CAS outputs are
		// generally available if the task is completed.
		if err != nil {
			return nil, skerr.Wrapf(err, "unable to get the cas output")
		}
		all_cas[i] = resp.GetCasOutputRoot()
	}

	return all_cas, nil
}

// findChildTaskIds returns all the children taskIds
func (client *swarmingClient) findChildTaskIds(ctx context.Context, taskID string) ([]string, error) {
	// We need the task start and completed time to narrow down the results.
	parentTask, err := client.TasksClient.GetResult(ctx, &swarmingv2.TaskIdWithPerfRequest{
		TaskId: taskID,
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "unable to get parent task details")
	}
	// runId shares the same prefix but ends with 1
	runIdPrefix := parentTask.TaskId
	runIdPrefix = runIdPrefix[:len(runIdPrefix)-1]

	resp, err := client.TasksClient.ListTasks(ctx, &swarmingv2.TasksWithPerfRequest{
		End:   parentTask.CompletedTs,
		Start: parentTask.CreatedTs,
		State: swarmingv2.StateQuery_QUERY_ALL,
		Tags:  []string{fmt.Sprintf("parent_task_id:%s1", runIdPrefix)},
		Limit: 1000, // 1000 is enough to find all the child tasks so we don't need to paginate
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "unable to find all the children tasks")
	}
	taskIDs := make([]string, len(resp.Items))
	for i, t := range resp.Items {
		taskIDs[i] = t.TaskId
	}
	return taskIDs, nil
}
