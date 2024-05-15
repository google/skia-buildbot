package common

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	// AcquireLockSignalName signal channel name for lock acquisition
	AcquireLockSignalName = "acquire-lock-event"
	// RequestLockSignalName channel name for request lock
	RequestLockSignalName = "request-lock-event"
	// MutexLockWorkflowName is the name of the temporal workflow mutexWorkflow
	mutexLockWorkflowName = "perf.mutex-lock"
)

type contextKey struct{}

var clientContextKey = &contextKey{}

type UnlockFunc func() error

// Mutex is a struct used by temporal to lock resources to allow workflows
// to utilize a resource sequentially. Mutex workflows hold onto swarming bots
// to ensure that they execute pairwise tasks in immediate sequential order
// without being interrupted by swarming tasks from other jobs.
// Mutex should only be used by run_benchmark or workflows that trigger
// run_benchmark
type Mutex struct {
	// currentWorkflowID is the temporal workflow that needs the resource
	currentWorkflowID string
	// lockNamespace is a namespace for the mutex workflow
	lockNamespace string
}

// NewMutex initializes mutex.
// currentWorkflowID is the temporal workflow that needs the resource.
// lockNamespace is a namespace for the mutex workflow.
func NewMutex(currentWorkflowID string, lockNamespace string) *Mutex {
	return &Mutex{
		currentWorkflowID: currentWorkflowID,
		lockNamespace:     lockNamespace,
	}
}

// RegisterTemporal registers the temporal workflows and activities
// that are needed to run Lock()
func RegisterTemporal(w worker.Worker) *worker.Worker {
	w.RegisterActivity(signalWithStartMutexWorkflowActivity)
	w.RegisterWorkflowWithOptions(mutexWorkflow, workflow.RegisterOptions{Name: mutexLockWorkflowName})
	return &w
}

// Lock applies the lock on resourceID by triggering a temporal workflow
// that creates a workflow with the resourceID as part of the workflowID.
// All external workflows outside of the mutex namespace that require
// resourceID will have to wait for the resource until the lock is released
// via UnlockFunc.
// The resourceID can be any arbitrary string.
// Example usage:
// func SampleWorkflowWithMutex(resourceID):
//
//	m := NewMutex("workflow", "namespace")
//	unlockFunc, err := m.Lock(ctx, resourceID, 10*time.Minute)
//	< perform workflow actions >
//	_ = unlockFunc() // this releases the lock on the resource
//	< remaining workflow actions >
//
// run a bunch of workflows in parallel, but because each workflow requires
// the same resource, they will not start until the lock is released
// func WorkflowThatNeedsSpecificResource:
//
//	rc := workflow.NewBufferedChannel(ctx, numChildren)
//	ec := workflow.NewBufferedChannel(ctx, numChildren)
//	wg := workflow.NewWaitGroup(ctx)
//	wg.Add(numChildren)
//	for numChildren {
//		workflow.Go(ctx, func(gCtx workflow.Context) {
//			defer wg.Done
//			if err := workflow.ExecuteChildWorkflow(gCtx, SampleWorkflowWithMutex, "bot-123").Get(); err != nil {
//				ec.Send(gCtx, err)
//				return
//			}
//			rc.Send(gCtx, status)
//		})
//
// Also, if another workflow starts and calls SampleWorkflowWithMutex("bot-123")
// while `WorkflowThatNeedsSpecificResource` is still running, it, will also wait until
// the lock on "bot-123" is released via unlockFunc().
func (s *Mutex) Lock(ctx workflow.Context, resourceID string, unlockTimeout time.Duration) (UnlockFunc, error) {

	activityCtx := workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
		ScheduleToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    5,
		},
	})

	var execution workflow.Execution
	err := workflow.ExecuteLocalActivity(activityCtx, signalWithStartMutexWorkflowActivity, s.lockNamespace, resourceID, s.currentWorkflowID, unlockTimeout).Get(ctx, &execution)
	if err != nil {
		return nil, err
	}

	var releaseLockChannelName string
	workflow.GetSignalChannel(ctx, AcquireLockSignalName).
		Receive(ctx, &releaseLockChannelName)

	unlockFunc := func() error {
		return workflow.SignalExternalWorkflow(ctx, execution.ID, execution.RunID,
			releaseLockChannelName, "releaseLock").Get(ctx, nil)
	}
	return unlockFunc, nil
}

// mutexWorkflow is a temporal workflow that verifies if the lock if available and
// will poll the lock's availability until it is released or times out (unlockTimeout)
// It is meant to only be triggered by signalWithStartMutexWorkflowActivity.
// Note the resourceID is appended to the workflowID when it is triggered by
// signalWithStartMutexWorkflowActivity, but is not used in this workflow.
// Temporal will still track the workflow inputs and outputs, so including the resourceID
// is for debugging convenience on the temporal UI.
func mutexWorkflow(ctx workflow.Context, namespace string, resourceID string, unlockTimeout time.Duration) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("started", "currentWorkflowID", workflow.GetInfo(ctx).WorkflowExecution.ID)
	var ack string
	requestLockCh := workflow.GetSignalChannel(ctx, RequestLockSignalName)
	for {
		var senderWorkflowID string
		// Check if the lock is used by another workflow
		// If not, break out of the loop
		if !requestLockCh.ReceiveAsync(&senderWorkflowID) {
			logger.Info("no more signals")
			break
		}
		var releaseLockChannelName string
		_ = workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} {
			return generateUnlockChannelName(senderWorkflowID)
		}).Get(&releaseLockChannelName)
		logger.Info("generated release lock channel name", "releaseLockChannelName", releaseLockChannelName)
		// Send release lock channel name back to another workflow holding the lock, via senderWorkflowID,
		// so that it can release the lock using release lock channel name
		// .Get(ctx, nil) blocks until the signal is sent.
		if err := workflow.SignalExternalWorkflow(ctx, senderWorkflowID, "", AcquireLockSignalName, releaseLockChannelName).Get(ctx, nil); err != nil {
			// If the external workflow at senderWorkflowID is closed (terminated/canceled/timeouted/completed/etc)
			// this would return error. Instead of failing the mutex workflow, release the lock immediately.
			// Mutex workflow failing would lead to all workflows that have sent requestLock will be waiting.
			logger.Info("SignalExternalWorkflow error", "Error", err)
			continue
		}
		logger.Info("signaled external workflow")
		selector := workflow.NewSelector(ctx)
		selector.AddFuture(workflow.NewTimer(ctx, unlockTimeout), func(f workflow.Future) {
			logger.Info("unlockTimeout exceeded")
		})
		selector.AddReceive(workflow.GetSignalChannel(ctx, releaseLockChannelName), func(c workflow.ReceiveChannel, more bool) {
			c.Receive(ctx, &ack)
			logger.Info("release signal received")
		})
		selector.Select(ctx)
	}
	return nil
}

// signalWithStartMutexWorkflowActivity triggers the MutexWorkflow using
// the namespace and resourceID as a unique workflow to prevent other
// workflows from accessing the same resourceID.
func signalWithStartMutexWorkflowActivity(ctx context.Context, namespace string, resourceID string, senderWorkflowID string, unlockTimeout time.Duration) (*workflow.Execution, error) {

	c := ctx.Value(clientContextKey).(client.Client)
	workflowID := fmt.Sprintf(
		"%s:%s:%s",
		"mutex",
		namespace,
		resourceID,
	)
	workflowOptions := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "mutex",
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    5,
		},
	}
	wr, err := c.SignalWithStartWorkflow(ctx, workflowID, RequestLockSignalName, senderWorkflowID, workflowOptions, mutexWorkflow, namespace, resourceID, unlockTimeout)

	if err != nil {
		activity.GetLogger(ctx).Error("Unable to signal with start workflow", "Error", err)
	} else {
		activity.GetLogger(ctx).Info("Signaled and started Workflow", "WorkflowID", wr.GetID(), "RunID", wr.GetRunID())
	}

	return &workflow.Execution{
		ID:    wr.GetID(),
		RunID: wr.GetRunID(),
	}, nil
}

// generateUnlockChannelName generates release lock channel name
func generateUnlockChannelName(senderWorkflowID string) string {
	return fmt.Sprintf("unlock-event-%s", senderWorkflowID)
}
