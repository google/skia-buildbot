// Package structuredlogging implements sklogimpl.Logger and logs to either
// stderr or stdout using structured logging. It is intended to be used inside
// of GKE, where logs to stdout/stderr are automatically ingested to Cloud
// Logging.
package structuredlogging

import (
	"bytes"
	"context"
	"fmt"
	"iter"
	"os"
	"runtime"
	"strings"

	"cloud.google.com/go/logging"
	"cloud.google.com/go/logging/apiv2/loggingpb"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sklog/sklogimpl"
)

// TODO(borenet): This is a silly workaround for "call has arguments but no
// formatting directives".
var (
	emptyFormatString = ""
)

func Debug(ctx context.Context, msg ...interface{}) {
	logger.LogCtx(ctx, 1, sklogimpl.Debug, emptyFormatString, msg...)
}

func Debugf(ctx context.Context, format string, v ...interface{}) {
	logger.LogCtx(ctx, 1, sklogimpl.Debug, format, v...)
}

func DebugfWithDepth(ctx context.Context, depth int, format string, v ...interface{}) {
	logger.LogCtx(ctx, 1+depth, sklogimpl.Debug, format, v...)
}

func Info(ctx context.Context, msg ...interface{}) {
	logger.LogCtx(ctx, 1, sklogimpl.Info, emptyFormatString, msg...)
}

func Infof(ctx context.Context, format string, v ...interface{}) {
	logger.LogCtx(ctx, 1, sklogimpl.Info, format, v...)
}

func InfofWithDepth(ctx context.Context, depth int, format string, v ...interface{}) {
	logger.LogCtx(ctx, 1+depth, sklogimpl.Info, format, v...)
}

func Warning(ctx context.Context, msg ...interface{}) {
	logger.LogCtx(ctx, 1, sklogimpl.Warning, emptyFormatString, msg...)
}

func Warningf(ctx context.Context, format string, v ...interface{}) {
	logger.LogCtx(ctx, 1, sklogimpl.Warning, format, v...)
}

func WarningfWithDepth(ctx context.Context, depth int, format string, v ...interface{}) {
	logger.LogCtx(ctx, 1+depth, sklogimpl.Warning, format, v...)
}

func Error(ctx context.Context, msg ...interface{}) {
	logger.LogCtx(ctx, 1, sklogimpl.Error, emptyFormatString, msg...)
}

func Errorf(ctx context.Context, format string, v ...interface{}) {
	logger.LogCtx(ctx, 1, sklogimpl.Error, format, v...)
}

func ErrorfWithDepth(ctx context.Context, depth int, format string, v ...interface{}) {
	logger.LogCtx(ctx, 1+depth, sklogimpl.Error, format, v...)
}

func Fatal(ctx context.Context, msg ...interface{}) {
	logger.LogCtx(ctx, 1, sklogimpl.Fatal, emptyFormatString, msg...)
}

func Fatalf(ctx context.Context, format string, v ...interface{}) {
	logger.LogCtx(ctx, 1, sklogimpl.Fatal, format, v...)
}

func FatalfWithDepth(ctx context.Context, depth int, format string, v ...interface{}) {
	logger.LogCtx(ctx, 1+depth, sklogimpl.Fatal, format, v...)
}

func Flush() {
	logger.Flush()
}

var logger *StructuredLogger

func init() {
	l, err := New(context.Background(), os.Stderr)
	if err != nil {
		sklog.Fatal(err)
	}
	logger = l
}

func Logger() *StructuredLogger {
	return logger
}

type StructuredLogger struct {
	logger *logging.Logger
}

func New(ctx context.Context, file *os.File) (*StructuredLogger, error) {
	logsClient, err := logging.NewClient(
		ctx, "fake-project-id" /* Unused with RedirectAsJSON */)
	if err != nil {
		return nil, err
	}
	logger := logsClient.Logger(
		"fake-log-id", /* Unused with RedirectAsJSON */
		logging.RedirectAsJSON((file)))
	return &StructuredLogger{
		logger: logger,
	}, nil
}

// Flush implements sklogimpl.Logger.
func (s *StructuredLogger) Flush() {
	if err := s.logger.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to flush logging.Logger: %s", err)
	}
}

// Log implements sklogimpl.Logger.
func (s *StructuredLogger) Log(depth int, severity sklogimpl.Severity, tmpl string, args ...interface{}) {
	s.LogCtx(context.Background(), depth+1, severity, tmpl, args...)
}

func (s *StructuredLogger) LogCtx(ctx context.Context, depth int, severity sklogimpl.Severity, tmpl string, args ...interface{}) {
	var buf bytes.Buffer
	if tmpl == "" {
		fmt.Fprint(&buf, args...)
	} else {
		fmt.Fprintf(&buf, tmpl, args...)
	}
	s.emit(ctx, depth, severity, buf.String())
	if severity == sklogimpl.Fatal {
		trace := stacks(true)
		s.emit(ctx, depth, severity, string(trace))
		s.Flush()
		os.Exit(255)
	}
}

// stacks is a wrapper for runtime.Stack that attempts to recover the data for all goroutines.
// TODO(borenet): Copy/pasted from jcgregorio/logger
func stacks(all bool) []byte {
	// We don't know how big the traces are, so grow a few times if they don't fit. Start large, though.
	n := 10000
	if all {
		n = 100000
	}
	var trace []byte
	for i := 0; i < 5; i++ {
		trace = make([]byte, n)
		nbytes := runtime.Stack(trace, all)
		if nbytes < len(trace) {
			return trace[:nbytes]
		}
		n *= 2
	}
	return trace
}

func (s *StructuredLogger) emit(ctx context.Context, depth int, severity sklogimpl.Severity, msg string) {
	loc := sourceLocation(depth)
	c := getCtx(ctx)
	for msg := range splitMessage(msg) {
		entry := logging.Entry{
			Payload:        msg,
			Severity:       convertSeverity(severity),
			SourceLocation: loc,
		}

		// TODO(borenet): Enable this whenever the logging API supports Split.
		// if len(splitMsg) > 1 {
		// 	entry.Split = &loggingpb.LogSplit{
		// 		Uid:         uuid.New().String(),
		// 		Index:       index,
		// 		TotalSplits: len(splitMsg),
		// 	}
		// }
		if c != nil {
			entry.Labels = c.Labels
			entry.HTTPRequest = c.HTTPRequest
			entry.Operation = c.Operation
		}
		s.logger.Log(entry)
	}
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

func sourceLocation(depth int) *loggingpb.LogEntrySourceLocation {
	_, file, line, ok := runtime.Caller(3 + depth)
	if !ok {
		return nil
	} else {
		slash := strings.LastIndex(file, "/")
		if slash >= 0 {
			file = file[slash+1:]
		}
	}
	return &loggingpb.LogEntrySourceLocation{
		File: file,
		Line: int64(line),
	}
}

// maxLogMessageBytes is presumed to be the maximum log entry message size
// before entries start getting truncated in GKE.
const maxLogMessageBytes = 50 * 1024

func splitMessage(msg string) iter.Seq[string] {
	if len(msg) <= maxLogMessageBytes {
		return func(yield func(string) bool) {
			yield(msg)
		}
	}

	splitLine := func(line string, yield func(string) bool) bool {
		for len(line) > maxLogMessageBytes {
			if !yield(line[:maxLogMessageBytes]) {
				return false
			}
			line = line[maxLogMessageBytes:]
		}
		if len(line) > 0 {
			if !yield(line) {
				return false
			}
		}
		return true
	}

	// Split the message into multiple messages, making an effort to keep
	// individual lines intact.
	return func(yield func(string) bool) {
		var b strings.Builder
		b.Grow(maxLogMessageBytes)
		firstLine := true
		for line := range strings.SplitSeq(msg, "\n") {
			if len(line) > maxLogMessageBytes {
				// Emit any existing batched log lines.
				if b.Len() > 0 {
					if !yield(b.String()) {
						return
					}
					b.Reset()
				}
				// Split the long line into smaller messages.
				if !splitLine(line, yield) {
					return
				}
			} else {
				if b.Len()+len(line)+1 > maxLogMessageBytes {
					// We can't add the current line to the existing batch. Emit the
					// existing batched log lines and then add the current line to
					// a new batch.
					if !yield(b.String()) {
						return
					}
					b.Reset()
				} else if !firstLine {
					b.WriteString("\n")
				}
				b.WriteString(line)
			}
			firstLine = false
		}
		if b.Len() > 0 {
			yield(b.String())
		}
	}
}

var (
	contextKey = &struct{}{}
)

type Context struct {
	HTTPRequest *logging.HTTPRequest `json:"httpRequest,omitempty"`
	Labels      map[string]string
	Operation   *loggingpb.LogEntryOperation `json:"operation,omitempty"`
}

func getCtx(ctx context.Context) *Context {
	if v := ctx.Value(contextKey); v != nil {
		return v.(*Context)
	}
	return nil
}

func WithContext(ctx context.Context, v Context) context.Context {
	return context.WithValue(ctx, contextKey, &v)
}
