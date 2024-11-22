package internal

import (
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.temporal.io/sdk/workflow"
)

type scheduledRun struct {
	childWorkflow  workflow.Future
	scheduledCount uint32
}

// BisectRun tracks current scheduled SingleCommitRun's and merges from other runs.
//
// This is not thread-safe. Schedule and Update usually happens in an I/O nonblocking
// manner where they are invoked in the same thread vis select.
type BisectRun struct {
	CommitRun
	ScheduledRuns []scheduledRun
}

func newBisectRun(cc *common.CombinedCommit) *BisectRun {
	return &BisectRun{
		CommitRun: CommitRun{
			Build: &workflows.Build{
				BuildParams: workflows.BuildParams{
					Commit: cc,
				},
			},
			Runs: make([]*workflows.TestRun, 0, benchmarkRunIterations[1]),
		},
		// typically two runs at most at the same time.
		ScheduledRuns: make([]scheduledRun, 0, 2),
	}
}

func (br *BisectRun) totalScheduledRuns() int32 {
	t := int32(0)
	for _, v := range br.ScheduledRuns {
		t = t + int32(v.scheduledCount)
	}
	return t
}

func (br *BisectRun) totalPendings() []workflow.Future {
	pendings := make([]workflow.Future, len(br.ScheduledRuns))
	for i, v := range br.ScheduledRuns {
		pendings[i] = v.childWorkflow
	}
	return pendings
}

// totalRuns returns the total number of existing runs and pending runs
func (br *BisectRun) totalRuns() int32 {
	return int32(len(br.Runs)) + br.totalScheduledRuns()
}

// scheduleRuns schedules the child workflow to run the given expected number of benchmarks.
//
// This is a non-blocking call. The future that returns can be used to update the runs.
func (br *BisectRun) scheduleRuns(ctx workflow.Context, jobID string, p workflows.BisectParams, newRuns int32) (workflow.ChildWorkflowFuture, error) {
	if newRuns <= 0 {
		// nothing to schedule, safely return
		return nil, nil
	}
	finishedIteration := int32(len(br.CommitRun.Runs))
	cf := workflow.ExecuteChildWorkflow(ctx, workflows.SingleCommitRunner, newRunnerParams(jobID, p, newRuns, br.Build.Commit, finishedIteration))
	br.ScheduledRuns = append(br.ScheduledRuns, scheduledRun{
		childWorkflow:  cf,
		scheduledCount: uint32(newRuns),
	})
	return cf, nil
}

// popRun pops the future for the workflow run if the given workflow is tracked.
func (br *BisectRun) popRun(f workflow.ChildWorkflowFuture) workflow.ChildWorkflowFuture {
	foundIdx := -1
	for i, r := range br.ScheduledRuns {
		if f == r.childWorkflow {
			foundIdx = i
			break
		}
	}

	if foundIdx < 0 {
		return nil
	}
	numRuns := len(br.ScheduledRuns)
	br.ScheduledRuns[foundIdx] = br.ScheduledRuns[numRuns-1]
	br.ScheduledRuns = br.ScheduledRuns[:numRuns-1]

	return f
}

// updateRuns fetches the CommitRun from the future and updates itself.
//
// This is a blocking call. This waits until the future is fulfilled. This can accept nil future
// in which case, it simply ignores.
func (br *BisectRun) updateRuns(ctx workflow.Context, cf workflow.ChildWorkflowFuture) error {
	if cf == nil {
		// nothing to update, safely return
		return nil
	}

	f := br.popRun(cf)
	if f == nil {
		return skerr.Fmt("updating runs (%v) from a different or already updated run (%v)", br.Build.Commit, cf)
	}

	var r *CommitRun
	if err := f.Get(ctx, &r); err != nil {
		return skerr.Wrap(err)
	}

	if br.Build.Commit.Key() != r.Build.Commit.Key() {
		// This shouldn't happen as we only tracks the future for this commit.
		return skerr.Fmt("updating runs (%v) from a different commit(%v)", br.Build.Commit, r.Build.Commit)
	}

	var childWE workflow.Execution
	if err := f.GetChildWorkflowExecution().Get(ctx, &childWE); err != nil {
		// This should never happen as we already get the value.
		return skerr.Wrap(err)
	}

	// no finished runs?
	// if Runs is nil, then there can be an error because the data hasn't been filled out.
	// but if it is empty, then we can safely skip because there can be no data generated.
	if r == nil || r.Runs == nil {
		return skerr.Fmt("no runs were found in the child workflow")
	}

	br.Build = r.Build
	br.Runs = append(br.Runs, r.Runs...)
	return nil
}

// nextRunSize returns the expected number of runs.
//
// nextRunSize return the bigger number of run if two given runs are not equal; otherwise,
// it tries to find the next iteration needed for the comparision to be significant.
//
// If minSampleSize is non-zero, it is used for the initial interation; otherwise, it picks
// up from the predefined number of iterations.
func nextRunSize(br1, br2 *BisectRun, minSampleSize int32) int32 {
	r1 := int32(len(br1.Runs))
	r2 := int32(len(br2.Runs))

	if r1 != r2 {
		return max(r1, r2)
	}
	if r1 == 0 && minSampleSize > 0 {
		return minSampleSize
	}
	for _, iter := range benchmarkRunIterations {
		if iter > r1 {
			return iter
		}
	}
	return getMaxSampleSize()
}
