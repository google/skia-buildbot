// Package tracing consolidates OpenCensus tracing initialization in one place.
package tracing

import (
	"os"

	"go.skia.org/infra/go/tracing"
	"go.skia.org/infra/perf/go/config"
)

const (
	autoDetectProjectID = ""
)

// Init tracing for this application.
func Init(local bool, cfg *config.InstanceConfig) error {
	f := cfg.TraceSampleProportion
	if local {
		return nil
	}

	return tracing.Initialize(float64(f), autoDetectProjectID, map[string]interface{}{
		// This environment variable should be set in the k8s templates.
		"podName": os.Getenv("MY_POD_NAME"),
	})
}
