// This package provides an interface to swap out the logging destination and
// implements logging metrics.

package sklog_impl

import "fmt"

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

// Char returns a one-byte indicator of the Severity.
func (s Severity) Char() byte {
	return "DIWEF"[s]
}

var severityStrings = [5]string{
	"DEBUG",
	"INFO",
	"WARNING",
	"ERROR",
	"FATAL",
}

// String returns the full name of the Severity.
func (s Severity) String() string {
	return severityStrings[s]
}

var severityBytes = [5][]byte{[]byte("DEBUG"),
	[]byte("INFO"),
	[]byte("WARNING"),
	[]byte("ERROR"),
	[]byte("FATAL"),
}

// Bytes returns the same value as String() converted to bytes. Do not modify the return value.
func (s Severity) Bytes() []byte {
	return severityBytes[s]
}

var stackdriverStrings = [5]string{
	"DEBUG",
	"INFO",
	"WARNING",
	"ERROR",
	"ALERT",
}

// StackdriverString returns the name of the severity per
// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#LogSeverity
func (s Severity) StackdriverString() string {
	return stackdriverStrings[s]
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

// Logger represents a log destination. All methods must be goroutine-safe.
type Logger interface {
	// Log sends a log message to a log destination. `depth` indicates the number
	// of stack frames to skip. (Implementations should call
	// `skerr.CallStack(2+depth, ...)`, assuming this call is in the Log method
	// itself, to skip the Log method and skerr.CallStack in addition to `depth`
	// frames.) `severity` is one of the severity constants.
	// This method handles both Print-like and Printf-like formatting; `fmt_tmpl` will
	// be the empty string when Print-like formatting is desired.
	// Any errors that occur during the call to Log should be ignored or written
	// to os.Stderr.
	// To support composing Loggers, this method should not automatically die when
	// severity is Fatal.
	Log(depth int, severity Severity, fmt_tmpl string, args ...interface{})

	// Log, Flush (if necessary), then end the program. This method should not
	// return.
	LogAndDie(depth int, severity Severity, fmt_tmpl string, args ...interface{})

	// Flush sends any log messages that may be buffered or queued to the log
	// destination before returning. Errors can be written to os.Stderr.
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
func Log(depth int, severity Severity, fmt_tmpl string, args ...interface{}) {
	sawLogWithSeverity(severity)
	logger.Log(depth+1, severity, fmt_tmpl, args...)
}

// Package-level LogAndDie function; for use by sklog package.
func LogAndDie(depth int, severity Severity, fmt_tmpl string, args ...interface{}) {
	// In LogAndDie, there is no callback to sawLogWithSeverity, as the program will soon exit
	// and the counter will be reset to 0.
	logger.LogAndDie(depth+1, severity, fmt_tmpl, args...)
	panic(fmt.Sprintf("logger of type %T failed to die after LogAndDie", logger))
}

// Package-level Flush function; for use by sklog package.
func Flush() {
	logger.Flush()
}

// LogMessageToString converts the last two params to Logger.Log to a string, as
// documented.
func LogMessageToString(fmt_tmpl string, args ...interface{}) string {
	if len(fmt_tmpl) == 0 {
		return fmt.Sprint(args...)
	} else {
		return fmt.Sprintf(fmt_tmpl, args...)
	}
}
