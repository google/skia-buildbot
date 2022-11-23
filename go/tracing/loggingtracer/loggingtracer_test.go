package loggingtracer

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/trace"
)

func TestLoggingTracer(t *testing.T) {
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

	ctx := context.Background()
	a(ctx)

	// These patterns make this test case fragile (i.e. inserting/deleting lines in this file might
	// break this test case), but it is important to test that we are reporting the correct line
	// numbers.
	expectedLogPatterns := []string{
		`Starting span: a \[.*/loggingtracer_test.go:65]`,
		`Starting span: b \[.*/loggingtracer_test.go:72]`,
		`Starting span: c \[.*/loggingtracer_test.go:79]`,
		`Starting span: d \[.*/loggingtracer_test.go:85]`,
		`Ending span: d \[.*/loggingtracer_test.go:87]`,
		`Ending span: c \[.*/loggingtracer_test.go:82]`,
		`Starting span: c \[.*/loggingtracer_test.go:79]`,
		`Starting span: d \[.*/loggingtracer_test.go:85]`,
		`Ending span: d \[.*/loggingtracer_test.go:87]`,
		`Ending span: c \[.*/loggingtracer_test.go:82]`,
		`Ending span: b \[.*/loggingtracer_test.go:76]`,
		`Starting span: e \[.*/loggingtracer_test.go:90]`,
		`Ending span: e \[.*/loggingtracer_test.go:92]`,
		`Ending span: a \[.*/loggingtracer_test.go:69]`,
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

func a(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "a")
	defer span.End()
	b(ctx)
	e(ctx)
}

func b(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "b")
	defer span.End()
	c(ctx)
	c(ctx)
}

func c(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "c")
	defer span.End()
	d(ctx)
}

func d(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "d")
	defer span.End()
}

func e(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "e")
	defer span.End()
}
