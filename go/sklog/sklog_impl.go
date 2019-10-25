// This file provides an interface to swap out the logging destination and
// implements logging metrics.

package sklog

import (
	"fmt"
	"io"
	"os"
)

const (
	// Severities used primarily by Cloud Logging.
	DEBUG    = "DEBUG"
	INFO     = "INFO"
	NOTICE   = "NOTICE"
	WARNING  = "WARNING"
	ERROR    = "ERROR"
	CRITICAL = "CRITICAL"
	ALERT    = "ALERT"
)

// AllSeverities is the list of all severities that sklog supports.
var AllSeverities = []string{
	DEBUG,
	INFO,
	NOTICE,
	WARNING,
	ERROR,
	CRITICAL,
	ALERT,
}

var (
	// logger is the module-level logger. By default it simply writes to Stderr.
	logger Logger

	// sawLogWithSeverity is used to report metrics about logs seen so we can
	// alert if many ERRORs are seen, for example. This is set up to break a
	// dependency cycle, such that sklog does not depend on metrics2.
	sawLogWithSeverity MetricsCallback = func(s string) {}

	// AllSeverities is the list of all severities that sklog supports.
	AllSeverities = []string{
		DEBUG,
		INFO,
		NOTICE,
		WARNING,
		ERROR,
		CRITICAL,
		ALERT,
	}
)

type MetricsCallback func(severity string)

// sawLogWithSeverity is used to report metrics about logs seen so we can
// alert if many ERRORs are seen, for example. This is set up to break a
// dependency cycle, such that sklog does not depend on metrics2.
var sawLogWithSeverity MetricsCallback = func(s string) {}

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
	// frames.) `severity` is one of the severity constants. `payload` is the
	// formatted log message. Errors can be written to os.Stderr.
	// To support composing Loggers, this method should not automatically die when
	// severity is ALERT.
	Log(depth int, severity string, payload string)

	// Log, Flush (if necessary), then end the program. This method should not
	// return.
	LogAndDie(depth int, severity string, payload string)

	// Flush sends any log messages that may be buffered or queued to the log
	// destination before returning. Errors can be written to os.Stderr.
	Flush()
}

// logger is the module-level logger. By default it simply writes to Stderr.
var logger Logger = NewSimpleLogger()

// SetLogger changes the package to use the given Logger.
func SetLogger(lg Logger) {
	logger = lg
}

func NewSimpleLogger() Logger {
	return simpleLogger{w: os.Stderr}
}

type simpleLogger struct {
	w io.Writer
}

func (l simpleLogger) Log(_ int, severity string, payload string) {
	_, _ = fmt.Fprintf(l.w, "%s %s\n", severity, payload)
}

func (l simpleLogger) LogAndDie(debug int, severity string, payload string) {
	l.Log(debug, severity, payload)
	// glog sets a timeout on the Flush for Fatal. We could do that here, but
	// I'd like to keep this as simple as possible, and it seems unlikely that
	// it will cause problems.
	l.Flush()
	panic(payload)
}

func (l simpleLogger) Flush() {
	// Like os.File
	type syncer interface {
		Sync() error
	}
	if s, ok := l.w.(syncer); ok {
		s.Sync()
		return
	}
	// Like bufio.Writer
	type flusher interface {
		Flush() error
	}
	if f, ok := l.w.(flusher); ok {
		f.Flush()
		return
	}
}
