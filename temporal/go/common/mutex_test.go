package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// sampleWorkflowWithMutex is used to unit test the mutex workflow
func sampleWorkflowWithMutex(ctx workflow.Context, resourceID string) error {
	currentWorkflowID := workflow.GetInfo(ctx).WorkflowExecution.ID
	m := NewMutex(currentWorkflowID, "TestUseCase")
	unlockFunc, err := m.Lock(ctx, resourceID, 10*time.Minute)
	if err != nil {
		return err
	}
	_ = workflow.Sleep(ctx, 10*time.Second)
	_ = unlockFunc()
	return nil
}

// mockMutexLock stubs mutex.Lock call
func mockMutexLock(env *testsuite.TestWorkflowEnvironment, resourceID string, mockError error) {
	execution := &workflow.Execution{ID: "mockID", RunID: "mockRunID"}
	env.OnActivity(signalWithStartMutexWorkflowActivity, mock.Anything, mock.Anything, resourceID, mock.Anything, mock.Anything).Return(execution, mockError)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(AcquireLockSignalName, "mockReleaseLockChannelName")
	}, time.Millisecond*0)
	if mockError == nil {
		env.OnSignalExternalWorkflow(mock.Anything, mock.Anything, execution.RunID, mock.Anything, mock.Anything).Return(nil)
	}
}

func TestSampleWorkflow_Success(t *testing.T) {
	mockResourceID := "mockResourceID"

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	mockMutexLock(env, mockResourceID, nil)
	env.ExecuteWorkflow(sampleWorkflowWithMutex, mockResourceID)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}

func TestMutexWorkflow_Success(t *testing.T) {
	mockNamespace := "mockNamespace"
	mockResourceID := "mockResourceID"
	mockUnlockTimeout := 10 * time.Minute
	mockSenderWorkflowID := "mockSenderWorkflowID"

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(RequestLockSignalName, mockSenderWorkflowID)
	}, time.Millisecond*0)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("unlock-event-mockSenderWorkflowID", "releaseLock")
	}, time.Millisecond*0)
	env.OnSignalExternalWorkflow(mock.Anything, mockSenderWorkflowID, "", AcquireLockSignalName, mock.Anything).Return(nil)

	env.ExecuteWorkflow(mutexWorkflow, mockNamespace, mockResourceID, mockUnlockTimeout)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

func TestMutexWorkflow_TimeoutSuccess(t *testing.T) {
	mockNamespace := "mockNamespace"
	mockResourceID := "mockResourceID"
	mockUnlockTimeout := 10 * time.Minute
	mockSenderWorkflowID := "mockSenderWorkflowID"

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(RequestLockSignalName, mockSenderWorkflowID)
	}, time.Millisecond*0)
	env.OnSignalExternalWorkflow(mock.Anything, mockSenderWorkflowID, "", AcquireLockSignalName, mock.Anything).Return(nil)

	env.ExecuteWorkflow(mutexWorkflow, mockNamespace, mockResourceID, mockUnlockTimeout)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}
