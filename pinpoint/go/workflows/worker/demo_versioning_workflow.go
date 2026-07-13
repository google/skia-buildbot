package main

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// DemoVersioningWorkflow is a dummy workflow used to demonstrate Build-ID versioning.
// Version 1 expects a single string argument and sleeps for 15 sec.
func DemoVersioningWorkflow(ctx workflow.Context, name string) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("V1 Started! Breaking change deployed successfully.", "Name", name)

	// Sleep for 15 seconds
	_ = workflow.Sleep(ctx, 15*time.Second)

	logger.Info("V1 Finished successfully!")
	return nil
}
