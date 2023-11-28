package tracingtest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.opencensus.io/trace"
)

func exampleTraceTestFunc(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "testFunc")
	defer span.End()
}

func TestExporter(t *testing.T) {
	exporter := &Exporter{}
	trace.RegisterExporter(exporter)
	defer trace.UnregisterExporter(exporter)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	ctx := context.Background()
	exampleTraceTestFunc(ctx)

	assert.NotEmpty(t, exporter.SpanData())
}
