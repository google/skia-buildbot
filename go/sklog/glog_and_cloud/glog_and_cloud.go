// This package offers a way to log using glog or Google Cloud Logging in a
// seemless way. By default, it will log everything using glog. Simply call
// glog_and_cloud.Init() to immediately start sending log messages
// to the configured Google Cloud Logging endpoint.

package glog_and_cloud

import (
	"bytes"
	"fmt"
	"net/http"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog/cloud_logging"
	"go.skia.org/infra/go/sklog/sklog_impl"
)

const (
	// Severities used primarily by Cloud Logging.
	DEBUG   = sklog_impl.Debug.StackdriverString()
	INFO    = sklog_impl.Info.StackdriverString()
	WARNING = sklog_impl.Warning.StackdriverString()
	ERROR   = sklog_impl.Error.StackdriverString()
	ALERT   = sklog_impl.Fatal.StackdriverString()

	// b/120145392
	KUBERNETES_FILE_LINE_NUMBER_WORKAROUND = true
)

// PreInit does the first step in initializing cloud logging.
//
// CLIENTS SHOULD NOT CALL PreInit directly. Instead use common.InitWith().
func PreInit(logGrouping, defaultReport string) error {
	if err := cloud_logging.PreInit(logGrouping, defaultReport, glogSkLogger{}); err != nil {
		return err
	}
	sklog_impl.SetLogger(sklogger{})
}

// PostInit finishes initializing cloud logging.
//
// CLIENTS SHOULD NOT CALL PostInit directly. Instead use common.InitWith().
func PostInit(c *http.Client, metricsCallback MetricsCallback) error {
	if err := cloud_logging.PostInit(c, metricsCallback); err != nil {
		return err
	}
	url := fmt.Sprintf(CLOUD_LOGGING_URL_FORMAT, defaultReportName, logger.(*logsClient).loggingResource.Labels["name"])
	glog.Infof(`=====================================================
Cloud logging configured, see %s for rest of logs. This file will only contain errors involved with cloud logging/metrics.
=====================================================`, url)
	return nil
}

// CLIENTS SHOULD NOT CALL Init directly. Instead use common.InitWithCloudLogging.
// Init initializes the module-level logger. logGrouping refers to the
// MonitoredResource's name. If blank, logGrouping defaults to the machine's hostname.
// defaultReportName refers to the default "virtual log file" name that Log Entries will be
// associated with if no other reportName is given. If an error is returned, cloud logging will not
//  be used, instead glog will.
// metricsCallback should not call any sklog.* methods, to avoid infinite recursion.
// Init should be called before the program creates any go routines
// such that all subsequent logs are properly sent to the Cloud.
func Init(c *http.Client, logGrouping, defaultReport string, metricsCallback MetricsCallback) error {
	if err := PreInit(logGrouping, defaultReport); err != nil {
		return err
	}
	return PostInit(c, metricsCallback)
}

func NewLogger() sklog_impl.Logger {
	return sklogger{}
}

type sklogger struct{}

func (_ sklogger) Log(depth int, severity sklog_impl.Severity, fmt string, args ...interface{}) {
	log(depth+1, severity.StackdriverString(), sklog_impl.LogMessageToString(fmt, args...))
}

func (skl sklogger) LogAndDie(depth int, severity sklog_impl.Severity, fmt string, args ...interface{}) {
	payload := LogMessageToString(fmt, args...)
	skl.Log(depth+1, severity, "", payload)
	skl.Flush()
	logToGlog(2+depthOffset, ALERT, payload)
}

func (_ sklogger) Flush() {
	if l := cloud_logging.Instance(); l != nil {
		l.Flush()
	}
	glog.Flush()
}

// CustomLog allows any clients to write a LogPayload to a report with a
// custom group name (e.g. "log file name"). This is the simplist way for
// an app to send logs to somewhere other than the default report name
// (typically based on the app-name).
func CustomLog(reportName string, payload *LogPayload) {
	if l := cloud_logging.Instance(); l != nil {
		l.CloudLog(reportName, payload)
	} else {
		// must be local or not initialized
		logToGlog(3, payload.Severity, payload.Payload)
	}
}

// log creates a log entry.  This log entry is either sent to Cloud Logging or glog if the former is
// not configured. Both logs include file and line information.
func log(depthOffset int, severity, payload string) {
	// See doc on sklog.Logger interface.
	stackDepth := 2 + depthOffset
	stacks := skerr.CallStack(5, stackDepth)

	prettyPayload := fmt.Sprintf("%s %v", stacks[0].String(), payload)
	l := cloud_logging.Instance()
	if l == nil {
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

	if l != nil {
		stack := map[string]string{
			"stacktrace_0": stacks[0].String(),
			"stacktrace_1": stacks[1].String(),
			"stacktrace_2": stacks[2].String(),
			"stacktrace_3": stacks[3].String(),
			"stacktrace_4": stacks[4].String(),
		}
		l.CloudLog(defaultReportName, &LogPayload{
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

type glogSkLogger struct{}

func (_ glogSkLogger) Log(depth int, severity sklog_impl.Severity, fmt string, args ...interface{}) {
	if severity == sklog_impl.Fatal {
		// Fatal will cause the program to die, which is not allowed by the
		// sklog_impl.Logger interface. Use Error instead.
		severity = sklog_impl.Error
	}
	logToGlog(depth+2, severity.StackdriverString(), sklog_impl.LogMessageToString(fmt, args...))
}

func (gskl glogSkLogger) LogAndDie(depth int, severity sklog_impl.Severity, fmt string, args ...interface{}) {
	logToGlog(depth+2, ALERT, LogMessageToString(fmt, args...))
}

func (_ glogSkLogger) Flush() {
	glog.Flush()
}
