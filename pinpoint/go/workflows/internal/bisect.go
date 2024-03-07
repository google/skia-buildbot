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

type commitRange struct {
	lower  *midpoint.CombinedCommit
	higher *midpoint.CombinedCommit
}

type commitMap map[string]*CommitRun

func (cm *commitMap) get(commit *midpoint.CombinedCommit) (*CommitRun, bool) {
	cr, ok := (*cm)[commit.Key()]
	return cr, ok
}

func (cm *commitMap) set(commit *midpoint.CombinedCommit, cr *CommitRun) {
	(*cm)[commit.Key()] = cr
}

func (cm *commitMap) calcNewRuns(lower, higher *midpoint.CombinedCommit) (int32, int32) {
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

func (cm *commitMap) updateRuns(commit *midpoint.CombinedCommit, newRun *CommitRun) *CommitRun {
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

// FindMidCommit is an Activity that finds the middle point of two commits.
//
// TODO(b/326352320): Move this into its own file.
func FindMidCommit(ctx context.Context, cr commitRange) (*midpoint.CombinedCommit, error) {
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrapf(err, "Problem setting up default token source")
	}
	c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).With2xxOnly().Client()
	m, err := midpoint.New(ctx, c).FindMidCommit(ctx, cr.lower.Main, cr.higher.Main)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return midpoint.NewCombinedCommit(m), nil
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

	jobID := uuid.New().String()
	e := &pb.BisectExecution{
		JobId: jobID,
	}

	commitMap := &commitMap{}
	commitStack := stack.New[*commitRange]()

	commitStack.Push(&commitRange{
		lower:  midpoint.NewCombinedCommit(midpoint.NewCommit(p.Request.StartGitHash), nil),
		higher: midpoint.NewCombinedCommit(midpoint.NewCommit(p.Request.EndGitHash), nil),
	})
	magnitude, err := strconv.ParseFloat(p.Request.ComparisonMagnitude, 64)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// TODO(b/322203189): Store and order the new commits so that the data can be relayed
	// to the UI
	for commitStack.Size() > 0 {
		logger.Debug("current commitStack: ", commitStack)
		cr := commitStack.Pop()
		logger.Debug("popped commitRange: ", cr)
		lRunsToSchedule, hRunsToSchedule := commitMap.calcNewRuns(cr.lower, cr.higher)
		var lf, hf workflow.ChildWorkflowFuture = nil, nil
		if lRunsToSchedule > 0 {
			lf = workflow.ExecuteChildWorkflow(ctx, workflows.SingleCommitRunner, newRunnerParams(jobID, p, lRunsToSchedule, cr.lower))
		}
		if hRunsToSchedule > 0 {
			hf = workflow.ExecuteChildWorkflow(ctx, workflows.SingleCommitRunner, newRunnerParams(jobID, p, hRunsToSchedule, cr.higher))
		}

		var lRun, hRun *CommitRun
		if err := lf.Get(ctx, &lRun); err != nil {
			return nil, skerr.Wrap(err)
		}
		if err := hf.Get(ctx, &hRun); err != nil {
			return nil, skerr.Wrap(err)
		}
		hRun = commitMap.updateRuns(cr.higher, hRun)
		lRun = commitMap.updateRuns(cr.lower, lRun)

		result, err := compareRuns(ctx, lRun, hRun, p.Request.Chart, magnitude)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		switch result.Verdict {
		case compare.Unknown:
			commitStack.Push(&commitRange{
				lower:  cr.lower,
				higher: cr.higher,
			})
			logger.Debug("pushed commitRange: ", commitStack.Peek())
		case compare.Different:
			// TODO(b/326352320): If the middle point has a different repo, it means that it looks into
			//	the autoroll and there are changes in DEPS. We need to construct a CombinedCommit so it
			//	can currently build with modified deps.
			var mid *midpoint.CombinedCommit
			if err := workflow.ExecuteActivity(ctx, FindMidCommit, cr.lower, cr.higher).Get(ctx, &mid); err != nil {
				return nil, skerr.Wrap(err)
			}
			// TODO(b/326352319): Update protos so that pb.BisectionExecution can track multiple culprits.
			// TODO(b/327019543): Create midpoint equality function to compare two CombinedCommits
			if mid.GetMainGitHash() == cr.lower.GetMainGitHash() {
				e.Commit = cr.higher.GetMainGitHash()
				break
			}
			commitStack.Push(&commitRange{
				lower:  cr.lower,
				higher: mid,
			})
			logger.Debug("pushed commitRange: ", commitStack.Peek())
			commitStack.Push(&commitRange{
				lower:  mid,
				higher: cr.higher,
			})
			logger.Debug("pushed commitRange: ", commitStack.Peek())
		}
	}
	return e, nil
}
