// Package tracing consolidates OpenCensus tracing initialization in one place.
package tracing

import (
	"os"

	"go.skia.org/infra/go/tracing"
)

// Initialize sets up trace options and exporting for this application. It will sample the given
// proportion of traces.
func Initialize(traceSampleProportion float64, instanceName string) error {
	return tracing.Initialize(traceSampleProportion, "skia-public", map[string]interface{}{
		// This environment variable should be set in the k8s templates.
		"podName":  os.Getenv("K8S_POD_NAME"),
		"instance": instanceName,
	})
}
