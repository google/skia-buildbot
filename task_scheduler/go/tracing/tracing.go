// Package tracing consolidates the setup logic for using opencensus tracing and exporting the
// metrics. It is based off of //perf/go/tracing/ and //gold/go/tracing/
package tracing

import (
	"time"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/skerr"
)

// Initialize sets up trace options and exporting for this application. It will sample the given
// proportion of traces.
func Initialize(traceSampleProportion float64) error {
	exporter, err := stackdriver.NewExporter(stackdriver.Options{
		// Use 10 times the default (because that's what perf does).
		TraceSpansBufferMaxBytes: 80_000_000,
		// It is not clear what the default interval is. One minute seems to be a good value since
		// that is the same as our Prometheus metrics are reported.
		ReportingInterval: time.Minute,
	})
	if err != nil {
		return skerr.Wrap(err)
	}

	trace.RegisterExporter(exporter)
	sampler := trace.ProbabilitySampler(traceSampleProportion)
	trace.ApplyConfig(trace.Config{DefaultSampler: sampler})
	return nil
}
