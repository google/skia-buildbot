// Package loggingtracer provides a trace.Tracer that wraps trace.DefaultTracer and logs span
// starts/ends. It is intended for local debugging only, as usage in production might cause noisy
// and/or verbose logs.
//
// To use this logger, call Initialize(). To customize the logger function, override Logf.
package loggingtracer

import (
	"context"
	"fmt"
	"runtime"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/sklog"
)

// Logf is the logging function used by this package.
var Logf func(format string, v ...interface{}) = sklog.Infof

// Initialize wraps trace.DefaultTracer with a trace.Tracer implementation that logs span
// starts/ends using the Logf function defined in this package.
//
// This is intended for local debugging only. Usage in production might cuase noisy/verbose logs.
func Initialize() {
	trace.DefaultTracer = loggingTracer{actualTracer: trace.DefaultTracer}
}

// getCaller returns the calling file:line via runtime.Caller.
func getCaller(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if ok {
		return fmt.Sprintf("%s:%d", file, line)
	}
	return "unknown caller"
}

// loggingTracer is a trace.Tracer that wraps another trace.Tracer and logs span stars/ends.
type loggingTracer struct {
	actualTracer trace.Tracer
}

// StartSpan implements the trace.Tracer interface.
func (t loggingTracer) StartSpan(ctx context.Context, name string, o ...trace.StartOption) (context.Context, *trace.Span) {
	Logf("Starting span: %s [%s]", name, getCaller(3))
	ctx, actualSpan := t.actualTracer.StartSpan(ctx, name, o...)
	return ctx, trace.NewSpan(loggingSpan{actualSpan: actualSpan, name: name})
}

// StartSpanWithRemoteParent implements the trace.Tracer interface.
func (t loggingTracer) StartSpanWithRemoteParent(ctx context.Context, name string, parent trace.SpanContext, o ...trace.StartOption) (context.Context, *trace.Span) {
	Logf("Starting span with remote parent: %s [%s]", name, getCaller(3))
	ctx, actualSpan := t.actualTracer.StartSpanWithRemoteParent(ctx, name, parent, o...)
	return ctx, trace.NewSpan(loggingSpan{actualSpan: actualSpan, name: name})
}

// FromContext implements the trace.Tracer interface.
func (t loggingTracer) FromContext(ctx context.Context) *trace.Span {
	return t.actualTracer.FromContext(ctx)
}

// NewContext implements the trace.Tracer interface.
func (t loggingTracer) NewContext(parent context.Context, s *trace.Span) context.Context {
	return t.actualTracer.NewContext(parent, s)
}

var _ trace.Tracer = (*loggingTracer)(nil)

// loggingSpan is a trace.SpanInterface that wraps a trace.Span and logs the span end.
type loggingSpan struct {
	actualSpan *trace.Span
	name       string
}

// IsRecordingEvents implements the trace.SpanInterface interface.
func (s loggingSpan) IsRecordingEvents() bool { return s.actualSpan.IsRecordingEvents() }

// End implements the trace.SpanInterface interface.
func (s loggingSpan) End() {
	Logf("Ending span: %s [%s]", s.name, getCaller(3))
	s.actualSpan.End()
}

// SpanContext implements the trace.SpanInterface interface.
func (s loggingSpan) SpanContext() trace.SpanContext { return s.actualSpan.SpanContext() }

// SetName implements the trace.SpanInterface interface.
func (s loggingSpan) SetName(name string) { s.actualSpan.SetName(name) }

// SetStatus implements the trace.SpanInterface interface.
func (s loggingSpan) SetStatus(status trace.Status) { s.actualSpan.SetStatus(status) }

// AddAttributes implements the trace.SpanInterface interface.
func (s loggingSpan) AddAttributes(attributes ...trace.Attribute) {
	s.actualSpan.AddAttributes(attributes...)
}

// Annotate implements the trace.SpanInterface interface.
func (s loggingSpan) Annotate(attributes []trace.Attribute, str string) {
	s.actualSpan.Annotate(attributes, str)
}

// Annotatef implements the trace.SpanInterface interface.
func (s loggingSpan) Annotatef(attributes []trace.Attribute, format string, a ...interface{}) {
	s.actualSpan.Annotatef(attributes, format, a...)
}

// AddMessageSendEvent implements the trace.SpanInterface interface.
func (s loggingSpan) AddMessageSendEvent(messageID, uncompressedByteSize, compressedByteSize int64) {
	s.actualSpan.AddMessageSendEvent(messageID, uncompressedByteSize, compressedByteSize)
}

// AddMessageReceiveEvent implements the trace.SpanInterface interface.
func (s loggingSpan) AddMessageReceiveEvent(messageID, uncompressedByteSize, compressedByteSize int64) {
	s.actualSpan.AddMessageReceiveEvent(messageID, uncompressedByteSize, compressedByteSize)
}

// AddLink implements the trace.SpanInterface interface.
func (s loggingSpan) AddLink(l trace.Link) { s.actualSpan.AddLink(l) }

// String implements the trace.SpanInterface interface.
func (s loggingSpan) String() string { return s.actualSpan.String() }

var _ trace.SpanInterface = (*loggingSpan)(nil)
