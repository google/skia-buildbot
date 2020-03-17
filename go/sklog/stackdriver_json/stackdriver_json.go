// stackdriver_json implements logging JSON that can be interpreted by Stackdriver as log messages.
package stackdriver_json

import (
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog/sklog_impl"
)

// Adapter implements sklog_impl.Logger by sending JSON structs to another logger's StructuredLog.
type Adapter struct {
	sklog_impl.Logger
}

// New returns an Adapter that uses the given sklog_impl.Logger.
func New(l sklog_impl.Logger) *Adapter {
	return &Adapter{
		Logger: l,
	}
}

// sourceLocation identifies a stack frame.
type sourceLocation struct {
	File     string `json:"file"`
	Line     int64  `json:"line"`
	Function string `json:"function"`
}

// structuredLog stores a log message in the format interpreted by Stackdriver.
type structuredLog struct {
	// Timestamp, Severity, and Message are interpreted by Stackdriver logging.
	// (Discovered by experimentation. You might think this is documented at
	// https://cloud.google.com/logging/docs/reference/v2/rpc/google.appengine.logging.v1#google.appengine.logging.v1.LogLine
	// but using "log_message" as the field name doesn't work.)
	Timestamp string `json:"time"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	// SourceLocation is documented at both
	// https://cloud.google.com/logging/docs/reference/v2/rpc/google.appengine.logging.v1#google.appengine.logging.v1.LogLine
	// and
	// https://cloud.google.com/logging/docs/reference/v2/rpc/google.logging.v2#logentry
	// but doesn't seem to do anything in Stackdriver logging. We use the same
	// schema just in case the feature is added at some point.
	SourceLocation sourceLocation `json:"source_location"`
	// The following are not intended to match Stackdriver logging types.
	Stack []sourceLocation `json:"stack"`
}

// Log implements sklog_impl.Logger.
func (a *Adapter) Log(depth int, severity sklog_impl.Severity, format string, args ...interface{}) {
	stacks := skerr.CallStack(5, 1+depth)
	logStack := make([]sourceLocation, len(stacks), len(stacks))
	for i, s := range stacks {
		logStack[i] = sourceLocation{
			File:     s.File,
			Line:     int64(s.Line),
			Function: s.FunctionName(),
		}
	}
	payload := sklog_impl.LogMessageToString(format, args...)
	a.StructuredLog(structuredLog{
		Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
		Severity:       severity.StackdriverString(),
		Message:        payload,
		SourceLocation: logStack[0],
		Stack:          logStack,
	})
}

// LogAndDie implements sklog_impl.Logger.
func (a *Adapter) LogAndDie(depth int, format string, args ...interface{}) {
	sklog_impl.DefaultLogAndDie(a, depth, format, args)
}

// Assert that we implement the sklog_impl.Logger interface:
var _ sklog_impl.Logger = (*Adapter)(nil)
