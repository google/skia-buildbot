package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// Test cases here use simulated workflow environment to advance timers,
// different timers are set up such that the futures are fulfilled at
// different point of workflow progress, and then be able to test
// different states of futures.

func Test_NewFuture_WaitOnAllFutures(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		f1 := workflow.NewTimer(ctx, 10*time.Second)
		f2 := workflow.NewTimer(ctx, 20*time.Second)
		f := NewFutureWithFutures(ctx, f1, f2)
		require.NoError(t, f.Get(ctx, nil))
		require.True(t, f1.IsReady())
		require.True(t, f2.IsReady())
		return nil
	})

	require.NoError(t, env.GetWorkflowError())
}

func Test_NewFuture_NilFutures_NoOp(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		f1 := workflow.NewTimer(ctx, 10*time.Second)
		f := NewFutureWithFutures(ctx, nil, f1)
		require.NoError(t, f.Get(ctx, nil))
		require.True(t, f1.IsReady())

		f = NewFutureWithFutures(ctx, nil, nil)
		require.NoError(t, f.Get(ctx, nil))

		f = NewFutureWithFutures(ctx)
		require.NoError(t, f.Get(ctx, nil))
		return nil
	})

	require.NoError(t, env.GetWorkflowError())
}

func Test_NewFuture_NestedFuture(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		// t10
		// t20 -> f1
		//        t1 -> f2
		t1 := workflow.NewTimer(ctx, time.Second)
		t10 := workflow.NewTimer(ctx, 10*time.Second)
		t20 := workflow.NewTimer(ctx, 20*time.Second)
		f1 := NewFutureWithFutures(ctx, t10, t20)

		f2 := NewFutureWithFutures(ctx, t1, f1)

		require.NoError(t, f2.Get(ctx, nil))
		require.True(t, f1.IsReady())
		require.True(t, t1.IsReady())
		require.True(t, t10.IsReady())
		require.True(t, t20.IsReady())
		return nil
	})

	require.NoError(t, env.GetWorkflowError())
}

func Test_NewFuture_WithFulfilledFuture_ShouldReturn(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		t10 := workflow.NewTimer(ctx, 10*time.Second)
		require.NoError(t, t10.Get(ctx, nil))
		t20 := workflow.NewTimer(ctx, 20*time.Second)
		f1 := NewFutureWithFutures(ctx, t10, t20)

		require.NoError(t, f1.Get(ctx, nil))
		require.True(t, t10.IsReady())
		require.True(t, t20.IsReady())
		return nil
	})

	require.NoError(t, env.GetWorkflowError())
}

func Test_NewFuture_WithDuplicates_ShouldReturn(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		t10 := workflow.NewTimer(ctx, 10*time.Second)
		f1 := NewFutureWithFutures(ctx, t10, t10, t10)

		require.NoError(t, f1.Get(ctx, nil))
		require.True(t, t10.IsReady())
		return nil
	})

	require.NoError(t, env.GetWorkflowError())
}

func Test_NewFuture_Cancel(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterDelayedCallback(func() {
		env.CancelWorkflow()
	}, 25*time.Second)

	env.ExecuteWorkflow(func(ctx workflow.Context, waitTime time.Duration) error {
		t10 := workflow.NewTimer(ctx, 10*time.Second)
		longWait := workflow.NewTimer(ctx, waitTime)
		f1 := NewFutureWithFutures(ctx, longWait, t10)

		// Wait for all futures are done until the context is cancelled.
		err := f1.Get(ctx, nil)
		require.ErrorContains(t, err, "canceled")

		// cancelled futures should be in ready state but with errors when Get()
		require.True(t, f1.IsReady())
		require.True(t, longWait.IsReady())
		require.True(t, t10.IsReady())

		// completed future shouldn't contain errors
		require.NoError(t, t10.Get(ctx, nil))
		require.ErrorContains(t, longWait.Get(ctx, nil), "canceled")

		// populate errors to be captured by the caller.
		return err
	}, time.Hour)

	require.Error(t, env.GetWorkflowError())
}
