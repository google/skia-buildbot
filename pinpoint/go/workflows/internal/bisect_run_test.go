package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/workflows"
	pb "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func generateSingleCommitRuns(hash string, count int) *CommitRun {
	return &CommitRun{
		Build: &workflows.Build{
			BuildParams: workflows.BuildParams{
				Commit: common.NewCombinedCommit(common.NewChromiumCommit(hash)),
			},
		},
		Runs: make([]*workflows.TestRun, count),
	}
}

func makeDefaultBisectParams() workflows.BisectParams {
	return workflows.BisectParams{Request: &pb.ScheduleBisectRequest{}}
}

func TestBisectRun_ScheduleZeroOrLessRun_ReturnNil(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	const jobID = "fake-job-0000-aaaa-9999-0123456789ab"
	bp := makeDefaultBisectParams()
	env.RegisterWorkflowWithOptions(SingleCommitRunner, workflow.RegisterOptions{Name: workflows.SingleCommitRunner})
	env.OnWorkflow(workflows.SingleCommitRunner, mock.Anything, mock.Anything).Return(mockedSingleCommitRun, nil)

	br := newBisectRun(common.NewCombinedCommit(common.NewChromiumCommit("fake-hash")))
	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		f, err := br.scheduleRuns(ctx, jobID, bp, 0)
		require.Nil(t, f)
		require.NoError(t, err)
		require.EqualValues(t, 0, br.totalScheduledRuns())
		require.EqualValues(t, 0, br.totalRuns())
		require.Len(t, br.Runs, 0)

		f, err = br.scheduleRuns(ctx, jobID, bp, -1)
		require.Nil(t, f)
		require.NoError(t, err)
		require.NoError(t, br.updateRuns(ctx, f), "update with nil runs should be no-op")

		require.EqualValues(t, 0, br.totalScheduledRuns())
		require.EqualValues(t, 0, br.totalRuns())
		require.Len(t, br.Runs, 0)
		return nil
	})

	require.NoError(t, env.GetWorkflowError())
}

func TestBisectRun_TotalRuns_WithScheduled_ReturnTotal(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	const jobID = "fake-job-0000-aaaa-9999-0123456789ab"
	bp := makeDefaultBisectParams()
	env.RegisterWorkflowWithOptions(SingleCommitRunner, workflow.RegisterOptions{Name: workflows.SingleCommitRunner})
	env.OnWorkflow(workflows.SingleCommitRunner, mock.Anything, mock.Anything).Return(mockedSingleCommitRun)

	br := newBisectRun(common.NewCombinedCommit(common.NewChromiumCommit("fake-hash")))
	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		require.Len(t, br.Runs, 0)
		require.EqualValues(t, 0, br.totalRuns())

		f, err := br.scheduleRuns(ctx, jobID, bp, 10)
		require.NoError(t, err)
		require.NotNil(t, f)
		require.EqualValues(t, 10, br.totalRuns())

		mf, err := br.scheduleRuns(ctx, jobID, bp, 20)
		require.EqualValues(t, 30, br.totalRuns())
		require.NotNil(t, mf)
		require.NoError(t, err)
		return nil
	})

	require.NoError(t, env.GetWorkflowError())
}

func TestBisectRun_ScheduleAndUpdate(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	const jobID = "fake-job-0000-aaaa-9999-0123456789ab"
	bp := makeDefaultBisectParams()

	env.RegisterWorkflowWithOptions(SingleCommitRunner, workflow.RegisterOptions{Name: workflows.SingleCommitRunner})
	env.OnWorkflow(workflows.SingleCommitRunner, mock.Anything, mock.Anything).Return(mockedSingleCommitRun, nil)

	br := newBisectRun(common.NewCombinedCommit(common.NewChromiumCommit("fake-hash")))
	const (
		firstRuns  = 10
		secondRuns = 5
		thirdRuns  = 15
	)
	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		firstFuture, err := br.scheduleRuns(ctx, jobID, bp, firstRuns)
		require.NotNil(t, firstFuture)
		require.NoError(t, err)

		secondFuture, err := br.scheduleRuns(ctx, jobID, bp, secondRuns)
		require.EqualValues(t, firstRuns+secondRuns, br.totalRuns())
		require.EqualValues(t, firstRuns+secondRuns, br.totalScheduledRuns())
		require.NotNil(t, secondFuture)
		require.NoError(t, err)

		require.NoError(t, br.updateRuns(ctx, secondFuture))
		require.Len(t, br.Runs, secondRuns)
		require.EqualValues(t, firstRuns+secondRuns, br.totalRuns())
		require.EqualValues(t, firstRuns, br.totalScheduledRuns())

		require.NoError(t, br.updateRuns(ctx, firstFuture))
		require.EqualValues(t, 0, br.totalScheduledRuns())
		require.EqualValues(t, firstRuns+secondRuns, br.totalRuns())
		require.Len(t, br.Runs, firstRuns+secondRuns)

		thirdFuture, err := br.scheduleRuns(ctx, jobID, bp, thirdRuns)
		require.EqualValues(t, thirdRuns, br.totalScheduledRuns())
		require.EqualValues(t, firstRuns+secondRuns+thirdRuns, br.totalRuns())
		require.NotNil(t, thirdFuture)
		require.NoError(t, err)
		require.NoError(t, br.updateRuns(ctx, thirdFuture))
		require.Len(t, br.Runs, firstRuns+secondRuns+thirdRuns)
		require.EqualValues(t, firstRuns+secondRuns+thirdRuns, br.totalRuns())
		return nil
	})

	require.NoError(t, env.GetWorkflowError())
	require.Len(t, br.Runs, firstRuns+secondRuns+thirdRuns)
}

func TestBisectRun_WithIncompleteRun_ReturnRuns(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	const hash1 = "fake-ffcaaab85ecf1a896da1d635aeca929edbe"
	const jobID = "fake-job-0000-aaaa-9999-0123456789ab"
	const (
		partialComplete = 8
		schedule        = 10
	)
	bp := makeDefaultBisectParams()

	env.RegisterWorkflowWithOptions(SingleCommitRunner, workflow.RegisterOptions{Name: workflows.SingleCommitRunner})
	env.OnWorkflow(workflows.SingleCommitRunner, mock.Anything, mock.Anything).Return(generateSingleCommitRuns(hash1, partialComplete), nil)

	br := newBisectRun(common.NewCombinedCommit(common.NewChromiumCommit(hash1)))
	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		f1, err := br.scheduleRuns(ctx, jobID, bp, schedule)
		require.NoError(t, err)

		f2, err := br.scheduleRuns(ctx, jobID, bp, schedule)
		require.NotNil(t, f2)
		require.NoError(t, err)

		require.NoError(t, br.updateRuns(ctx, f1))
		require.EqualValues(t, schedule, br.totalScheduledRuns(), "partial completed runs should remove the initial scheduled runs.")
		require.EqualValues(t, schedule+partialComplete, br.totalRuns(), "should only update with partially completed runs.")

		require.NoError(t, br.updateRuns(ctx, f2))
		require.EqualValues(t, 0, br.totalScheduledRuns())
		require.EqualValues(t, partialComplete+partialComplete, br.totalRuns(), "should only update with partially completed runs.")
		require.Len(t, br.Runs, partialComplete+partialComplete)

		return nil
	})

	require.NoError(t, env.GetWorkflowError())
	require.Len(t, br.Runs, partialComplete+partialComplete)
}

func TestBisectRun_UnmatchedCommit_ShouldError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	const jobID = "fake-job-0000-aaaa-9999-0123456789ab"
	bp := makeDefaultBisectParams()

	env.RegisterWorkflowWithOptions(SingleCommitRunner, workflow.RegisterOptions{Name: workflows.SingleCommitRunner})
	env.OnWorkflow(workflows.SingleCommitRunner, mock.Anything, mock.Anything).Return(mockedSingleCommitRun, nil)

	const hash1 = "fake-ffcaaab85ecf1a896da1d635aeca929edbe"
	const other = "other-e0c1a4e8cae6103adbd4c2feacbf0c99bb"

	br1 := newBisectRun(common.NewCombinedCommit(common.NewChromiumCommit(hash1)))
	br2 := newBisectRun(common.NewCombinedCommit(common.NewChromiumCommit(other)))
	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		f1, err := br1.scheduleRuns(ctx, jobID, bp, 10)
		require.NoError(t, err)

		require.ErrorContains(t, br2.updateRuns(ctx, f1), "different")

		require.EqualValues(t, 10, br1.totalScheduledRuns())
		require.NoError(t, br1.updateRuns(ctx, f1))
		return nil
	})

	require.NoError(t, env.GetWorkflowError())
	require.Len(t, br1.Runs, 10)
}

func TestBisectRun_WithAlreadyUpdated_ShouldError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	const jobID = "fake-job-0000-aaaa-9999-0123456789ab"
	bp := makeDefaultBisectParams()

	env.RegisterWorkflowWithOptions(SingleCommitRunner, workflow.RegisterOptions{Name: workflows.SingleCommitRunner})
	env.OnWorkflow(workflows.SingleCommitRunner, mock.Anything, mock.Anything).Return(mockedSingleCommitRun, nil)

	const hash1 = "fake-ffcaaab85ecf1a896da1d635aeca929edbe"
	br := newBisectRun(common.NewCombinedCommit(common.NewChromiumCommit(hash1)))
	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		f1, err := br.scheduleRuns(ctx, jobID, bp, 10)
		require.EqualValues(t, 10, br.totalScheduledRuns())
		require.NoError(t, err)
		require.NoError(t, br.updateRuns(ctx, f1))

		require.EqualValues(t, 0, br.totalScheduledRuns())
		require.ErrorContains(t, br.updateRuns(ctx, f1), "already updated")
		return nil
	})

	require.NoError(t, env.GetWorkflowError())
	require.Len(t, br.Runs, 10)
}

func TestNextRunSize_BothEqual_MoreRunsForBoth(t *testing.T) {
	const hash1 = "fake-ffcaaab85ecf1a896da1d635aeca929edbe"
	test := func(name string, runs, minSampleSize, expected int) {
		t.Run(name, func(t *testing.T) {
			br := &BisectRun{CommitRun: *generateSingleCommitRuns(hash1, runs)}
			assert.EqualValues(t, expected, nextRunSize(br, br, int32(minSampleSize)))
		})
	}
	// see benchmarkRunIterations for how these run iterations are calculated
	test("0 runs each should expect 10 runs", 0, 10, 10)
	test("0 runs each with minSampleSize 20 should expect 20 runs", 0, 20, 20)
	test("15 runs each with minSampleSize 15 should expect 20 runs", 15, 15, 20)
	test("10 runs each should expect 20 runs", 10, 10, 20)
	test("20 runs each should expect 40 runs", 20, 10, 40)
}

func TestNextRunSize_LowerCommitMoreRuns_OnlySchedulesMoreRunsForHigherCommit(t *testing.T) {
	const hash1 = "fake1-1111111111111196da1d635aeca929edbe"
	const hash2 = "fake2-2222222222222296da1d635aeca929edbe"
	br1 := &BisectRun{CommitRun: *generateSingleCommitRuns(hash1, 0)}
	br2 := &BisectRun{CommitRun: *generateSingleCommitRuns(hash2, 10)}
	require.EqualValues(t, 10, nextRunSize(br1, br2, 0))
	require.EqualValues(t, 10, nextRunSize(br2, br1, 0))
}

func TestNextRunSize_WithNonZero_ReturnMinSampleSize(t *testing.T) {
	const hash1 = "fake1-fcaaab85ecf1a896da1d635aeca929edbe"
	br := &BisectRun{CommitRun: *generateSingleCommitRuns(hash1, 0)}
	require.EqualValues(t, benchmarkRunIterations[0], nextRunSize(br, br, 0))
	require.EqualValues(t, 10, nextRunSize(br, br, 10))
	require.EqualValues(t, 20, nextRunSize(br, br, 20))
}
