// The package sklog offers a way to log using glog or Google Cloud Logging in a seemless way.
// By default, the Module level functions (e.g. Infof, Errorln) will all log using glog.  Simply
// call sklog.InitCloudLogging() to immediately start sending log messages to the configured
// Google Cloud Logging endpoint.

package sklog

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/skia-dev/glog"
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

type MetricsCallback func(severity string)

var (
	// The module-level logger.  If this is nil, we will just log using glog.
	logger CloudLogger

	// The module-level default report name.  See cloud_logging.go for more information.
	defaultReportName string

	// used to report metrics about logs seen so we can alert if many ERRORs are seen, for example.
	// This is set up to break a dependency cycle, such that sklog does not depend on metrics2.
	sawLogWithSeverity MetricsCallback = func(s string) {}
)

// These convenience methods will either make a Cloud Logging Entry using the current time and the
// default report name associated with the CloudLogger or log to glog if Cloud Logging is not
// configured.  They match the glog interface.  Info and Infoln do the same thing (as do all pairs),
// because adding a newline to the end of a Cloud Logging Entry or a glog entry means nothing as all
// logs are separate entries.
func Info(msg ...interface{}) {
	sawLogWithSeverity(INFO)
	log(INFO, defaultReportName, fmt.Sprint(msg...))
}

func Infof(format string, v ...interface{}) {
	sawLogWithSeverity(INFO)
	log(INFO, defaultReportName, fmt.Sprintf(format, v...))
}

func Infoln(msg ...interface{}) {
	sawLogWithSeverity(INFO)
	log(INFO, defaultReportName, fmt.Sprintln(msg...))
}

func Warning(msg ...interface{}) {
	sawLogWithSeverity(WARNING)
	log(WARNING, defaultReportName, fmt.Sprint(msg...))
}

func Warningf(format string, v ...interface{}) {
	sawLogWithSeverity(WARNING)
	log(WARNING, defaultReportName, fmt.Sprintf(format, v...))
}

func Warningln(msg ...interface{}) {
	sawLogWithSeverity(WARNING)
	log(WARNING, defaultReportName, fmt.Sprintln(msg...))
}

func Error(msg ...interface{}) {
	sawLogWithSeverity(ERROR)
	log(ERROR, defaultReportName, fmt.Sprint(msg...))
}

func Errorf(format string, v ...interface{}) {
	sawLogWithSeverity(ERROR)
	log(ERROR, defaultReportName, fmt.Sprintf(format, v...))
}

func Errorln(msg ...interface{}) {
	sawLogWithSeverity(ERROR)
	log(ERROR, defaultReportName, fmt.Sprintln(msg...))
}

// Fatal* uses an ALERT Cloud Logging Severity and then panics, similar to glog.Fatalf()
// In Fatal*, there is no callback to sawLogWithSeverity, as the program will soon exit
// and the counter will be reset to 0.
func Fatal(msg ...interface{}) {
	log(ALERT, defaultReportName, fmt.Sprint(msg...))
	Flush()
	panic(fmt.Sprint(msg...))
}

func Fatalf(format string, v ...interface{}) {
	log(ALERT, defaultReportName, fmt.Sprintf(format, v...))
	Flush()
	panic(fmt.Sprintf(format, v...))
}

func Fatalln(msg ...interface{}) {
	log(ALERT, defaultReportName, fmt.Sprintln(msg...))
	Flush()
	panic(fmt.Sprintln(msg...))
}

func Flush() {
	if logger != nil {
		logger.Flush()
	}
	glog.Flush()
}

// log creates a log entry.  This log entry is either sent to Cloud Logging or glog if the former is
// not configured.  reportName is the "virtual log file" used by cloud logging.  reportName is
// ignored by glog. Both logs include file and line information.
func log(severity, reportName, payload string) {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
		line = 1
	} else {
		slash := strings.LastIndex(file, "/")
		if slash >= 0 {
			file = file[slash+1:]
		}
	}
	prettyPayload := fmt.Sprintf("%s:%d %v", file, line, payload)
	if logger == nil {
		logToGlog(3, severity, payload)
	} else {
		// TODO(kjlubick): After cloud logging has baked in a while, remove the backup log to glog
		logToGlog(3, severity, payload)
		logger.CloudLog(reportName, &LogPayload{
			Time:     time.Now(),
			Severity: severity,
			Payload:  prettyPayload,
		})
	}
}

// logToGlog creates a glog entry.  Depth is how far up the call stack to extract file information.
// Severity and msg (message) are self explanatory.
func logToGlog(depth int, severity string, msg interface{}) {
	switch severity {
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
