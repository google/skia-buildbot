package main

import (
	"context"
	"flag"
	"fmt"
	"os/user"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/internal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"

	pb "go.skia.org/infra/pinpoint/proto/v1"
)

var (
	// Run the following command to portforward Temporal service so the client can connect to it.
	// kubectl port-forward service/temporal --address 0.0.0.0 -n temporal 7233:7233
	hostPort                = flag.String("hostPort", "localhost:7233", "Host the worker connects to.")
	namespace               = flag.String("namespace", "default", "The namespace the worker registered to.")
	taskQueue               = flag.String("taskQueue", "", "Task queue name registered to worker services.")
	commit                  = flag.String("commit", "611b5a084486cd6d99a0dad63f34e320a2ebc2b3", "Git commit hash to build Chrome.")
	triggerBisectFlag       = flag.Bool("bisect", false, "toggle true to trigger bisect workflow")
	triggerSingleCommitFlag = flag.Bool("single-commit", false, "toggle true to trigger single commit runner workflow")
)

func defaultWorkflowOptions() client.StartWorkflowOptions {
	return client.StartWorkflowOptions{
		ID:        uuid.New().String(),
		TaskQueue: *taskQueue,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    30 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    1,
		},
	}
}

func triggerBisectWorkflow(c client.Client) *pb.BisectExecution {
	ctx := context.Background()
	// based off of https://pinpoint-dot-chromeperf.appspot.com/job/17ab3cfa9e0000
	p := &workflows.BisectParams{
		Request: &pb.ScheduleBisectRequest{
			ComparisonMode:      "performance",
			StartGitHash:        "8f2037564966f83e53701d157622dd42b931a13f", // 1266617
			EndGitHash:          "049ab03450dd980d3afc27f13edfef9f510ed819", // 1266622
			Configuration:       "win-11-perf",
			Benchmark:           "system_health.memory_desktop",
			Story:               "load:chrome:blank",
			Chart:               "memory:chrome:all_processes:reported_by_chrome:cc:effective_size",
			ComparisonMagnitude: "786432.0",
			// TODO(@sunxiaodi): support optional aggregation method
			AggregationMethod: "mean",
			Project:           "chromium",
		},
	}
	var be *pb.BisectExecution
	we, err := c.ExecuteWorkflow(ctx, defaultWorkflowOptions(), internal.BisectWorkflow, p)
	if err != nil {
		sklog.Fatalf("Unable to execute workflow: %v", err)
		return nil
	}
	sklog.Infof("Started workflow.. WorkflowID: %v RunID: %v", we.GetID(), we.GetRunID())

	if err := we.Get(ctx, &be); err != nil {
		sklog.Fatalf("Unable to get result: %v", err)
		return nil
	}
	return be
}

func triggerSingleCommitRunner(c client.Client) *internal.CommitRun {
	ctx := context.Background()
	p := &internal.SingleCommitRunnerParams{
		PinpointJobID:     "123",
		BotConfig:         "win-11-perf",
		Benchmark:         "v8.browsing_desktop",
		Story:             "browse:social:twitter_infinite_scroll:2018",
		Chart:             "v8:gc:cycle:main_thread:young:atomic",
		AggregationMethod: "mean",
		CombinedCommit: &midpoint.CombinedCommit{
			Main: &midpoint.Commit{GitHash: *commit},
		},
		Iterations: 3,
	}
	var cr *internal.CommitRun
	we, err := c.ExecuteWorkflow(ctx, defaultWorkflowOptions(), workflows.SingleCommitRunner, p)
	if err != nil {
		sklog.Fatalf("Unable to execute workflow: %v", err)
		return nil
	}
	sklog.Infof("Started workflow.. WorkflowID: %v RunID: %v", we.GetID(), we.GetRunID())

	if err := we.Get(ctx, &cr); err != nil {
		sklog.Fatalf("Unable to get result: %v", err)
		return nil
	}
	return cr
}

func triggerBuildChrome(c client.Client) *swarmingV1.SwarmingRpcsCASReference {
	bcp := workflows.BuildChromeParams{
		PinpointJobID: "123",
		Commit: &midpoint.CombinedCommit{
			Main: &midpoint.Commit{GitHash: *commit},
		},
		Device: "mac-m1_mini_2020-perf",
		Target: "performance_test_suite",
	}
	we, err := c.ExecuteWorkflow(context.Background(), defaultWorkflowOptions(), workflows.BuildChrome, bcp)
	if err != nil {
		sklog.Fatalf("Unable to execute workflow: %v", err)
		return nil
	}

	sklog.Infof("Started workflow.. WorkflowID: %v RunID: %v", we.GetID(), we.GetRunID())

	// Synchronously wait for the workflow completion.
	var result *swarmingV1.SwarmingRpcsCASReference
	err = we.Get(context.Background(), &result)
	if err != nil {
		sklog.Errorf("Unable get workflow result: %v", err)
	}
	return result
}

// Sample client to trigger a BuildChrome workflow.
func main() {
	flag.Parse()

	if *taskQueue == "" {
		if u, err := user.Current(); err != nil {
			sklog.Fatalf("Unable to get the current user: %s", err)
		} else {
			*taskQueue = fmt.Sprintf("localhost.%s", u.Username)
		}
	}

	// The client is a heavyweight object that should be created once per process.
	c, err := client.Dial(client.Options{
		HostPort:  *hostPort,
		Namespace: *namespace,
	})
	if err != nil {
		sklog.Errorf("Unable to create client", err)
		return
	}
	defer c.Close()

	if *triggerBisectFlag {
		result := triggerBisectWorkflow(c)
		sklog.Infof("Workflow result: %v", spew.Sdump(result))
	}
	if *triggerSingleCommitFlag {
		result := triggerSingleCommitRunner(c)
		sklog.Infof("Workflow result: %v", spew.Sdump(result))
	}
}
