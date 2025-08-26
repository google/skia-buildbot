// Package tracing consolidates OpenCensus tracing initialization in one place.
package tracing

import (
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
)

// Init tracing for this application.
func Init(local bool, cfg *config.InstanceConfig) error {
	sklog.Warning("Reminder that all tracing is currently turned off.")
	return nil
	/*
		f := cfg.TraceSampleProportion
		if local {
			f = 1.0
		}
		// TODO(jcgregorio) Add a Tracing section to Config, for now hard-code the ProjectID.
		//  https://skbug.com/12686
		return tracing.Initialize(float64(f), "skia-monitoring", map[string]interface{}{
			// This environment variable should be set in the k8s templates.
			"podName": os.Getenv("MY_POD_NAME"),
		})
	*/
}
