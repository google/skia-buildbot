package loggingtracer

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/now"
)

func TestLoggingTracer(t *testing.T) {
	fakeNow := time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)

	// Initialize logging tracer.
	originalTracer := trace.DefaultTracer
	Initialize()
	defer func() { trace.DefaultTracer = originalTracer }()

	// Capture logs.
	var logs []string
	originalLogf := Logf
	Logf = func(format string, v ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, v...))
	}
	defer func() { Logf = originalLogf }()

	ctx := now.TimeTravelingContext(fakeNow)
	a(ctx)

	// These patterns make this test case fragile (i.e. inserting/deleting lines in this file might
	// break this test case), but it is important to test that we are reporting the correct line
	// numbers.
	expectedLogPatterns := []string{
		`^Starting span: a \[.*/loggingtracer_test.go:69]$`,
		`^Starting span: b \[.*/loggingtracer_test.go:77]$`,
		`^Starting span: c \[.*/loggingtracer_test.go:85]$`,
		`^Starting span: d \[.*/loggingtracer_test.go:92]$`,
		`^Ending span: d \[.*/loggingtracer_test.go:94] \(1s\)$`,
		`^Ending span: c \[.*/loggingtracer_test.go:88] \(3s\)$`,
		`^Starting span: c \[.*/loggingtracer_test.go:85]$`,
		`^Starting span: d \[.*/loggingtracer_test.go:92]$`,
		`^Ending span: d \[.*/loggingtracer_test.go:94] \(1s\)$`,
		`^Ending span: c \[.*/loggingtracer_test.go:88] \(3s\)$`,
		`^Ending span: b \[.*/loggingtracer_test.go:81] \(11s\)$`,
		`^Starting span: e \[.*/loggingtracer_test.go:98]$`,
		`^Ending span: e \[.*/loggingtracer_test.go:100] \(500ms\)$`,
		`^Ending span: a \[.*/loggingtracer_test.go:73] \(21.5s\)$`,
	}

	require.Len(t, logs, len(expectedLogPatterns))
	for i, pattern := range expectedLogPatterns {
		match, err := regexp.MatchString(pattern, logs[i])
		require.NoError(t, err)
		assert.Truef(
			t,
			match,
			"Log message does not match the expected pattern.\nActual message: %q.\nExpected pattern: %q.",
			logs[i],
			pattern)
	}
}

func a(ctx context.Context) context.Context {
	ctx, span := trace.StartSpan(ctx, "a")
	defer span.End()
	ctx = b(ctx)
	ctx = e(ctx)
	return fakeSpanDelay(ctx, span, 10*time.Second)
}

func b(ctx context.Context) context.Context {
	ctx, span := trace.StartSpan(ctx, "b")
	defer span.End()
	ctx = c(ctx)
	ctx = c(ctx)
	return fakeSpanDelay(ctx, span, 5*time.Second)
}

func c(ctx context.Context) context.Context {
	ctx, span := trace.StartSpan(ctx, "c")
	defer span.End()
	ctx = d(ctx)
	return fakeSpanDelay(ctx, span, 2*time.Second)
}

func d(ctx context.Context) context.Context {
	ctx, span := trace.StartSpan(ctx, "d")
	defer span.End()
	return fakeSpanDelay(ctx, span, time.Second)
}

func e(ctx context.Context) context.Context {
	ctx, span := trace.StartSpan(ctx, "e")
	defer span.End()
	return fakeSpanDelay(ctx, span, 500*time.Millisecond)
}

func fakeSpanDelay(ctx context.Context, span *trace.Span, duration time.Duration) context.Context {
	ctxGetterSetter := span.Internal().(spanCtxGetterSetter)
	ctx = context.WithValue(ctxGetterSetter.GetCtx(), now.ContextKey, now.Now(ctx).Add(duration))
	ctxGetterSetter.SetCtx(ctx)
	return ctx
}
