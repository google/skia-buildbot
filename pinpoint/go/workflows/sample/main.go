package main

import (
	"context"
	"flag"
	"fmt"
	"os/user"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/catapult"
	"go.skia.org/infra/pinpoint/go/workflows/internal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"

	pb "go.skia.org/infra/pinpoint/proto/v1"
)

var (
	// Run the following command to portforward Temporal service so the client can connect to it.
	// kubectl port-forward service/temporal --address 0.0.0.0 -n temporal 7233:7233
	hostPort                  = flag.String("hostPort", "localhost:7233", "Host the worker connects to.")
	namespace                 = flag.String("namespace", "default", "The namespace the worker registered to.")
	taskQueue                 = flag.String("taskQueue", "", "Task queue name registered to worker services.")
	commit                    = flag.String("commit", "611b5a084486cd6d99a0dad63f34e320a2ebc2b3", "Git commit hash to build Chrome.")
	triggerBisectFlag         = flag.Bool("bisect", false, "toggle true to trigger bisect workflow")
	triggerCulpritFinderFlag  = flag.Bool("culprit-finder", false, "toggle true to trigger culprit-finder aka sandwich verification workflow")
	triggerSingleCommitFlag   = flag.Bool("single-commit", false, "toggle true to trigger single commit runner workflow")
	triggerPairwiseRunnerFlag = flag.Bool("pairwise-runner", false, "toggle true to trigger pairwise commit runner workflow")
	triggerPairwiseFlag       = flag.Bool("pairwise", false, "toggle true to trigger pairwise workflow")
	triggerBugUpdateFlag      = flag.Bool("update-bug", false, "toggle true to trigger post bug comment workflow")
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

func triggerCulpritFinderWorkflow(c client.Client) (*pb.CulpritFinderExecution, error) {
	// Based off of b/344943386
	ctx := context.Background()
	p := &workflows.CulpritFinderParams{
		Request: &pb.ScheduleCulpritFinderRequest{
			StartGitHash:         "c73e059a2ac54302b2951e4b4f1f7d94d92a707a",
			EndGitHash:           "979c9324d3c6474c15335e676ac7123312d5df82",
			Configuration:        "mac-m2-pro-perf",
			Benchmark:            "system_health.common_desktop",
			Story:                "load:games:bubbles:2020",
			Chart:                "cpu_time_percentage",
			AggregationMethod:    "mean",
			ComparisonMagnitude:  "0.0504",
			ImprovementDirection: "DOWN",
		},
	}

	var cfe *pb.CulpritFinderExecution
	we, err := c.ExecuteWorkflow(ctx, defaultWorkflowOptions(), workflows.CulpritFinderWorkflow, p)
	if err != nil {
		return nil, skerr.Wrapf(err, "Unable to execute workflow")
	}
	sklog.Infof("Started workflow.. WorkflowID: %v RunID: %v", we.GetID(), we.GetRunID())

	if err := we.Get(ctx, &cfe); err != nil {
		return nil, skerr.Wrapf(err, "Unable to get result")
	}
	return cfe, nil
}

func triggerBisectWorkflow(c client.Client) (*pb.BisectExecution, error) {
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
	var be *pb.BisectExecution
	we, err := c.ExecuteWorkflow(ctx, defaultWorkflowOptions(), catapult.CatapultBisectWorkflow, p)
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
	// based off of https://pinpoint-dot-chromeperf.appspot.com/job/1372a174810000
	p := &internal.PairwiseCommitsRunnerParams{
		SingleCommitRunnerParams: internal.SingleCommitRunnerParams{
			PinpointJobID:     "179a34b2be0000",
			BotConfig:         "linux-perf",
			Benchmark:         "v8.browsing_desktop",
			Story:             "browse:tools:docs_scrolling",
			Chart:             "v8:gc:cycle:main_thread:full:atomic",
			AggregationMethod: "mean",
			Iterations:        6,
		},
		Seed:        54321,
		LeftCommit:  common.NewCombinedCommit(&pb.Commit{GitHash: "6c7b055afe2bd688ee3e7d9f035191cdd1bbd0be"}),
		RightCommit: common.NewCombinedCommit(&pb.Commit{GitHash: "1ff117b69e38d05f97872061e256a3e1225f7368"}),
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

func triggerPairwiseWorkflow(c client.Client) (*pb.PairwiseExecution, error) {
	ctx := context.Background()
	p := &workflows.PairwiseParams{
		Request: &pb.SchedulePairwiseRequest{
			StartCommit: &pb.CombinedCommit{
				Main: common.NewChromiumCommit("b4378eb24acedae3c2ad6d7c06dea6a2ddee89b0"),
			},
			EndCommit: &pb.CombinedCommit{
				Main: common.NewChromiumCommit("61adb993e8a46e38caac98dcb80c306391692079"),
			},
			Configuration:        "mac-m2-pro-perf",
			Benchmark:            "v8.browsing_desktop",
			Story:                "browse:search:google:2020",
			Chart:                "memory:chrome:renderer_processes:reported_by_chrome:blink_gc:allocated_objects_size",
			AggregationMethod:    "mean",
			InitialAttemptCount:  "30",
			ImprovementDirection: "DOWN",
		},
	}

	var pe *pb.PairwiseExecution
	we, err := c.ExecuteWorkflow(ctx, defaultWorkflowOptions(), workflows.PairwiseWorkflow, p)
	if err != nil {
		return nil, skerr.Wrapf(err, "Unable to execute workflow")
	}
	sklog.Infof("Started workflow.. WorkflowID: %v RunID: %v", we.GetID(), we.GetRunID())

	if err := we.Get(ctx, &pe); err != nil {
		return nil, skerr.Wrapf(err, "Unable to get result")
	}
	return pe, nil
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
		CombinedCommit:    common.NewCombinedCommit(&pb.Commit{GitHash: *commit}),
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

func triggerBuildChrome(c client.Client) *apipb.CASReference {
	bcp := workflows.BuildParams{
		WorkflowID: "123",
		Commit:     common.NewCombinedCommit(&pb.Commit{GitHash: *commit}),
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
	var result *apipb.CASReference
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
	if *triggerCulpritFinderFlag {
		result, err = triggerCulpritFinderWorkflow(c)
	}
	if *triggerSingleCommitFlag {
		result, err = triggerSingleCommitRunner(c)
	}
	if *triggerPairwiseRunnerFlag {
		result, err = triggerPairwiseRunner(c)
	}
	if *triggerPairwiseFlag {
		result, err = triggerPairwiseWorkflow(c)
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
