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

type CommitRange struct {
	Lower  *midpoint.CombinedCommit
	Higher *midpoint.CombinedCommit
}

type CommitMap map[string]*CommitRun

func (cm *CommitMap) get(commit *midpoint.CombinedCommit) (*CommitRun, bool) {
	cr, ok := (*cm)[commit.Key()]
	return cr, ok
}

func (cm *CommitMap) set(commit *midpoint.CombinedCommit, cr *CommitRun) {
	(*cm)[commit.Key()] = cr
}

func (cm *CommitMap) calcNewRuns(lower, higher *midpoint.CombinedCommit) (int32, int32) {
	lRunCount, hRunCount := int32(0), int32(0)
	lr, ok := cm.get(lower)
	if ok {
		lRunCount = int32(len(lr.Runs))
	}
	hr, ok := cm.get(higher)
	if ok {
		hRunCount = int32(len(hr.Runs))
	}
	lRunsToSchedule, hRunsToSchedule := int32(0), int32(0)
	if lRunCount == hRunCount {
		for _, iter := range benchmarkRunIterations {
			if iter > int32(lRunCount) {
				lRunsToSchedule = iter - int32(lRunCount)
				hRunsToSchedule = iter - int32(hRunCount)
				break
			}
		}
	} else if lRunCount > hRunCount {
		// balance number of runs between the two commits
		hRunsToSchedule = lRunCount - hRunCount
	} else {
		lRunsToSchedule = hRunCount - lRunCount
	}
	return lRunsToSchedule, hRunsToSchedule
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

func GetAllValues(ctx context.Context, cr *CommitRun, chart string) ([]float64, error) {
	return cr.AllValues(chart), nil
}

// FindMidCommitActivity is an Activity that finds the middle point of two commits.
//
// TODO(b/326352320): Move this into its own file.
func FindMidCommitActivity(ctx context.Context, cr *CommitRange) (*midpoint.CombinedCommit, error) {
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

	CommitMap := &CommitMap{}
	commitStack := stack.New[*CommitRange]()

	commitStack.Push(&CommitRange{
		Lower:  midpoint.NewCombinedCommit(midpoint.NewCommit(p.Request.StartGitHash), nil),
		Higher: midpoint.NewCombinedCommit(midpoint.NewCommit(p.Request.EndGitHash), nil),
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
		logger.Debug("popped CommitRange: ", *cr)
		lRunsToSchedule, hRunsToSchedule := CommitMap.calcNewRuns(cr.Lower, cr.Higher)
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
			lRun = CommitMap.updateRuns(cr.Lower, lRun)
		} else {
			lRun, _ = CommitMap.get(cr.Lower)
		}

		if hf != nil {
			if err := hf.Get(ctx, &hRun); err != nil {
				return nil, skerr.Wrap(err)
			}
			hRun = CommitMap.updateRuns(cr.Higher, hRun)
		} else {
			hRun, _ = CommitMap.get(cr.Higher)
		}

		result, err := compareRuns(ctx, lRun, hRun, p.Request.Chart, magnitude)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		switch result.Verdict {
		case compare.Unknown:
			commitStack.Push(&CommitRange{
				Lower:  cr.Lower,
				Higher: cr.Higher,
			})
			logger.Debug("pushed CommitRange: ", *commitStack.Peek())
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
			// TODO(sunxiaodi@): When adding commits, the number of runs needed for individual
			// commits need to be included. Without it, the following can happen:
			// Compare A & C, get midpoint B.
			// Push A & B
			// Push B & C
			// Pop B & C
			// Run B 10 times so that it equals number of runs of C
			// Pop A & B
			// Run A & B 10 times because both A & B have 10 runs already.
			// What should happen instead is after A & B pop, neither runs more tests.
			commitStack.Push(&CommitRange{
				Lower:  cr.Lower,
				Higher: mid,
			})
			logger.Debug("pushed CommitRange: ", commitStack.Peek())
			commitStack.Push(&CommitRange{
				Lower:  mid,
				Higher: cr.Higher,
			})
			logger.Debug("pushed CommitRange: ", commitStack.Peek())
		}
	}
	return e, nil
}
