package internal

import (
	"context"
	"strconv"

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

var benchmarkRunIterations = [...]int32{10, 20, 40, 80, 160}

func getMaxSampleSize() int32 {
	return benchmarkRunIterations[len(benchmarkRunIterations)-1]
}

// bisectRunTracker stores all the running bisect runs.
//
// It keeps track of all the runs by indexes. The BisectRun will be updated from different
// future fulfillment. One can wait for a bisect run that's already triggered by a different
// bisection. This usually happens when the mid commit is computed and the result will be
// used for comparisions from both sides. This can also happen when one comparision requires
// more runs, and part of them is already triggered by another comparision.
type BisectRunIndex int
type bisectRunTracker struct {
	runs []*BisectRun
}

func (t *bisectRunTracker) newRun(cc *midpoint.CombinedCommit) (BisectRunIndex, *BisectRun) {
	r := newBisectRun(cc)
	t.runs = append(t.runs, r)
	return BisectRunIndex(len(t.runs) - 1), r
}

func (t bisectRunTracker) get(r BisectRunIndex) *BisectRun {
	if r < 0 || int(r) >= len(t.runs) {
		return nil
	}
	return t.runs[int(r)]
}

// CommitRangeTracker stores a commit range as [Lower, Higher] and expected sample size.
//
// It stores bisect run as indexes as it needs to be serialized. The indexes
// are stable within the workflow thru bisectRunTracker.
type CommitRangeTracker struct {
	Lower              BisectRunIndex
	Higher             BisectRunIndex
	ExpectedSampleSize int32
}

func (t CommitRangeTracker) CloneWithHigher(higher BisectRunIndex) CommitRangeTracker {
	return CommitRangeTracker{
		Lower:              t.Lower,
		Higher:             higher,
		ExpectedSampleSize: t.ExpectedSampleSize,
	}
}

func (t CommitRangeTracker) CloneWithLower(lower BisectRunIndex) CommitRangeTracker {
	return CommitRangeTracker{
		Lower:              lower,
		Higher:             t.Higher,
		ExpectedSampleSize: t.ExpectedSampleSize,
	}
}

func (t CommitRangeTracker) CloneWithExpectedSampleSize(expectedSampleSize int32) CommitRangeTracker {
	return CommitRangeTracker{
		Lower:              t.Lower,
		Higher:             t.Higher,
		ExpectedSampleSize: expectedSampleSize,
	}
}

// newTrackerWithHashes returns a CommitRangeTracker containing two new BisectRun.
//
// This is a helper function to make the code slightly cleaner, this function will eventually retire.
func (t *bisectRunTracker) newTrackerWithHashes(lowerHash, higherHash string, expectedSize int32) CommitRangeTracker {
	lower, _ := t.newRun(midpoint.NewCombinedCommit(midpoint.NewChromiumCommit(lowerHash)))
	higher, _ := t.newRun(midpoint.NewCombinedCommit(midpoint.NewChromiumCommit(higherHash)))
	return CommitRangeTracker{
		Lower:              lower,
		Higher:             higher,
		ExpectedSampleSize: expectedSize,
	}
}

type CommitValues struct {
	Commit *midpoint.CombinedCommit
	Values []float64
}

// GetAllValuesLocalActivity wraps CommitRun's AllValues as a local activity
func GetAllValuesLocalActivity(ctx context.Context, cr *CommitRun, chart string) (*CommitValues, error) {
	return &CommitValues{cr.Commit, cr.AllValues(chart)}, nil
}

// FindMidCommitActivity is an Activity that finds the middle point of two commits.
//
// TODO(b/326352320): Move this into its own file.
func FindMidCommitActivity(ctx context.Context, lower, higher *midpoint.CombinedCommit) (*midpoint.CombinedCommit, error) {
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrapf(err, "Problem setting up default token source")
	}
	c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).With2xxOnly().Client()
	m, err := midpoint.New(ctx, c).FindMidCombinedCommit(ctx, lower, higher)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return m, nil
}

func newRunnerParams(jobID string, p workflows.BisectParams, it int32, cc *midpoint.CombinedCommit) *SingleCommitRunnerParams {
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
	var lValues, hValues *CommitValues
	if err := workflow.ExecuteLocalActivity(ctx, GetAllValuesLocalActivity, lRun, chart).Get(ctx, &lValues); err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := workflow.ExecuteLocalActivity(ctx, GetAllValuesLocalActivity, hRun, chart).Get(ctx, &hValues); err != nil {
		return nil, skerr.Wrap(err)
	}

	var result *compare.CompareResults
	if err := workflow.ExecuteActivity(ctx, ComparePerformanceActivity, lValues.Values, hValues.Values, mag).Get(ctx, &result); err != nil {
		return nil, skerr.Wrap(err)
	}

	return result, nil
}

// BisectWorkflow is a Workflow definition that takes a range of git hashes and finds the culprit.
func BisectWorkflow(ctx workflow.Context, p *workflows.BisectParams) (*pb.BisectExecution, error) {
	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
	ctx = workflow.WithLocalActivityOptions(ctx, localActivityOptions)

	logger := workflow.GetLogger(ctx)

	// TODO(sunxiaodi@) Add the job ID to the bisection request
	// so that tasks can be recycled to assist with debugging
	// This task also requires edits to single commit runner.
	jobID := uuid.New().String()
	e := &pb.BisectExecution{
		JobId:    jobID,
		Culprits: []string{},
	}

	mh := workflow.GetMetricsHandler(ctx).WithTags(map[string]string{
		"job_id":    jobID,
		"benchmark": p.Request.Benchmark,
		"config":    p.Request.Configuration,
		"story":     p.Request.Story,
	})
	mh.Counter("bisect_count").Inc(1)
	defer func() {
		if len(e.Culprits) > 0 {
			mh.Counter("bisect_found_culprit_count").Inc(1)
		}
	}()

	// TODO(sunxiaodi@): migrate these default params to service/service_impl/validate
	// compare.ComparePerformance will assume the normalizedMagnitude is 1.0
	// when the rawMagnitude is 0.0
	magnitude := float64(0.0)
	if p.Request.ComparisonMagnitude != "" {
		var err error
		magnitude, err = strconv.ParseFloat(p.Request.ComparisonMagnitude, 64)
		// TODO(sunxiaodi@): Can use default comparison magnitude rather than throw error
		if err != nil {
			return nil, skerr.Wrapf(err, "comparison magnitude %s cannot be converted to float", p.Request.ComparisonMagnitude)
		}
	}

	// minSampleSize is the minimum number of benchmark runs for each attempt
	// Default is 10.
	minSampleSize := benchmarkRunIterations[0]
	if p.Request.InitialAttemptCount != "" {
		ss, err := strconv.ParseInt(p.Request.InitialAttemptCount, 10, 32)
		if err != nil {
			return nil, skerr.Wrapf(err, "initial attempt count %s cannot be converted to int", p.Request.ComparisonMagnitude)
		}
		if ss < 10 {
			logger.Warn("Initial attempt count %s is less than the default 10. Setting minSampleSize to 10.", p.Request.InitialAttemptCount)
		} else {
			minSampleSize = int32(ss)
		}
	}

	// schedulePairRuns is a helper function to schedule new benchmark runs from two BisectRun.
	// It captures common local variable and attempts to make the code cleaner in the for-loop below.
	schedulePairRuns := func(lower, higher *BisectRun) (workflow.ChildWorkflowFuture, workflow.ChildWorkflowFuture, error) {
		expected := nextRunSize(lower, higher, minSampleSize)
		lf, err := lower.scheduleRuns(ctx, jobID, *p, expected-lower.totalRuns())
		if err != nil {
			logger.Warn("Failed to schedule more runs.", "commit", *lower.Commit, "error", err)
			return nil, nil, skerr.Wrap(err)
		}

		hf, err := higher.scheduleRuns(ctx, jobID, *p, expected-higher.totalRuns())
		if err != nil {
			logger.Warn("Failed to schedule more runs.", "commit", *higher.Commit, "error", err)
			return nil, nil, err
		}

		return lf, hf, nil
	}

	tracker := bisectRunTracker{}
	commitStack := stack.New[CommitRangeTracker]()
	commitStack.Push(tracker.newTrackerWithHashes(p.Request.StartGitHash, p.Request.EndGitHash, minSampleSize))

	// TODO(b/322203189): Store and order the new commits so that the data can be relayed
	// to the UI
	for commitStack.Size() > 0 {
		cr := commitStack.Pop()

		lower, higher := tracker.get(cr.Lower), tracker.get(cr.Higher)
		lf, hf, err := schedulePairRuns(lower, higher)
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		if err := lower.updateRuns(ctx, lf); err != nil {
			return nil, skerr.Wrap(err)
		}

		if err := higher.updateRuns(ctx, hf); err != nil {
			return nil, skerr.Wrap(err)
		}

		result, err := compareRuns(ctx, &higher.CommitRun, &lower.CommitRun, p.Request.Chart, magnitude)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		switch result.Verdict {
		case compare.Unknown:
			// Only push to stack if less than getMaxSampleSize(). At normalized magnitudes
			// < 0.4, it is possible to get to the max sample size and still reach an unknown
			// verdict. Running more samples is too expensive. Instead, assume the two samples
			// are the statistically similar.
			// assumes that cr.Lower and cr.Higher will have the same number of runs
			if len(lower.Runs) >= int(getMaxSampleSize()) {
				// TODO(haowoo@): add metric to measure this occurrence
				logger.Warn("reached unknown verdict with p-value %d and sample size of %d", result.PValue, len(lower.Runs))
				break
			}
			commitStack.Push(cr.CloneWithExpectedSampleSize(nextRunSize(lower, higher, minSampleSize)))

		case compare.Different:
			// TODO(b/326352320): If the middle point has a different repo, it means that it looks into
			//	the autoroll and there are changes in DEPS. We need to construct a CombinedCommit so it
			//	can currently build with modified deps.
			var mid *midpoint.CombinedCommit
			if err := workflow.ExecuteActivity(ctx, FindMidCommitActivity, lower.Commit, higher.Commit).Get(ctx, &mid); err != nil {
				return nil, skerr.Wrap(err)
			}

			// TODO(b/326352319): Update protos so that pb.BisectionExecution can track multiple culprits.
			if mid.Key() == lower.Commit.Key() {
				// TODO(b/329502712): Append additional info to bisectionExecution
				// such as p-values, average difference
				e.Culprits = append(e.Culprits, higher.Commit.GetMainGitHash())
				break
			}

			midIdx, midRun := tracker.newRun(mid)

			// Both higher and lower should contain the same number runs so we would expect the same
			// number of runs for both sides (lower, mid) and (mid, higher)
			cr = cr.CloneWithExpectedSampleSize(nextRunSize(lower, midRun, minSampleSize))
			commitStack.Push(cr.CloneWithHigher(midIdx))
			commitStack.Push(cr.CloneWithLower(midIdx))
		}
	}
	return e, nil
}
