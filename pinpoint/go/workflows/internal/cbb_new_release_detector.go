package internal

import (
	"log"

	"go.skia.org/infra/go/skerr"
	"go.temporal.io/sdk/workflow"
)

// CbbNewReleaseDetectorWorkflow is the most basic Workflow Defintion.
func CbbNewReleaseDetectorWorkflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)

	var builds []BuildInfo
	if err := workflow.ExecuteActivity(ctx, GetChromeReleasesInfoActivity).Get(ctx, &builds); err != nil {
		return skerr.Wrap(err)
	}
	// TODO(b/388894957): Remove printing builds info.
	for _, build := range builds {
		log.Printf("Channel:%s, Platform:%s, Version:%s", build.Channel, build.Platform, build.Version)
	}

	// TODO(b/388894957): Use Spanner to detect new releases.
	return nil
}
