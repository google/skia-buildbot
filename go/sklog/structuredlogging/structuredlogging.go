// Package structuredlogging implements sklogimpl.Logger and logs to either
// stderr or stdout using structured logging. It is intended to be used inside
// of GKE, where logs to stdout/stderr are automatically ingested to Cloud
// Logging.
package structuredlogging

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"cloud.google.com/go/logging"
	"github.com/google/uuid"
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

var logger = New(os.Stderr)

type StructuredLogger struct {
	file *os.File
}

func New(dst *os.File) *StructuredLogger {
	return &StructuredLogger{
		file: dst,
	}
}

// Log implements sklogimpl.Logger.
func (s *StructuredLogger) Log(depth int, severity sklogimpl.Severity, tmpl string, args ...interface{}) {
	s.LogCtx(context.Background(), depth, severity, tmpl, args...)
}

// flush implements sklogimpl.Logger.
func (s *StructuredLogger) Flush() {
	_ = s.file.Sync()
}

type LogEntry struct {
	HTTPRequest    *LogEntryHTTPRequest    `json:"httpRequest,omitempty"`
	JSONPayload    string                  `json:"jsonPayload,omitempty"`
	Labels         map[string]string       `json:"labels,omitempty"`
	Operation      *LogEntryOperation      `json:"operation,omitempty"`
	Severity       logging.Severity        `json:"severity,omitempty"`
	SourceLocation *LogEntrySourceLocation `json:"sourceLocation,omitempty"`
	Split          *LogEntrySplit          `json:"split,omitempty"`
	TextPayload    string                  `json:"textPayload,omitempty"`
	Trace          string                  `json:"trace,omitempty"`
}

type LogEntrySplit struct {
	UID         string `json:"uid"`
	Index       int    `json:"index"`
	TotalSplits int    `json:"totalSplits"`
}

type LogEntryHTTPRequest struct {
	RequestMethod                  string `json:"requestMethod,omitempty"`
	RequestUrl                     string `json:"requestUrl,omitempty"`
	RequestSize                    string `json:"requestSize,omitempty"`
	Status                         int    `json:"status,omitempty"`
	ResponseSize                   string `json:"responseSize,omitempty"`
	UserAgent                      string `json:"userAgent,omitempty"`
	RemoteIp                       string `json:"remoteIp,omitempty"`
	ServerIp                       string `json:"serverIp,omitempty"`
	Referer                        string `json:"referer,omitempty"`
	Latency                        string `json:"latency,omitempty"`
	CacheLookup                    bool   `json:"cacheLookup,omitempty"`
	CacheHit                       bool   `json:"cacheHit,omitempty"`
	CacheValidatedWithOriginServer bool   `json:"cacheValidatedWithOriginServer,omitempty"`
	CacheFillBytes                 string `json:"cacheFillBytes,omitempty"`
	Protocol                       string `json:"protocol,omitempty"`
}

type LogEntryOperation struct {
	ID       string `json:"id,omitempty"`
	Producer string `json:"producer,omitempty"`
	First    bool   `json:"first,omitempty"`
	Last     bool   `json:"last,omitempty"`
}

type LogEntrySourceLocation struct {
	File     string `json:"file,omitempty"`
	Line     string `json:"line,omitempty"`
	Function string `json:"function,omitempty"`
}

func (s *StructuredLogger) LogCtx(ctx context.Context, depth int, severity sklogimpl.Severity, tmpl string, args ...interface{}) {
	s.emit(ctx, depth, severity, fmt.Sprintf(tmpl, args...))
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
	splitMsg := splitMessage(msg)
	for index, msg := range splitMsg {
		entry := LogEntry{
			Severity:       convertSeverity(severity),
			SourceLocation: loc,
			TextPayload:    msg,
		}
		if len(splitMsg) > 1 {
			entry.Split = &LogEntrySplit{
				UID:         uuid.New().String(),
				Index:       index,
				TotalSplits: len(splitMsg),
			}
		}
		if c != nil {
			entry.Labels = c.Labels
			entry.HTTPRequest = c.HTTPRequest
			entry.Operation = c.Operation
		}
		b, _ := json.Marshal(entry)
		_, _ = s.file.Write(append(b, '\n'))
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

func sourceLocation(depth int) *LogEntrySourceLocation {
	_, file, line, ok := runtime.Caller(4 + depth)
	if !ok {
		return nil
	} else {
		slash := strings.LastIndex(file, "/")
		if slash >= 0 {
			file = file[slash+1:]
		}
	}
	return &LogEntrySourceLocation{
		File: file,
		Line: strconv.Itoa(line),
	}
}

func splitMessage(msg string) []string {
	// TODO(borenet): Split on newlines according to maximum log entry size.
	return strings.Split(msg, "\n")
}

var (
	contextKey = &struct{}{}
)

type Context struct {
	HTTPRequest *LogEntryHTTPRequest `json:"httpRequest,omitempty"`
	Labels      map[string]string
	Operation   *LogEntryOperation `json:"operation,omitempty"`
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
