package internal

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.skia.org/infra/go/skerr"
	pinpoint_common "go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/temporal/go/common"
	"go.temporal.io/sdk/workflow"

	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
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

func (t *bisectRunTracker) newRun(cc *pinpoint_common.CombinedCommit) (BisectRunIndex, *BisectRun) {
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

func newRunnerParams(jobID string, p workflows.BisectParams, it int32, cc *pinpoint_common.CombinedCommit, finishedIteration int32) *SingleCommitRunnerParams {
	return &SingleCommitRunnerParams{
		CombinedCommit:    cc,
		PinpointJobID:     jobID,
		BotConfig:         p.Request.Configuration,
		Benchmark:         p.Request.Benchmark,
		Story:             p.Request.Story,
		Chart:             p.Request.Chart,
		AggregationMethod: p.Request.AggregationMethod,
		Iterations:        it,
		FinishedIteration: finishedIteration,
		BotIds:            p.BotIds,
	}
}

// BisectExecution is a mirror of pinpoint_proto.BisectExecution, with additional raw data.
//
// When this BisectExecution embeds pinpoint_proto.BisectExecution, it fails to store
// CommitPairValues and BisectRuns, which are used to curate the information for Catapult
// Pinpoint.
// TODO(b/322203189) - This is a temporary solution for backwards compatibilty to the
// Catapult UI and should be removed when the catapult package is deprecated.
type BisectExecution struct {
	JobId string
	// TODO(b/322203189): replace Culprits with DetailedCulprits. This field is used by the
	// catapult bisect UI write.
	Culprits []*pinpoint_proto.CombinedCommit
	// DetailedCulprits stores a list of culprits and the commit prior to the culprit.
	// Without this field, culprit verification would not know the commit prior to any culprit
	// and be unable to verify the correct pair of commits.
	DetailedCulprits []*pinpoint_proto.Culprit
	CreateTime       *timestamppb.Timestamp
	Comparisons      []*CombinedResults
	RunData          []*BisectRun
}

// BisectWorkflow is a Workflow definition that takes a range of git hashes and finds the culprit.
func BisectWorkflow(ctx workflow.Context, p *workflows.BisectParams) (be *BisectExecution, wkErr error) {
	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
	ctx = workflow.WithLocalActivityOptions(ctx, localActivityOptions)

	logger := workflow.GetLogger(ctx)

	jobID := uuid.New().String()
	if p.JobID != "" {
		jobID = p.JobID
	}
	be = &BisectExecution{
		JobId:            jobID,
		Culprits:         []*pinpoint_proto.CombinedCommit{},
		DetailedCulprits: []*pinpoint_proto.Culprit{},
		CreateTime:       timestamppb.Now(),
		Comparisons:      []*CombinedResults{},
	}

	mh := workflow.GetMetricsHandler(ctx).WithTags(map[string]string{
		"job_id":    jobID,
		"user":      p.Request.User,
		"benchmark": p.Request.Benchmark,
		"config":    p.Request.Configuration,
		"story":     p.Request.Story,
	})
	mh.Counter("bisect_start_count").Inc(1)
	wkStartTime := time.Now().UnixNano()
	defer func() {
		duration := time.Now().UnixNano() - wkStartTime
		mh.Timer("bisect_duration").Record(time.Duration(duration))
		mh.Counter("bisect_complete_count").Inc(1)

		if wkErr != nil {
			mh.Counter("bisect_err_count").Inc(1)
		}
		if errors.Is(ctx.Err(), workflow.ErrCanceled) || errors.Is(ctx.Err(), workflow.ErrDeadlineExceeded) {
			mh.Counter("bisect_timeout_count").Inc(1)
		}

		if be != nil && len(be.Culprits) > 0 {
			mh.Counter("bisect_found_culprit_count").Inc(1)
		}
	}()

	// Find the available bot list
	if err := workflow.ExecuteActivity(ctx, FindAvailableBotsActivity, p.Request.Configuration, time.Now().UnixNano()).Get(ctx, &p.BotIds); err != nil {
		return nil, skerr.Wrapf(err, "failed to find available bots")
	}

	magnitude := p.GetMagnitude()
	improvementDir := p.GetImprovementDirection()

	// minSampleSize is the minimum number of benchmark runs for each attempt
	// Default is 10.
	minSampleSize := p.GetInitialAttempt()
	if minSampleSize < benchmarkRunIterations[0] {
		logger.Warn("Initial attempt count %d is less than the default %d. Setting to default.", minSampleSize, benchmarkRunIterations[0])
		minSampleSize = benchmarkRunIterations[0]
	}

	// schedulePairRuns is a helper function to schedule new benchmark runs from two BisectRun.
	// It captures common local variable and attempts to make the code cleaner in the for-loop below.
	schedulePairRuns := func(lower, higher *BisectRun) (workflow.ChildWorkflowFuture, workflow.ChildWorkflowFuture, error) {
		expected := nextRunSize(lower, higher, minSampleSize)
		lf, err := lower.scheduleRuns(ctx, jobID, *p, expected-lower.totalRuns())
		if err != nil {
			logger.Warn("Failed to schedule more runs.", "commit", lower.Build.Commit, "error", err)
			return nil, nil, skerr.Wrap(err)
		}

		hf, err := higher.scheduleRuns(ctx, jobID, *p, expected-higher.totalRuns())
		if err != nil {
			logger.Warn("Failed to schedule more runs.", "commit", higher.Build.Commit, "error", err)
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
			compareResult, err := compareRuns(ctx, lower, higher, p.Request.Chart, magnitude, improvementDir)
			// The compare fails but we continue to bisect for the remainings.
			// TODO(sunxiaodi@): Revisit compare runs error handling. compare.ComparePerformance
			// and compare.CompareFunctional should not return error but are written to return error.
			// GetAllValues also does not return error, so that means there are no errors passed around
			// these functions.
			if err != nil {
				logger.Warn(fmt.Sprintf("Failed to compare runs: %v", err))
				continue
			}
			be.Comparisons = append(be.Comparisons, compareResult)

			result := compareResult.Result
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
						logger.Warn(fmt.Sprintf("Failed to update lower runs for (%v): %v", lower.Build.Commit, err))
					}

					err = higher.updateRuns(ctx, hf)
					if err != nil {
						logger.Warn(fmt.Sprintf("Failed to update higher runs for (%v): %v", higher.Build.Commit, err))
					}
					workflow.Go(ctx, func(gCtx workflow.Context) {
						comparisons.Send(gCtx, cr)
					})
					pendings++
				})

			case compare.Different:
				var mid *pinpoint_common.CombinedCommit
				if err := workflow.ExecuteActivity(ctx, FindMidCommitActivity, lower.Build.Commit, higher.Build.Commit).Get(ctx, &mid); err != nil {
					logger.Warn(fmt.Sprintf("Failed to find middle commit: %v", err))
					break
				}

				var equal bool
				if err := workflow.ExecuteActivity(ctx, CheckCombinedCommitEqualActivity, lower.Build.Commit, mid).Get(ctx, &equal); err != nil {
					logger.Warn("Failed to determine equality between two combined commits")
					break
				}
				if equal {
					// TODO(b/329502712): Append additional info to bisectionExecution
					// such as p-values, average difference
					culprit := (*pinpoint_proto.CombinedCommit)(higher.Build.Commit)
					be.Culprits = append(be.Culprits, culprit)
					be.DetailedCulprits = append(be.DetailedCulprits,
						&pinpoint_proto.Culprit{
							Prior:   (*pinpoint_proto.CombinedCommit)(lower.Build.Commit),
							Culprit: culprit,
						},
					)
					break
				}

				midRunIdx, midRun := tracker.newRun(mid)
				mf, err := midRun.scheduleRuns(ctx, be.JobId, *p, nextRunSize(lower, midRun, minSampleSize))
				if err != nil {
					logger.Warn(fmt.Sprintf("Failed to schedule more runs for (%v): %v", mid, err))
					break
				}

				pendings++
				selector.AddFuture(mf, func(f workflow.Future) {
					pendings--
					err := midRun.updateRuns(ctx, mf)
					if err != nil {
						logger.Warn(fmt.Sprintf("Failed to update runs for (%v): %v", mid, err))
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
	lowerIdx, lower := tracker.newRun(pinpoint_common.NewCombinedCommit(pinpoint_common.NewChromiumCommit(p.Request.StartGitHash)))
	higherIdx, higher := tracker.newRun(pinpoint_common.NewCombinedCommit(pinpoint_common.NewChromiumCommit(p.Request.EndGitHash)))
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

	be.RunData = make([]*BisectRun, len(tracker.runs))
	copy(be.RunData, tracker.runs)

	return be, nil
}
