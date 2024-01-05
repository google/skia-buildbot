// Package tracingtest provides helpers for testing opencensus tracing instrumentation.
package tracingtest

import (
	"sync"

	"go.opencensus.io/trace"
)

// Exporter is an in-memory implementation of [trace.Exporter] suitable
// for verifying opencensus tracing calls in unit tests.
type Exporter struct {
	mu       sync.Mutex
	spanData []*trace.SpanData
}

func (tte *Exporter) ExportSpan(s *trace.SpanData) {
	tte.mu.Lock()
	defer tte.mu.Unlock()
	tte.spanData = append(tte.spanData, s)
}

// SpanData returns any SpanData exported so far.
func (tte *Exporter) SpanData() []*trace.SpanData {
	tte.mu.Lock()
	defer tte.mu.Unlock()
	return tte.spanData
}

// Exporter implements [trace.Exporter].
var _ trace.Exporter = &Exporter{}
