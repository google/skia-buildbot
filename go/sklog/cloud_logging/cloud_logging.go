package cloud_logging

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/logging"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog/sklog_impl"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

// cloudLogger is a sklog_impl.Logger which uses the non-deprecated
// cloud.google.com/go/logging package.
type cloudLogger struct {
	logger *logging.Logger
}

// New returns a sklog_impl.Logger instance. Writes the log URL to stdout.
func New(ctx context.Context, projectId, logId string, ts oauth2.TokenSource, labels map[string]string) (*cloudLogger, error) {
	logsClient, err := logging.NewClient(ctx, projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	logger := logsClient.Logger(logId, logging.CommonLabels(labels))
	fmt.Printf("Connected Cloud Logging; logs can be found here: https://console.cloud.google.com/logs/viewer?project=%s&resource=gce_instance&logName=projects%%2F%s%%2Flogs%%2F%s", projectId, projectId, logId)
	return &cloudLogger{
		logger: logger,
	}, nil
}

// Return the logging.Logger.
func (cl *cloudLogger) Logger() *logging.Logger {
	return cl.logger
}

func convertSeverity(severity sklog_impl.Severity) logging.Severity {
	switch severity {
	case sklog_impl.Debug:
		return logging.Debug
	case sklog_impl.Info:
		return logging.Info
	case sklog_impl.Warning:
		return logging.Warning
	case sklog_impl.Error:
		return logging.Error
	case sklog_impl.Fatal:
		return logging.Alert
	default:
		return logging.Default
	}
}

// Log implements sklog_impl.Logger.
func (cl *cloudLogger) Log(depth int, severity sklog_impl.Severity, format string, args ...interface{}) {
	// The following is copied from previous impl; we should consider replacing this with structured
	// logs.
	// See doc on sklog.Logger interface.
	stackDepth := 2 + depth
	stacks := skerr.CallStack(5, stackDepth)

	prettyPayload := strings.Builder{}
	_, _ = prettyPayload.WriteString(stacks[0].String())
	_ = prettyPayload.WriteByte(' ')
	if len(format) == 0 {
		_, _ = fmt.Fprint(&prettyPayload, args...)
	} else {
		_, _ = fmt.Fprintf(&prettyPayload, format, args...)
	}
	stack := map[string]string{
		"stacktrace_0": stacks[0].String(),
		"stacktrace_1": stacks[1].String(),
		"stacktrace_2": stacks[2].String(),
		"stacktrace_3": stacks[3].String(),
		"stacktrace_4": stacks[4].String(),
	}
	cl.logger.Log(logging.Entry{
		Payload:   prettyPayload.String(),
		Timestamp: time.Now(),
		Severity:  convertSeverity(severity),
		Labels:    stack,
	})
}

// LogAndDie implements sklog_impl.Logger.
func (cl *cloudLogger) LogAndDie(depth int, format string, args ...interface{}) {
	sklog_impl.DefaultLogAndDie(cl, depth, format, args)
}

// Flush implements sklog_impl.Logger.
func (cl *cloudLogger) Flush() {
	if err := cl.logger.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to flush logging.Logger: %s", err)
	}
}

var _ sklog_impl.Logger = (*cloudLogger)(nil)
