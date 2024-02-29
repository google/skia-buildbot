package internal

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/workflows"
	pb "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/workflow"
	"golang.org/x/oauth2/google"
)

var (
	localActivityOptions = workflow.LocalActivityOptions{
		ScheduleToCloseTimeout: 15 * time.Second,
	}
	activityOptions = workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
	}
	childWorkflowOptions = workflow.ChildWorkflowOptions{
		// 4 hours of compile time + 8 hours of test run time
		WorkflowExecutionTimeout: 12 * time.Hour,
	}

	benchmarkRunIterations = []int32{10, 20, 40, 60, 120}
)

func GetAllValues(cr *CommitRun, chart string) ([]float64, error) {
	return cr.AllValues(chart), nil
}

// FindMidCommit is an Acitivty that finds the middle point of two commits.
//
// TODO(b/326352320): Move this into its own file.
func FindMidCommit(ctx context.Context, left, right *midpoint.Commit) (*midpoint.Commit, error) {
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrapf(err, "Problem setting up default token source")
	}
	c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).With2xxOnly().Client()
	return midpoint.New(ctx, c).FindMidCommit(ctx, left, right)
}

func newRunnerParams(jobID string, p *workflows.BisectParams, it int32, cc *midpoint.CombinedCommit) *SingleCommitRunnerParams {
	return &SingleCommitRunnerParams{
		CombinedCommit:    cc,
		PinpointJobID:     jobID,
		BotConfig:         p.Request.Configuration,
		Benchmark:         p.Request.Benchmark,
		Story:             p.Request.Story,
		Chart:             p.Request.Chart,
		AggregationMethod: p.Request.AggregationMethod,
		Iterations:        it,
	}
}

// BisectWorkflow is a Workflow definition that takes a range of git hashes and finds the culprit.
func BisectWorkflow(ctx workflow.Context, p *workflows.BisectParams) (*pb.BisectExecution, error) {
	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, activityOptions)
	ctx = workflow.WithLocalActivityOptions(ctx, localActivityOptions)

	jobID := uuid.New().String()
	e := &pb.BisectExecution{
		JobId: jobID,
	}

	lower := midpoint.NewCommit(p.Request.StartGitHash)
	higher := midpoint.NewCommit(p.Request.EndGitHash)
	magnitude, err := strconv.ParseFloat(p.Request.ComparisonMagnitude, 64)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	var lRun, hRun *CommitRun
	lf := workflow.ExecuteChildWorkflow(ctx, workflows.SingleCommitRunner, newRunnerParams(jobID, p, benchmarkRunIterations[0], &midpoint.CombinedCommit{Main: lower}))
	hf := workflow.ExecuteChildWorkflow(ctx, workflows.SingleCommitRunner, newRunnerParams(jobID, p, benchmarkRunIterations[0], &midpoint.CombinedCommit{Main: higher}))
	if err := lf.Get(ctx, &lRun); err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := hf.Get(ctx, &hRun); err != nil {
		return nil, skerr.Wrap(err)
	}

	var lValues, hValues []float64
	if err := workflow.ExecuteLocalActivity(ctx, GetAllValues, lRun, p.Request.Chart).Get(ctx, &lValues); err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := workflow.ExecuteLocalActivity(ctx, GetAllValues, hRun, p.Request.Chart).Get(ctx, &hValues); err != nil {
		return nil, skerr.Wrap(err)
	}

	var result *compare.CompareResults
	if err := workflow.ExecuteActivity(ctx, ComparePerformanceActivity, lValues, hValues, magnitude).Get(ctx, &result); err != nil {
		return nil, skerr.Wrap(err)
	}

	if result.Verdict == compare.Same {
		return e, nil
	}

	// TODO(b/327019543): Currently, it only does one binary search and tries to find the first
	//	culprit. For a given range, it possibly contains multiple culprits. So we should search on
	//	both sides and find all the possible culprits.
	for {
		var mid *midpoint.Commit
		// TODO(b/326352320): If the middle point has a different repo, it means that it looks into
		//	the autoroll and there are changes in DEPS. We need to construct a CombinedCommit so it
		//	can currently build with modified deps.
		if err := workflow.ExecuteActivity(ctx, FindMidCommit, lower, higher).Get(ctx, &mid); err != nil {
			return nil, skerr.Wrap(err)
		}

		// lower and higher is adjacent, and they are different, then the culprit is
		// the higher commit
		if mid.GitHash == lower.GitHash && mid.RepositoryUrl == lower.RepositoryUrl {
			e.Commit = higher.GitHash
			break
		}

		var mRun *CommitRun
		mf := workflow.ExecuteChildWorkflow(ctx, workflows.SingleCommitRunner, newRunnerParams(jobID, p, benchmarkRunIterations[0], &midpoint.CombinedCommit{Main: mid}))
		if err := mf.Get(ctx, &mRun); err != nil {
			return nil, skerr.Wrap(err)
		}

		var lValues, mValues []float64
		if err := workflow.ExecuteLocalActivity(ctx, GetAllValues, lRun, p.Request.Chart).Get(ctx, &lValues); err != nil {
			return nil, skerr.Wrap(err)
		}
		if err := workflow.ExecuteLocalActivity(ctx, GetAllValues, mRun, p.Request.Chart).Get(ctx, &mValues); err != nil {
			return nil, skerr.Wrap(err)
		}
		if err := workflow.ExecuteActivity(ctx, ComparePerformanceActivity, lValues, mValues, magnitude).Get(ctx, &result); err != nil {
			return nil, skerr.Wrap(err)
		}

		if result.Verdict == compare.Different {
			higher = mid
			hRun = mRun
			continue
		}

		var hValues []float64
		if err := workflow.ExecuteLocalActivity(ctx, GetAllValues, hRun, p.Request.Chart).Get(ctx, &hValues); err != nil {
			return nil, skerr.Wrap(err)
		}
		if err := workflow.ExecuteActivity(ctx, ComparePerformanceActivity, mValues, hValues, magnitude).Get(ctx, &result); err != nil {
			return nil, skerr.Wrap(err)
		}
		if result.Verdict == compare.Different {
			lower = mid
			lRun = mRun
			continue
		}

		// They are both same, no culprit found
		break
	}

	return e, nil
}
