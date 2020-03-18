// This package provides an interface to swap out the logging destination and
// implements logging metrics.

package sklog_impl

import (
	"fmt"
	"os"
	"runtime/debug"
)

// Severity identifies the sort of log: info, warning etc.
type Severity int

// These constants identify the log levels in order of increasing severity.
const (
	Debug Severity = iota
	Info
	Warning
	Error
	Fatal

	// RFC3339NanoZeroPad fixes time.RFC3339Nano which only uses as many
	// sub-second digits are required to represent the time, which makes it
	// unsuitable for sorting.  This format ensures that all 9 nanosecond digits
	// are used, padding with zeroes if necessary.
	RFC3339NanoZeroPad = "2006-01-02T15:04:05.000000000Z07:00"
)

// String returns the full name of the Severity.
func (s Severity) String() string {
	switch s {
	case Debug:
		return "DEBUG"
	case Info:
		return "INFO"
	case Warning:
		return "WARNING"
	case Error:
		return "ERROR"
	case Fatal:
		return "FATAL"
	default:
		return ""
	}
}

// StackdriverString returns the name of the severity per
// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#LogSeverity
func (s Severity) StackdriverString() string {
	switch s {
	case Debug:
		return "DEBUG"
	case Info:
		return "INFO"
	case Warning:
		return "WARNING"
	case Error:
		return "ERROR"
	case Fatal:
		return "ALERT"
	default:
		return ""
	}
}

// AllSeverities returns a list of all severities understood by this package.
func AllSeverities() []Severity {
	return []Severity{
		Debug,
		Info,
		Warning,
		Error,
		Fatal,
	}
}

// metricsCallback should not call any sklog.* methods, to avoid infinite recursion.
type MetricsCallback func(severity Severity)

// sawLogWithSeverity is used to report metrics about logs seen so we can
// alert if many ERRORs are seen, for example. This is set up to break a
// dependency cycle, such that sklog does not depend on metrics2.
var sawLogWithSeverity MetricsCallback = func(s Severity) {}

// SetMetricsCallback sets sawLogWithSeverity.
//
// This is set up to break a dependency cycle, such that sklog does not depend
// on metrics2.
func SetMetricsCallback(metricsCallback MetricsCallback) {
	if metricsCallback != nil {
		sawLogWithSeverity = metricsCallback
	}
}

// Logger represents a log destination. All methods must be goroutine-safe. All
// methods must handle all errors, either by ignoring the errors or writing to
// os.Stderr.
type Logger interface {
	// Log sends a log message to a log destination. `depth` indicates the number
	// of stack frames to skip. (Implementations should call
	// `skerr.CallStack(2+depth, ...)`, assuming this call is in the Log method
	// itself, to skip the Log method and skerr.CallStack in addition to `depth`
	// frames.) `severity` is one of the severity constants.
	// This method handles both Print-like and Printf-like formatting; `format`
	// will be the empty string when Print-like formatting is desired.
	// To support composing Loggers, this method should not automatically die when
	// severity is Fatal.
	Log(depth int, severity Severity, format string, args ...interface{})

	// Log, Flush (if necessary), then end the program. This method should not
	// return. Severity is assumed to be Fatal.
	// TODO(dogben): Remove this method and implement in package-level LogAndDie
	// when all implementations are able to log at Fatal level without dying. See
	// DefaultLogAndDie.
	LogAndDie(depth int, format string, args ...interface{})

	// Flush sends any log messages that may be buffered or queued to the log
	// destination before returning.
	Flush()
}

// logger is the module-level logger. THIS MUST BE SET by an init function in
// sklog.go; otherwise there's a very good chance of getting a nil pointer
// panic.
var logger Logger = nil

// SetLogger changes the package to use the given Logger.
func SetLogger(lg Logger) {
	logger = lg
}

// Package-level Log function; for use by sklog package.
func Log(depth int, severity Severity, format string, args ...interface{}) {
	sawLogWithSeverity(severity)
	logger.Log(depth+1, severity, format, args...)
}

// Package-level LogAndDie function; for use by sklog package.
func LogAndDie(depth int, format string, args ...interface{}) {
	// In LogAndDie, there is no callback to sawLogWithSeverity, as the program will soon exit
	// and the counter will be reset to 0.
	logger.LogAndDie(depth+1, format, args...)
	_, _ = fmt.Fprintf(os.Stderr, "logger of type %T failed to die after LogAndDie", logger)
	os.Exit(255)
}

// Package-level Flush function; for use by sklog package.
func Flush() {
	logger.Flush()
}

// LogMessageToString converts the last two params to Logger.Log to a string, as
// documented.
func LogMessageToString(format string, args ...interface{}) string {
	if len(format) == 0 {
		return fmt.Sprint(args...)
	} else {
		return fmt.Sprintf(format, args...)
	}
}

// DefaultLogAndDie provides an implementation of LogAndDie for Loggers that do
// not have special behavior.
func DefaultLogAndDie(l Logger, depth int, format string, args ...interface{}) {
	l.Log(depth+1, Fatal, format, args...)
	l.Flush()
	debug.PrintStack()
	os.Exit(255)
}
