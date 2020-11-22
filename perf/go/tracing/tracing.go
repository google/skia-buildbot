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
	exporter, err := stackdriver.NewExporter(stackdriver.Options{
		TraceSpansBufferMaxBytes: 80_000_000,
		DefaultTraceAttributes: map[string]interface{}{
			"podName": os.Getenv("MY_POD_NAME"),
		},
	})
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
