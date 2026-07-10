package main

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// DemoVersioningWorkflow is a dummy workflow used to demonstrate Build-ID versioning.
// Version 1 expects a single string argument and sleeps for 15 sec.
// V1 signature (left as a comment for the demo):
// func DemoVersioningWorkflow(ctx workflow.Context, name string) error {

// V2 breaking change: added 'version' argument and new log messages
func DemoVersioningWorkflow(ctx workflow.Context, name string, version int) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("V2 Started! Breaking change deployed successfully.", "Name", name, "Version", version)

	// Sleep for 15 seconds
	_ = workflow.Sleep(ctx, 15*time.Second)

	logger.Info("V2 Finished successfully!")
	return nil
}
