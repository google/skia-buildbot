package cloudlogging

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/logging"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

// cloudLogger is a sklogimpl.Logger which uses the non-deprecated
// cloud.google.com/go/logging package.
type cloudLogger struct {
	logger *logging.Logger
}

// New returns a sklogimpl.Logger instance. Writes the log URL to stdout.
func New(ctx context.Context, projectID, logID string, ts oauth2.TokenSource, labels map[string]string) (*cloudLogger, error) {
	logsClient, err := logging.NewClient(ctx, projectID, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	logger := logsClient.Logger(logID, logging.CommonLabels(labels))
	fmt.Printf("Connected Cloud Logging; logs can be found here:\n\thttps://console.cloud.google.com/logs/viewer?project=%s&advancedFilter=logName%%3D%%22projects%%2F%s%%2Flogs%%2F%s%%22\n", projectID, projectID, logID)
	return &cloudLogger{
		logger: logger,
	}, nil
}

// Return the logging.Logger.
func (cl *cloudLogger) Logger() *logging.Logger {
	return cl.logger
}

func convertSeverity(severity sklogimpl.Severity) logging.Severity {
	switch severity {
	case sklogimpl.Debug:
		return logging.Debug
	case sklogimpl.Info:
		return logging.Info
	case sklogimpl.Warning:
		return logging.Warning
	case sklogimpl.Error:
		return logging.Error
	case sklogimpl.Fatal:
		return logging.Alert
	default:
		return logging.Default
	}
}

// Log implements sklogimpl.Logger.
func (cl *cloudLogger) Log(depth int, severity sklogimpl.Severity, format string, args ...interface{}) {
	// The following is copied from previous impl; we should consider replacing this with structured
	// logs.
	// See doc on sklog.Logger interface.
	var stack map[string]string
	prettyPayload := strings.Builder{}
	if severity == sklogimpl.Fatal {
		stackDepth := 2 + depth
		stacks := skerr.CallStack(5, stackDepth)

		_, _ = prettyPayload.WriteString(stacks[0].String())
		_ = prettyPayload.WriteByte(' ')
		stack = map[string]string{
			"stacktrace_0": stacks[0].String(),
			"stacktrace_1": stacks[1].String(),
			"stacktrace_2": stacks[2].String(),
			"stacktrace_3": stacks[3].String(),
			"stacktrace_4": stacks[4].String(),
		}
	}
	if len(format) == 0 {
		_, _ = fmt.Fprint(&prettyPayload, args...)
	} else {
		_, _ = fmt.Fprintf(&prettyPayload, format, args...)
	}
	cl.logger.Log(logging.Entry{
		Payload:   prettyPayload.String(),
		Timestamp: time.Now(),
		Severity:  convertSeverity(severity),
		Labels:    stack,
	})
	if severity == sklogimpl.Fatal {
		cl.Flush()
		os.Exit(255)
	}
}

// Flush implements sklogimpl.Logger.
func (cl *cloudLogger) Flush() {
	if err := cl.logger.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to flush logging.Logger: %s", err)
	}
}

var _ sklogimpl.Logger = (*cloudLogger)(nil)
