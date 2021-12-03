// Package tracing consolidates the setup logic for using opencensus tracing and exporting the
// metrics.
package tracing

import (
	"time"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/skerr"
)

// Initialize sets up trace options and exporting for this application. It will sample the given
// proportion of traces. All traces will have the given key-value pairs attached.
func Initialize(traceSampleProportion float64, projectID string, defaultAttrs map[string]interface{}) error {
	exporter, err := stackdriver.NewExporter(stackdriver.Options{
		ProjectID: projectID,
		// Use 10 times the default
		TraceSpansBufferMaxBytes: 80_000_000,
		// It is not clear what the default interval is. One minute seems to be a good value since
		// that is the same as our Prometheus metrics are reported.
		ReportingInterval:      time.Minute,
		DefaultTraceAttributes: defaultAttrs,
	})
	if err != nil {
		return skerr.Wrap(err)
	}

	trace.RegisterExporter(exporter)
	sampler := trace.ProbabilitySampler(traceSampleProportion)
	trace.ApplyConfig(trace.Config{DefaultSampler: sampler})
	return nil
}
