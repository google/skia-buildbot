package main

import (
	"context"
	"flag"
	"fmt"
	"os/user"
	"strconv"
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
	enumspb "go.temporal.io/api/enums/v1"
)

var (
	// Run the following command to portforward Temporal service so the client can connect to it.
	// kubectl port-forward service/temporal --address 0.0.0.0 -n temporal 7233:7233
	hostPort                  = flag.String("hostPort", "localhost:7233", "Host the worker connects to.")
	namespace                 = flag.String("namespace", "default", "The namespace the worker registered to.")
	taskQueue                 = flag.String("taskQueue", "", "Task queue name registered to worker services.")
	commit                    = flag.String("commit", "611b5a084486cd6d99a0dad63f34e320a2ebc2b3", "Git commit hash to build Chrome.")
	startGitHash              = flag.String("start-git-hash", "c73e059a2ac54302b2951e4b4f1f7d94d92a707a", "Start git commit hash for bisect.")
	endGitHash                = flag.String("end-git-hash", "979c9324d3c6474c15335e676ac7123312d5df82", "End git commit hash for bisect.")
	configuration             = flag.String("configuration", "mac-m2-pro-perf", "Bot configuration to use.")
	benchmark                 = flag.String("benchmark", "speedometer3.crossbench", "Benchmark to run.")
	story                     = flag.String("story", "default", "Story to run.")
	chart                     = flag.String("chart", "Score", "Chart (metric or test result) to collect.")
	aggregationMethod         = flag.String("aggregation-method", "mean", "Aggregation method to use for bisect.")
	comparisonMagnitude       = flag.String("comparison-magnitude", "0.1", "Comparison magnitude for bisect.")
	improvementDirection      = flag.String("improvement-direction", "UP", "Improvement direction for bisect (UP or DOWN)")
	jobId                     = flag.String("job-id", "123", "Pinpoint job ID to use.")
	iterations                = flag.Int("iterations", 2, "Number of iterations to run the story.")
	extraArg                  = flag.String("extra-arg", "", "Extra argument to pass to test.")
	triggerBisectFlag         = flag.Bool("bisect", false, "toggle true to trigger bisect workflow")
	triggerCulpritFinderFlag  = flag.Bool("culprit-finder", false, "toggle true to trigger culprit-finder aka sandwich verification workflow")
	triggerSingleCommitFlag   = flag.Bool("single-commit", false, "toggle true to trigger single commit runner workflow")
	triggerPairwiseRunnerFlag = flag.Bool("pairwise-runner", false, "toggle true to trigger pairwise commit runner workflow")
	triggerPairwiseFlag       = flag.Bool("pairwise", false, "toggle true to trigger pairwise workflow")
	triggerBugUpdateFlag      = flag.Bool("update-bug", false, "toggle true to trigger post bug comment workflow")
	triggerQueryPairwiseFlag  = flag.Bool("query-pairwise", false, "toggle true to trigger querying of pairwise flows")
	triggerCbbRunnerFlag      = flag.Bool("cbb-runner", false, "toggle true to trigger CBB runner workflow")
	triggerCbbNewReleaseFlag  = flag.Bool("cbb-new-release", false, "toggle true to trigger CbbNewReleaseDetectorWorkflow")
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
			StartGitHash:         *startGitHash,
			EndGitHash:           *endGitHash,
			Configuration:        *configuration,
			Benchmark:            *benchmark,
			Story:                *story,
			Chart:                *chart,
			AggregationMethod:    *aggregationMethod,
			ComparisonMagnitude:  *comparisonMagnitude,
			ImprovementDirection: *improvementDirection,
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
			StartGitHash:         *startGitHash,
			EndGitHash:           *endGitHash,
			Configuration:        *configuration,
			Benchmark:            *benchmark,
			Story:                *story,
			Chart:                *chart,
			ComparisonMagnitude:  *comparisonMagnitude,
			AggregationMethod:    *aggregationMethod,
			Project:              "chromium",
			ImprovementDirection: *improvementDirection,
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
			PinpointJobID:     *jobId,
			BotConfig:         *configuration,
			Benchmark:         *benchmark,
			Story:             *story,
			Chart:             *chart,
			AggregationMethod: *aggregationMethod,
			Iterations:        int32(*iterations),
		},
		Seed:        54321,
		LeftCommit:  common.NewCombinedCommit(&pb.Commit{GitHash: *startGitHash}),
		RightCommit: common.NewCombinedCommit(&pb.Commit{GitHash: *endGitHash}),
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

// based off of https://pinpoint-dot-chromeperf.appspot.com/job/2e79457b-4d19-4e3b-9553-7baf1fd9a0e1
func triggerPairwiseWorkflow(c client.Client) (*pb.PairwiseExecution, error) {
	ctx := context.Background()
	p := &workflows.PairwiseParams{
		Request: &pb.SchedulePairwiseRequest{
			StartCommit: &pb.CombinedCommit{
				Main: common.NewChromiumCommit(*startGitHash),
			},
			EndCommit: &pb.CombinedCommit{
				Main: common.NewChromiumCommit(*endGitHash),
			},
			Configuration:        *configuration,
			Benchmark:            *benchmark,
			Story:                *story,
			Chart:                *chart,
			AggregationMethod:    *aggregationMethod,
			InitialAttemptCount:  strconv.Itoa(*iterations),
			ImprovementDirection: *improvementDirection,
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
		PinpointJobID:     *jobId,
		BotConfig:         *configuration,
		Benchmark:         *benchmark,
		Story:             *story,
		Chart:             *chart,
		AggregationMethod: *aggregationMethod,
		CombinedCommit:    common.NewCombinedCommit(&pb.Commit{GitHash: *commit}),
		Iterations:        int32(*iterations),
	}
	if *extraArg != "" {
		p.ExtraArgs = []string{*extraArg}
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
		WorkflowID: *jobId,
		Commit:     common.NewCombinedCommit(&pb.Commit{GitHash: *commit}),
		Device:     *configuration,
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

func triggerQueryPairwise(c client.Client) (*pb.QueryPairwiseResponse, error) {
	ctx := context.Background()
	workflow_id := "45fe60b9-668e-44a9-9991-678221ba264d"

	resp, err := c.DescribeWorkflowExecution(ctx, workflow_id, "")
	if err != nil {
		return nil, skerr.Wrapf(err, "Unable to describe workflow")
	}

	workflowStatus := resp.GetWorkflowExecutionInfo().GetStatus()
	fmt.Print(workflowStatus)
	var pairwiseExecution pb.PairwiseExecution

	switch workflowStatus {
	case enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED:

		workflowRun := c.GetWorkflow(ctx, workflow_id, "")

		errGet := workflowRun.Get(ctx, &pairwiseExecution)
		if errGet != nil {
			return nil, skerr.Wrapf(errGet, "Pairwise workflow completed, but failed to get results")
		}

		return &pb.QueryPairwiseResponse{
			Status:    pb.PairwiseJobStatus_PAIRWISE_JOB_STATUS_COMPLETED,
			Execution: &pairwiseExecution,
		}, nil

	case enumspb.WORKFLOW_EXECUTION_STATUS_FAILED,
		enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT,
		enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED:

		return &pb.QueryPairwiseResponse{
			Status:    pb.PairwiseJobStatus_PAIRWISE_JOB_STATUS_FAILED,
			Execution: &pairwiseExecution,
		}, nil

	case enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED:
		return &pb.QueryPairwiseResponse{
			Status:    pb.PairwiseJobStatus_PAIRWISE_JOB_STATUS_CANCELED,
			Execution: &pairwiseExecution,
		}, nil

	case enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING,
		enumspb.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW:
		return &pb.QueryPairwiseResponse{
			Status:    pb.PairwiseJobStatus_PAIRWISE_JOB_STATUS_RUNNING,
			Execution: &pairwiseExecution,
		}, nil
	}
	return nil, nil
}

func triggerCbbRunner(c client.Client) (*internal.CommitRun, error) {
	ctx := context.Background()
	p := &internal.CbbRunnerParams{
		BotConfig: "mac-m3-pro-perf-cbb",
		Commit:    common.NewCombinedCommit(common.NewChromiumCommit(*commit)),
		Browser:   "chrome",
		Channel:   "stable",
		// For testing purposes, this is currently hardcoded to run 2 iterations of Speedometer 3.
		// A mechanism will be added in the future to allow specifying benchmarks and iterations
		// on the command line.
		Benchmarks: []internal.BenchmarkRunConfig{{Benchmark: "speedometer3", Iterations: 2}},
	}
	var cr *internal.CommitRun
	we, err := c.ExecuteWorkflow(ctx, defaultWorkflowOptions(), workflows.CbbRunner, p)
	if err != nil {
		return nil, skerr.Wrapf(err, "Unable to execute workflow")
	}
	sklog.Infof("Started workflow.. WorkflowID: %v RunID: %v", we.GetID(), we.GetRunID())

	if err := we.Get(ctx, &cr); err != nil {
		return nil, skerr.Wrapf(err, "Unable to get result")
	}
	return cr, nil
}

func triggerCbbNewReleaseDetector(c client.Client) (bool, error) {
	ctx := context.Background()

	var success bool
	we, err := c.ExecuteWorkflow(ctx, defaultWorkflowOptions(), workflows.CbbNewReleaseDetector)
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
	if *triggerQueryPairwiseFlag {
		result, err = triggerQueryPairwise(c)
	}
	if *triggerCbbRunnerFlag {
		result, err = triggerCbbRunner(c)
	}

	if *triggerCbbNewReleaseFlag {
		result, err = triggerCbbNewReleaseDetector(c)
	}

	if err != nil {
		sklog.Errorf("Workflow failed:", err)
		return
	}
	sklog.Infof("Workflow result: %v", spew.Sdump(result))
}
