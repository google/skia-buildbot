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
	"go.skia.org/infra/go/skerr"
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
	triggerPairwiseFlag     = flag.Bool("pairwise", false, "toggle true to trigger pairwise commit runner workflow")
	triggerBugUpdateFlag    = flag.Bool("update-bug", false, "toggle true to trigger post bug comment workflow")
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

func triggerBisectWorkflow(c client.Client) (*internal.BisectExecution, error) {
	ctx := context.Background()
	// based off of https://pinpoint-dot-chromeperf.appspot.com/job/17ab3cfa9e0000
	p := &workflows.BisectParams{
		Request: &pb.ScheduleBisectRequest{
			ComparisonMode:       "performance",
			StartGitHash:         "8f2037564966f83e53701d157622dd42b931a13f", // 1266617
			EndGitHash:           "049ab03450dd980d3afc27f13edfef9f510ed819", // 1266622
			Configuration:        "win-11-perf",
			Benchmark:            "system_health.memory_desktop",
			Story:                "load:chrome:blank",
			Chart:                "memory:chrome:all_processes:reported_by_chrome:cc:effective_size",
			ComparisonMagnitude:  "786432.0",
			AggregationMethod:    "mean",
			Project:              "chromium",
			ImprovementDirection: "DOWN",
		},
	}
	var be *internal.BisectExecution
	we, err := c.ExecuteWorkflow(ctx, defaultWorkflowOptions(), internal.BisectWorkflow, p)
	if err != nil {
		return nil, skerr.Wrapf(err, "Unable to execute workflow")
	}
	sklog.Infof("Started workflow.. WorkflowID: %v RunID: %v", we.GetID(), we.GetRunID())

	if err := we.Get(ctx, &be); err != nil {
		return nil, skerr.Wrapf(err, "Unable to get result")
	}
	return be, nil
}

func triggerPairwiseRunner(c client.Client) (*internal.PairwiseRun, error) {
	ctx := context.Background()
	// based off of https://pinpoint-dot-chromeperf.appspot.com/job/179a34b2be0000
	p := &internal.PairwiseCommitsRunnerParams{
		SingleCommitRunnerParams: internal.SingleCommitRunnerParams{
			PinpointJobID:     "179a34b2be0000",
			BotConfig:         "android-pixel4-perf",
			Benchmark:         "blink_perf.bindings",
			Story:             "gc-mini-tree.html",
			Chart:             "gc-mini-tree",
			AggregationMethod: "mean",
			CombinedCommit:    midpoint.NewCombinedCommit(&pb.Commit{GitHash: *commit}),
			Iterations:        6,
		},
		Seed: 54321,
		LeftBuild: workflows.Build{
			BuildChromeParams: workflows.BuildChromeParams{
				Commit: midpoint.NewCombinedCommit(&pb.Commit{GitHash: "573a50658f4301465569c3faf00a145093a1fe9b"}), // 1284448
			},
			CAS: &swarmingV1.SwarmingRpcsCASReference{
				CasInstance: "projects/chrome-swarming/instances/default_instance",
				Digest: &swarmingV1.SwarmingRpcsDigest{
					Hash:      "062ccf0a30a362d8e4df3c9b82172a78e3d62c2990eb30927f5863a6b08e80bb",
					SizeBytes: 810,
				},
			},
		},
		RightBuild: workflows.Build{
			BuildChromeParams: workflows.BuildChromeParams{
				Commit: midpoint.NewCombinedCommit(&pb.Commit{GitHash: "a633e198b79b2e0c83c72a3006cdffe642871e22"}), // 1284449
			},
			CAS: &swarmingV1.SwarmingRpcsCASReference{
				CasInstance: "projects/chrome-swarming/instances/default_instance",
				Digest: &swarmingV1.SwarmingRpcsDigest{
					Hash:      "51845150f953c33ee4c0900589ba916ca28b7896806460aa8935c0de2b209db6",
					SizeBytes: 810,
				},
			},
		},
	}

	var pr *internal.PairwiseRun
	we, err := c.ExecuteWorkflow(ctx, defaultWorkflowOptions(), workflows.PairwiseCommitsRunner, p)
	if err != nil {
		return nil, skerr.Wrapf(err, "Unable to execute workflow")
	}
	sklog.Infof("Started workflow.. WorkflowID: %v RunID: %v", we.GetID(), we.GetRunID())

	if err := we.Get(ctx, &pr); err != nil {
		return nil, skerr.Wrapf(err, "Unable to get result")
	}
	return pr, nil
}

func triggerSingleCommitRunner(c client.Client) (*internal.CommitRun, error) {
	ctx := context.Background()
	p := &internal.SingleCommitRunnerParams{
		PinpointJobID:     "123",
		BotConfig:         "win-11-perf",
		Benchmark:         "v8.browsing_desktop",
		Story:             "browse:social:twitter_infinite_scroll:2018",
		Chart:             "v8:gc:cycle:main_thread:young:atomic",
		AggregationMethod: "mean",
		CombinedCommit:    midpoint.NewCombinedCommit(&pb.Commit{GitHash: *commit}),
		Iterations:        3,
	}
	var cr *internal.CommitRun
	we, err := c.ExecuteWorkflow(ctx, defaultWorkflowOptions(), workflows.SingleCommitRunner, p)
	if err != nil {
		return nil, skerr.Wrapf(err, "Unable to execute workflow")
	}
	sklog.Infof("Started workflow.. WorkflowID: %v RunID: %v", we.GetID(), we.GetRunID())

	if err := we.Get(ctx, &cr); err != nil {
		return nil, skerr.Wrapf(err, "Unable to get result")
	}
	return cr, nil
}

func triggerBuildChrome(c client.Client) *swarmingV1.SwarmingRpcsCASReference {
	bcp := workflows.BuildChromeParams{
		WorkflowID: "123",
		Commit:     midpoint.NewCombinedCommit(&pb.Commit{GitHash: *commit}),
		Device:     "mac-m1_mini_2020-perf",
		Target:     "performance_test_suite",
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

func triggerBugUpdateWorkflow(c client.Client) (bool, error) {
	ctx := context.Background()

	var success bool
	we, err := c.ExecuteWorkflow(ctx, defaultWorkflowOptions(), workflows.BugUpdate, 333705433, "hello world")
	if err != nil {
		return false, skerr.Wrapf(err, "Unable to execute the workflow")
	}
	sklog.Infof("Started workflow.. WorkflowID: %v RunID: %v", we.GetID(), we.GetRunID())

	if err := we.Get(ctx, &success); err != nil {
		return false, skerr.Wrapf(err, "Unable to write to buganizer")
	}
	return success, nil
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

	var result interface{}
	if *triggerBisectFlag {
		result, err = triggerBisectWorkflow(c)
	}
	if *triggerSingleCommitFlag {
		result, err = triggerSingleCommitRunner(c)
	}
	if *triggerPairwiseFlag {
		result, err = triggerPairwiseRunner(c)
	}
	if *triggerBugUpdateFlag {
		result, err = triggerBugUpdateWorkflow(c)
	}
	if err != nil {
		sklog.Errorf("Workflow failed:", err)
		return
	}
	sklog.Infof("Workflow result: %v", spew.Sdump(result))
}
