package internal

import (
	"time"

	"go.skia.org/infra/pinpoint/go/run_benchmark"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var (
	// Default option for the local activity.
	//
	// The local activity should be completed within 10 seconds, otherwise it should be
	// executed as a regular activity. We also don't expect retryable errors however we
	// leave some buffers just in case, and this should be quickly populated.
	localActivityOptions = workflow.LocalActivityOptions{
		ScheduleToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 10,
		},
	}

	// Default option for the regular activity.
	//
	// Activity usually communicates with the external services and is expected to complete
	// within a minute. RetryPolicy helps to recover from unexpected network errors or service
	// interruptions.
	// For activities that expect long running time and complex dependent services, a separate
	// option should be curated for individual activities.
	regularActivityOptions = workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 10,
		},
	}

	// Default option for the child workflow.
	//
	// This generally means time tolerance from the most top level workflow, in this case, it is
	// the bisection workflow. The actual timeout heavily depends on the swarming resources.
	// We don't want to leave this running for very long but also know there are cases where
	// the resources will not be immediately available.
	// This setting indicates that each child job should finish within 12 hours.
	childWorkflowOptions = workflow.ChildWorkflowOptions{
		// 4 hours of compile time + 8 hours of test run time
		WorkflowExecutionTimeout: 12 * time.Hour,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 4,
		},
		// When the parent workflow got cancelled, cancellation will be requested of the child workflow
		ParentClosePolicy: enums.PARENT_CLOSE_POLICY_REQUEST_CANCEL,
	}

	// Acitivity option for Building Chrome.
	buildActivityOption = workflow.ActivityOptions{
		// Expect longer running time
		StartToCloseTimeout: 6 * time.Hour,
		// The default gRPC timeout is 5 minutes, longer than that so it can capture grpc errors.
		HeartbeatTimeout: 6 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    15 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    1 * time.Minute,
			MaximumAttempts:    3,
		},
	}

	// Child Workflow option for Building Chrome
	buildWorkflowOptions = workflow.ChildWorkflowOptions{
		WorkflowExecutionTimeout: 4 * time.Hour,
		RetryPolicy: &temporal.RetryPolicy{
			// We don't want to retry building if we know it is failing.
			MaximumAttempts: 1,
		},
	}

	// Two hours pending timeouts + six hours running timeouts
	// Those are defined here:
	// https://chromium.googlesource.com/chromium/src/+/3b293fe/testing/buildbot/chromium.perf.json#499
	runBenchmarkWorkflowOptions = workflow.ChildWorkflowOptions{
		WorkflowExecutionTimeout: 8 * time.Hour,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}

	runBenchmarkActivityOption = workflow.ActivityOptions{
		// Make timeout of Temporal activity 1 minute longer than swarming task timeout,
		// so that after a swarming task times out, Temporal activity has some extra
		// time to handle it.
		ScheduleToCloseTimeout: (run_benchmark.ExecutionTimeoutSecs + 60) * time.Second,
		HeartbeatTimeout:       6 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    15 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    1 * time.Minute,
			MaximumAttempts:    3,
		},
	}

	runBenchmarkPendingActivityOption = workflow.ActivityOptions{
		ScheduleToCloseTimeout: run_benchmark.PendingTimeoutSecs * time.Second,
		HeartbeatTimeout:       6 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    15 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    1 * time.Minute,
			MaximumAttempts:    3,
		},
	}

	getBrowerVersionsWorkflowOptions = workflow.ChildWorkflowOptions{
		WorkflowExecutionTimeout: 2 * time.Hour,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	}
)
