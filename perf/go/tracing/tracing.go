// Package tracing consolidates OpenCensus tracing initialization in one place.
package tracing

import (
	"os"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/skerr"
)

// Init tracing for this application.
func Init(local bool) error {
	options := stackdriver.Options{
		// TODO(jcgregorio) Add a Tracing section to Config, for now hard-code the ProjectID. https://skbug.com/12686
		ProjectID:                "skia-public",
		TraceSpansBufferMaxBytes: 80_000_000,
		DefaultTraceAttributes: map[string]interface{}{
			"podName": os.Getenv("MY_POD_NAME"),
		},
	}
	exporter, err := stackdriver.NewExporter(options)
	if err != nil {
		return skerr.Wrap(err)
	}

	trace.RegisterExporter(exporter)
	sampler := trace.ProbabilitySampler(0.2)
	if local {
		sampler = trace.AlwaysSample()
	}
	trace.ApplyConfig(trace.Config{DefaultSampler: sampler})
	return nil
}
