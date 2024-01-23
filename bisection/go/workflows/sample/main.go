package main

import (
	"context"
	"flag"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/bisection/go/workflows"
	"go.skia.org/infra/go/sklog"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

var (
	// Run the following command to portforward Temporal service so the client can connect to it.
	// kubectl port-forward service/temporal --address 0.0.0.0 -n temporal 7233:7233
	hostPort  = flag.String("hostPort", "localhost:7233", "Host the worker connects to.")
	taskQueue = flag.String("taskQueue", "localhost.dev", "Task queue name registered to worker services.")
	commit    = flag.String("commit", "23c9ed2e467ff7ef8c3cbcbd3228cc1eb9f128de", "Git commit hash to build Chrome.")
)

// Sample client to trigger a BuildChrome workflow.
func main() {
	flag.Parse()

	// The client is a heavyweight object that should be created once per process.
	c, err := client.Dial(client.Options{
		HostPort: *hostPort,
	})
	if err != nil {
		sklog.Errorf("Unable to create client", err)
		return
	}
	defer c.Close()

	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.New().String(),
		TaskQueue: *taskQueue,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    30 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    1,
		},
	}

	bcp := workflows.BuildChromeParams{
		PinpointJobID: "123",
		Commit:        *commit,
		Device:        "mac-m1_mini_2020-perf",
		Target:        "performance_test_suite",
	}
	we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, workflows.BuildChrome, bcp)
	if err != nil {
		sklog.Fatalf("Unable to execute workflow: %v", err)
		return
	}

	sklog.Infof("Started workflow.. WorkflowID: %v RunID: %v", we.GetID(), we.GetRunID())

	// Synchronously wait for the workflow completion.
	var result *swarmingV1.SwarmingRpcsCASReference
	err = we.Get(context.Background(), &result)
	if err != nil {
		sklog.Errorf("Unable get workflow result: %v", err)
	}
	sklog.Infof("Workflow result: %v", spew.Sdump(result))
}
