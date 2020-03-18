// Package glog_and_cloud offers a way to log using glog or Google Cloud Logging
// in a seamless way. By default, it will log everything using glog. Simply call
// glog_and_cloud.InitCloudLogging() to immediately start sending log messages
// to the configured Google Cloud Logging endpoint.

package glog_and_cloud

import (
	"bytes"
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog/sklog_impl"
)

const (
	// b/120145392
	KUBERNETES_FILE_LINE_NUMBER_WORKAROUND = true
)

var (
	// Severities used primarily by Cloud Logging.
	DEBUG    = sklog_impl.Debug.StackdriverString()
	INFO     = sklog_impl.Info.StackdriverString()
	WARNING  = sklog_impl.Warning.StackdriverString()
	ERROR    = sklog_impl.Error.StackdriverString()
	CRITICAL = "CRITICAL"
	ALERT    = sklog_impl.Fatal.StackdriverString()

	// cloudLogger is the module-level logger. If this is nil, we will just log using glog.
	cloudLogger CloudLogger

	// defaultReportName is the module-level default report name, set in PreInitCloudLogging.
	// See cloud_logging.go for more information.
	defaultReportName string
)

func NewLogger() sklog_impl.Logger {
	return sklogger{}
}

type sklogger struct{}

func (_ sklogger) Log(depth int, severity sklog_impl.Severity, fmt string, args ...interface{}) {
	log(depth+1, severity.StackdriverString(), defaultReportName, sklog_impl.LogMessageToString(fmt, args...))
}

func (skl sklogger) LogAndDie(depth int, fmt string, args ...interface{}) {
	payload := sklog_impl.LogMessageToString(fmt, args...)
	if cloudLogger != nil {
		log(depth+1, ALERT, defaultReportName, payload)
		cloudLogger.Flush()
	}
	// logToGlog expects the depth argument to include logToGlog's frame.
	logToGlog(depth+2, ALERT, payload)
}

func (_ sklogger) Flush() {
	if cloudLogger != nil {
		cloudLogger.Flush()
	}
	glog.Flush()
}

// CustomLog allows any clients to write a LogPayload to a report with a
// custom group name (e.g. "log file name"). This is the simplest way for
// an app to send logs to somewhere other than the default report name
// (typically based on the app-name).
func CustomLog(reportName string, payload *LogPayload) {
	if cloudLogger != nil {
		cloudLogger.CloudLog(reportName, payload)
	} else {
		// must be local or not initialized
		logToGlog(3, payload.Severity, payload.Payload)
	}
}

// SetLogger changes the package to use the given CloudLogger.
func SetLogger(lg CloudLogger) {
	cloudLogger = lg
}

// log creates a log entry.  This log entry is either sent to Cloud Logging or glog if the former is
// not configured.  reportName is the "virtual log file" used by cloud logging.  reportName is
// ignored by glog. Both logs include file and line information.
func log(depthOffset int, severity, reportName, payload string) {
	// See doc on sklog.Logger interface.
	stackDepth := 2 + depthOffset
	stacks := skerr.CallStack(5, stackDepth)

	prettyPayload := fmt.Sprintf("%s %v", stacks[0].String(), payload)
	if cloudLogger == nil {
		// TODO(kjlubick): After cloud logging has baked in a while, remove the backup logs to glog
		if severity == ALERT {
			// Include the stacktrace.
			payload += "\n\n" + string(debug.Stack())

			// First log directly to glog as an error, in case the write to
			// cloud logging fails to ensure that the message does get
			// logged to disk. ALERT, aka, Fatal* will be logged to glog
			// after the call to CloudLog. If we called logToGlog with
			// alert, it will die before reporting the fatal to CloudLog.
			logToGlog(stackDepth, ERROR, fmt.Sprintf("FATAL: %s", payload))
		} else {
			// In the non-ALERT case, log using glog before CloudLog, in
			// case something goes wrong.
			logToGlog(stackDepth, severity, payload)
		}
	}

	if cloudLogger != nil {
		stack := map[string]string{
			"stacktrace_0": stacks[0].String(),
			"stacktrace_1": stacks[1].String(),
			"stacktrace_2": stacks[2].String(),
			"stacktrace_3": stacks[3].String(),
			"stacktrace_4": stacks[4].String(),
		}
		cloudLogger.CloudLog(reportName, &LogPayload{
			Time:        time.Now(),
			Severity:    severity,
			Payload:     prettyPayload,
			ExtraLabels: stack,
		})
	}
}

// logToGlog creates a glog entry.  Depth is how far up the call stack to extract file information.
// Severity and msg (message) are self explanatory.
func logToGlog(depth int, severity string, msg string) {
	if KUBERNETES_FILE_LINE_NUMBER_WORKAROUND {
		_, file, line, ok := runtime.Caller(depth)
		if !ok {
			file = "???"
			line = 1
		} else {
			slash := strings.LastIndex(file, "/")
			if slash >= 0 {
				file = file[slash+1:]
			}
		}

		// Following the example of glog, avoiding fmt.Printf for performance reasons
		//https://github.com/golang/glog/blob/master/glog.go#L560
		buf := bytes.Buffer{}
		buf.WriteString(file)
		buf.WriteRune(':')
		buf.WriteString(strconv.Itoa(line))
		buf.WriteRune(' ')
		buf.WriteString(msg)
		msg = buf.String()
	}
	switch severity {
	case DEBUG:
		glog.InfoDepth(depth, msg)
	case INFO:
		glog.InfoDepth(depth, msg)
	case WARNING:
		glog.WarningDepth(depth, msg)
	case ERROR:
		glog.ErrorDepth(depth, msg)
	case ALERT:
		glog.FatalDepth(depth, msg)
	default:
		glog.ErrorDepth(depth, msg)
	}
}

var _ sklog_impl.Logger = sklogger{}
