package internal

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/zyedidia/generic/stack"
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

	benchmarkRunIterations = []int32{10, 20, 40, 80, 160}
)

// CommitRangeTracker stores a commit range as [Lower, Higher]
// and tracks the expected sample size required for comparison.
type CommitRangeTracker struct {
	Lower              *midpoint.CombinedCommit
	Higher             *midpoint.CombinedCommit
	ExpectedSampleSize int32
}

type CommitMap map[string]*CommitRun

func (cm *CommitMap) get(commit *midpoint.CombinedCommit) (*CommitRun, bool) {
	cr, ok := (*cm)[commit.Key()]
	return cr, ok
}

func (cm *CommitMap) set(commit *midpoint.CombinedCommit, cr *CommitRun) {
	(*cm)[commit.Key()] = cr
}

func (cm *CommitMap) updateRuns(commit *midpoint.CombinedCommit, newRun *CommitRun) *CommitRun {
	cr, ok := cm.get(commit)
	if !ok {
		cr = newRun
	} else {
		cr.Runs = append(cr.Runs, newRun.Runs...)
	}
	cm.set(commit, cr)
	return cr
}

func (cm *CommitMap) calcSampleSize(lower, higher *midpoint.CombinedCommit) int32 {
	lRunCount, hRunCount := int32(0), int32(0)
	lr, ok := cm.get(lower)
	if ok {
		lRunCount = int32(len(lr.Runs))
	}
	hr, ok := cm.get(higher)
	if ok {
		hRunCount = int32(len(hr.Runs))
	}
	if lRunCount == hRunCount {
		for _, iter := range benchmarkRunIterations {
			if iter > lRunCount {
				return iter
			}
		}
	}
	// balance number of runs between the two commits
	return max(lRunCount, hRunCount)
}

func (cr *CommitRangeTracker) calcNewRuns(cm *CommitMap) (int32, int32, error) {
	if cr.ExpectedSampleSize == 0 {
		return 0, 0, skerr.Fmt("ExpectedSampleSize is 0 for commits %v and %v", *cr.Lower, *cr.Higher)
	}
	lRunCount, hRunCount := int32(0), int32(0)
	lr, ok := cm.get(cr.Lower)
	if ok {
		lRunCount = int32(len(lr.Runs))
	}
	hr, ok := cm.get(cr.Higher)
	if ok {
		hRunCount = int32(len(hr.Runs))
	}
	lRunsToSchedule, hRunsToSchedule := cr.ExpectedSampleSize-lRunCount, cr.ExpectedSampleSize-hRunCount
	if lRunsToSchedule < 0 || hRunsToSchedule < 0 {
		return 0, 0, skerr.Fmt("Number of runs to schedule is less than 0 for CommitRangeTracker %v", *cr)
	}
	return lRunsToSchedule, hRunsToSchedule, nil
}

func GetAllValues(ctx context.Context, cr *CommitRun, chart string) ([]float64, error) {
	return cr.AllValues(chart), nil
}

// FindMidCommitActivity is an Activity that finds the middle point of two commits.
//
// TODO(b/326352320): Move this into its own file.
func FindMidCommitActivity(ctx context.Context, cr *CommitRangeTracker) (*midpoint.CombinedCommit, error) {
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrapf(err, "Problem setting up default token source")
	}
	c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).With2xxOnly().Client()
	m, err := midpoint.New(ctx, c).FindMidCommit(ctx, cr.Lower.Main, cr.Higher.Main)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return midpoint.NewCombinedCommit(m, nil), nil
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

func compareRuns(ctx workflow.Context, lRun, hRun *CommitRun, chart string, mag float64) (*compare.CompareResults, error) {
	var lValues, hValues []float64
	if err := workflow.ExecuteLocalActivity(ctx, GetAllValues, lRun, chart).Get(ctx, &lValues); err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := workflow.ExecuteLocalActivity(ctx, GetAllValues, hRun, chart).Get(ctx, &hValues); err != nil {
		return nil, skerr.Wrap(err)
	}

	var result *compare.CompareResults
	if err := workflow.ExecuteActivity(ctx, ComparePerformanceActivity, lValues, hValues, mag).Get(ctx, &result); err != nil {
		return nil, skerr.Wrap(err)
	}

	return result, nil
}

// BisectWorkflow is a Workflow definition that takes a range of git hashes and finds the culprit.
func BisectWorkflow(ctx workflow.Context, p *workflows.BisectParams) (*pb.BisectExecution, error) {
	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, activityOptions)
	ctx = workflow.WithLocalActivityOptions(ctx, localActivityOptions)

	logger := workflow.GetLogger(ctx)

	// TODO(sunxiaodi@) Add the job ID to the bisection request
	// so that tasks can be recycled to assist with debugging
	// This task also requires edits to single commit runner.
	jobID := uuid.New().String()
	e := &pb.BisectExecution{
		JobId: jobID,
	}

	cm := &CommitMap{}
	commitStack := stack.New[*CommitRangeTracker]()

	commitStack.Push(&CommitRangeTracker{
		Lower:              midpoint.NewCombinedCommit(midpoint.NewCommit(p.Request.StartGitHash), nil),
		Higher:             midpoint.NewCombinedCommit(midpoint.NewCommit(p.Request.EndGitHash), nil),
		ExpectedSampleSize: benchmarkRunIterations[0],
	})
	// TODO(sunxiaodi@) allow for optional comparison magnitude. If nil
	// assume the normalized magnitude = 1.0
	magnitude, err := strconv.ParseFloat(p.Request.ComparisonMagnitude, 64)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// TODO(b/322203189): Store and order the new commits so that the data can be relayed
	// to the UI
	for commitStack.Size() > 0 {
		logger.Debug("current commitStack: ", *commitStack)
		cr := commitStack.Pop()
		logger.Debug("popped CommitRangeTracker: ", *cr)
		lRunsToSchedule, hRunsToSchedule, err := cr.calcNewRuns(cm)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		var lf, hf workflow.ChildWorkflowFuture = nil, nil
		if lRunsToSchedule > 0 {
			lf = workflow.ExecuteChildWorkflow(ctx, workflows.SingleCommitRunner, newRunnerParams(jobID, p, lRunsToSchedule, cr.Lower))
		}
		if hRunsToSchedule > 0 {
			hf = workflow.ExecuteChildWorkflow(ctx, workflows.SingleCommitRunner, newRunnerParams(jobID, p, hRunsToSchedule, cr.Higher))
		}

		var lRun, hRun *CommitRun
		if lf != nil {
			if err := lf.Get(ctx, &lRun); err != nil {
				return nil, skerr.Wrap(err)
			}
			lRun = cm.updateRuns(cr.Lower, lRun)
		} else {
			lRun, _ = cm.get(cr.Lower)
		}

		if hf != nil {
			if err := hf.Get(ctx, &hRun); err != nil {
				return nil, skerr.Wrap(err)
			}
			hRun = cm.updateRuns(cr.Higher, hRun)
		} else {
			hRun, _ = cm.get(cr.Higher)
		}

		result, err := compareRuns(ctx, lRun, hRun, p.Request.Chart, magnitude)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		switch result.Verdict {
		case compare.Unknown:
			commitStack.Push(&CommitRangeTracker{
				Lower:              cr.Lower,
				Higher:             cr.Higher,
				ExpectedSampleSize: cm.calcSampleSize(cr.Lower, cr.Higher),
			})
			logger.Debug("pushed CommitRangeTracker: ", *commitStack.Peek())
		case compare.Different:
			// TODO(b/326352320): If the middle point has a different repo, it means that it looks into
			//	the autoroll and there are changes in DEPS. We need to construct a CombinedCommit so it
			//	can currently build with modified deps.
			var mid *midpoint.CombinedCommit
			if err := workflow.ExecuteActivity(ctx, FindMidCommitActivity, cr).Get(ctx, &mid); err != nil {
				return nil, skerr.Wrap(err)
			}
			// TODO(b/326352319): Update protos so that pb.BisectionExecution can track multiple culprits.
			// TODO(b/327019543): Create midpoint equality function to compare two CombinedCommits
			if mid.GetMainGitHash() == cr.Lower.GetMainGitHash() {
				e.Commit = cr.Higher.GetMainGitHash()
				break
			}
			commitStack.Push(&CommitRangeTracker{
				Lower:              cr.Lower,
				Higher:             mid,
				ExpectedSampleSize: cm.calcSampleSize(cr.Lower, cr.Higher),
			})
			logger.Debug("pushed CommitRangeTracker: ", commitStack.Peek())
			commitStack.Push(&CommitRangeTracker{
				Lower:              mid,
				Higher:             cr.Higher,
				ExpectedSampleSize: cm.calcSampleSize(cr.Lower, cr.Higher),
			})
			logger.Debug("pushed CommitRangeTracker: ", commitStack.Peek())
		}
	}
	return e, nil
}
