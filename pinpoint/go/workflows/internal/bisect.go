package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/workflows"
	pb "go.skia.org/infra/pinpoint/proto/v1"
	"go.skia.org/infra/temporal/go/common"
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

// CommitRangeTracker stores a commit range as [Lower, Higher].
//
// It stores bisect run as indexes as it needs to be serialized. The indexes
// are stable within the workflow thru bisectRunTracker.
type CommitRangeTracker struct {
	Lower  BisectRunIndex
	Higher BisectRunIndex
}

// CloneWithHigher clones itself with the overriden higher index.
func (t CommitRangeTracker) CloneWithHigher(higher BisectRunIndex) CommitRangeTracker {
	return CommitRangeTracker{
		Lower:  t.Lower,
		Higher: higher,
	}
}

// CloneWithHigher clones itself with the overriden lower index.
func (t CommitRangeTracker) CloneWithLower(lower BisectRunIndex) CommitRangeTracker {
	return CommitRangeTracker{
		Lower:  lower,
		Higher: t.Higher,
	}
}

type CommitValues struct {
	Commit *midpoint.CombinedCommit
	Values []float64
}

// GetAllValuesLocalActivity wraps CommitRun's AllValues as a local activity
func GetAllValuesLocalActivity(ctx context.Context, cr *BisectRun, chart string) (*CommitValues, error) {
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

func compareRuns(ctx workflow.Context, lRun, hRun *BisectRun, chart string, mag float64) (*compare.CompareResults, error) {
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

	// The buffer size should be big enough to process incoming comparisons.
	// The comparision is usually handled right away in the next Select().
	// Also, comparisons.Send happens in a goroutine because the receiving happens
	// in the same thread as Select(), and it needs to be non-blocking.
	comparisons := workflow.NewBufferedChannel(ctx, 100)
	tracker := bisectRunTracker{}

	// pendings keeps track of all the ongoing benchmarks and comparisons.
	// It counts the number of messages in the comparisons channel and the number of
	// all the futures that are in flight. Because we generate new comparison and new
	// futures as we go, this number is dynamic and has to be computed when it is
	// running.
	// There will be two cases when we increase this counter:
	//	1) a new future is added to the selector so we need to wait for its fulfillment;
	//	2) a new comparison is sent to the channel so we need to wait for its process.
	// The counter will be decresed when either of the above is processed.
	pendings := 0

	// selector is used to implement the concurrent bisections in a non-blocking manner.
	// the single commit runner is tracked by the future and it inserts a message into
	// the channel to be processed after the runner completes. The channel receives the
	// message to bisect commits. selector continues to run to process all the messages
	// and futures, until the pendings is decreased to 0, in which case, there is no
	// further messages or future to wait for.
	// selector's callback is run in the same thread so we don't need to worry about
	// race conditions here.
	selector := workflow.NewSelector(ctx)

	// TODO(b/326352379): The errors are not handled here because they are running concurently.
	//	Each error may interrupt the other or may continue the rest.
	selector.AddReceive(comparisons, func(c workflow.ReceiveChannel, more bool) {
		for {
			// cr needs to be created every time in the for-loop as the code below captures this
			// variable in Go-routines.
			var cr CommitRangeTracker
			if !c.ReceiveAsync(&cr) {
				break
			}
			pendings--
			lower, higher := tracker.get(cr.Lower), tracker.get(cr.Higher)
			result, err := compareRuns(ctx, lower, higher, p.Request.Chart, magnitude)

			// The compare fails but we continue to bisect for the remainings.
			// TODO(haowoo@): Failures in the middle may not block the entire bisection, but need to be
			//	logged. We need to diff between the retry-able and non-retry-able failures so that we can
			//	decide if we can continue or not.
			if err != nil {
				logger.Warn(fmt.Sprintf("Failed to compare runs: %v", err))
				continue
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

				lf, hf, err := schedulePairRuns(lower, higher)
				if err != nil {
					logger.Warn(fmt.Sprintf("Failed to schedule more runs (%v)", err))
					break
				}

				pendings++
				futures := append(lower.totalPendings(), higher.totalPendings()...)
				selector.AddFuture(common.NewFutureWithFutures(ctx, futures...), func(f workflow.Future) {
					pendings--
					err := lower.updateRuns(ctx, lf)
					if err != nil {
						logger.Warn(fmt.Sprintf("Failed to update lower runs for (%v): %v", *lower.Commit, err))
					}

					err = higher.updateRuns(ctx, hf)
					if err != nil {
						logger.Warn(fmt.Sprintf("Failed to update higher runs for (%v): %v", *higher.Commit, err))
					}
					workflow.Go(ctx, func(gCtx workflow.Context) {
						comparisons.Send(gCtx, cr)
					})
					pendings++
				})

			case compare.Different:
				var mid *midpoint.CombinedCommit
				if err := workflow.ExecuteActivity(ctx, FindMidCommitActivity, lower.Commit, higher.Commit).Get(ctx, &mid); err != nil {
					logger.Warn(fmt.Sprintf("Failed to find middle commit: %v", err))
					break
				}

				// TODO(b/326352319): Update protos so that pb.BisectionExecution can track multiple culprits.
				if mid.Key() == lower.Commit.Key() {
					// TODO(b/329502712): Append additional info to bisectionExecution
					// such as p-values, average difference
					e.Culprits = append(e.Culprits, higher.Commit.GetMainGitHash())
					break
				}

				midRunIdx, midRun := tracker.newRun(mid)
				mf, err := midRun.scheduleRuns(ctx, e.JobId, *p, nextRunSize(lower, midRun, minSampleSize))
				if err != nil {
					logger.Warn(fmt.Sprintf("Failed to schedule more runs for (%v): %v", *mid, err))
					break
				}

				pendings++
				selector.AddFuture(mf, func(f workflow.Future) {
					pendings--
					err := midRun.updateRuns(ctx, mf)
					if err != nil {
						logger.Warn(fmt.Sprintf("Failed to update runs for (%v): %v", *mid, err))
						return
					}
					workflow.Go(ctx, func(gCtx workflow.Context) {
						comparisons.Send(gCtx, cr.CloneWithHigher(midRunIdx))
						comparisons.Send(gCtx, cr.CloneWithLower(midRunIdx))
					})
					pendings = pendings + 2
				})
			}
		}
	})

	// Schedule the first pair and wait for all to finish before continuing.
	lowerIdx, lower := tracker.newRun(midpoint.NewCombinedCommit(midpoint.NewChromiumCommit(p.Request.StartGitHash)))
	higherIdx, higher := tracker.newRun(midpoint.NewCombinedCommit(midpoint.NewChromiumCommit(p.Request.EndGitHash)))
	lf, hf, err := schedulePairRuns(lower, higher)
	if err != nil {
		// If we are able to schedule in the beginning, there is less chance we will fail in the middle.
		return nil, skerr.Wrapf(err, "failed to schedule initial runs")
	}

	if err := lower.updateRuns(ctx, lf); err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := higher.updateRuns(ctx, hf); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Send the first pair to the channel to process.
	pendings++
	comparisons.SendAsync(CommitRangeTracker{Lower: lowerIdx, Higher: higherIdx})

	// TODO(b/322203189): Store and order the new commits so that the data can be relayed
	// to the UI
	for pendings > 0 {
		selector.Select(ctx)
	}
	return e, nil
}
