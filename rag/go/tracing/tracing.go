package tracing

import (
	"os"

	"go.skia.org/infra/go/tracing"
)

const (
	autoDetectProjectID = ""
)

// Init tracing for this application.
func Init(local bool, serviceName string, sampleProportion float64) error {
	if local {
		return nil
	}

	return tracing.Initialize(sampleProportion, autoDetectProjectID, map[string]interface{}{
		// This environment variable should be set in the k8s templates.
		"podName": os.Getenv("MY_POD_NAME"),
		"service": serviceName,
	})
}
