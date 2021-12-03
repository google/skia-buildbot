// Package tracing consolidates OpenCensus tracing initialization in one place.
package tracing

import (
	"os"

	"go.skia.org/infra/go/tracing"
)

// Init tracing for this application.
func Init(local bool) error {
	f := 0.2
	if local {
		f = 1.0
	}
	// TODO(jcgregorio) Add a Tracing section to Config, for now hard-code the ProjectID.
	//  https://skbug.com/12686
	return tracing.Initialize(f, "skia-public", map[string]interface{}{
		// This environment variable should be set in the k8s templates.
		"podName": os.Getenv("MY_POD_NAME"),
	})
}
