// the gcl package offers a wrapper (CloudLogger) around the Google Cloud Logging api
package gcl

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/util"
	logging "google.golang.org/api/logging/v2beta1"
)

const (
	DEFAULT   = "DEFAULT"
	DEBUG     = "DEBUG"
	INFO      = "INFO"
	NOTICE    = "NOTICE"
	WARNING   = "WARNING"
	ERROR     = "ERROR"
	CRITICAL  = "CRITICAL"
	ALERT     = "ALERT"
	EMERGENCY = "EMERGENCY"

	LOGGING_WRITE_SCOPE = logging.LoggingWriteScope
	LOGGING_READ_SCOPE  = logging.LoggingReadScope
)

// A CloudLogger interacts with the CloudLogging api
type CloudLogger interface {
	// Log writes the log payload to the cloud log with a given reportName. ReportName is the name
	// of the " virtual log file" that the logs belong to. Logs of the same reportName will be
	// grouped together. Log entries will automatically be labeled with the machine's hostname.
	// Most clients will use the convenience methods below instead of Log.  The primary use case for
	// Log is if customizing the reportName, the time, or use one of the less common severities.
	Log(reportName string, payload *LogPayload)
}

// LogPayload represents the contents of a log message.
type LogPayload struct {
	// Payload is the text content of the log file.
	Payload string
	// Time is the when the log happened.
	Time time.Time
	// Severity is one of the strings found in this file.
	Severity string
}

type logsClient struct {
	service *logging.Service
	// retrieved once and stored here to avoid constant os.Hostname() calls.
	hostname string
	// From the docs: "In the v2 API, Stackdriver Logging associates every log entry with a
	// monitored resource using the resource field in the v2 LogEntry object. A monitored resource
	// is an abstraction used to characterize many kinds of objects in your cloud infrastructure."
	// We create a monitored resource of the generic type logging_log, aka "Log stream". This
	// resource takes one label: "name", which we supply with "skolo-" + logGrouping.  Despite what
	// "GET https://logging.googleapis.com/v2beta1/monitoredResourceDescriptors" says, projectId is
	// NOT set by this label, but by the LogName.
	// MonitoredResource name is the first stage of grouping logs.  For example, setting the
	// MonitoredResource's name based on the hostname (the default) forces the logs to be broken
	// apart by physical machine.  One case in which this may  not be desirable is when there are
	// tmany identical machines.  In this case, he logGrouping should be set to something like
	// "raspberry-pis", and then logs from all raspberry pis will be grouped together.  The name of
	// the "virtual log file" is the second stage of grouping and labels on the LogEntry allows a
	// third stage of grouping, see Log().
	loggingResource *logging.MonitoredResource
}

// The module-level logger
var logger CloudLogger

// The module-level default report name.
var defaultReportName string

// Init initializes the module-level logger.  logGrouping refers to the MonitoredResource's name.
// If blank, logGrouping defaults to the machine's hostname.  defaultReportName refers to the
// default "virtual log file" name that the convenience methods will log to.
func Init(c *http.Client, logGrouping, defaultReport string) error {
	lc, err := logging.New(c)
	if err != nil {
		return fmt.Errorf("Problem setting up logging.Service: %s", err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Could not get hostname: %s", err)
	}
	if logGrouping == "" {
		logGrouping = hostname
	}
	r := &logging.MonitoredResource{
		Type: "logging_log",
		Labels: map[string]string{
			"name": "skolo-" + logGrouping,
		},
	}
	defaultReportName = defaultReport
	logger = &logsClient{
		service:         lc,
		loggingResource: r,
		hostname:        hostname,
	}
	return nil
}

// Instance returns the module-level logger
func Instance() CloudLogger {
	return logger
}

// SetErrorLogger sets the globally available CloudLogger which will be used for reporting errors.
// This should be used only for mocking.
func SetLogger(c CloudLogger) {
	logger = c
}

// LogError writes an error to CloudLogging if the global logger has been set.
// Otherwise, it just prints it using glog.
func Log(reportName string, payload *LogPayload) {
	if logger == nil {
		glog.Fatalf("Cloud logger has not been initialized")
	}
	logger.Log(reportName, payload)
}

// LogError writes an error to CloudLogging if the global logger has been set.
// Otherwise, it just prints it using glog.
func LogError(reportName string, err error) {
	if logger == nil {
		glog.Fatalf("Cloud logger has not been initialized")
	}
	logger.Log(reportName, &LogPayload{
		Time:     time.Now(),
		Severity: ERROR,
		Payload:  err.Error(),
	})
}

// log sends a log to Cloud logging of a given severity.  It includes the file and
// line information like glog.
func log(severity, reportName string, payloads ...string) {
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
	for _, payload := range payloads {
		payload = fmt.Sprintf("%s:%d %s", file, line, payload)
		if logger == nil {
			glog.Fatalf("Cloud logger has not been initialized")
		}
		logger.Log(reportName, &LogPayload{
			Time:     time.Now(),
			Severity: severity,
			Payload:  payload,
		})
	}
}

// These convenience methods use the current time and the default report name associated with
// the CloudLogger.  They match the glog interface.  Info and Infoln do the same thing (as do all
// pairs), because adding a newline to the end of a Cloud logging message means nothing as all
// logs are separate entries.
func Info(msg ...string) {
	log(INFO, defaultReportName, msg...)
}

func Infof(format string, v ...interface{}) {
	log(INFO, defaultReportName, fmt.Sprintf(format, v...))
}

func Infoln(msg ...string) {
	log(INFO, defaultReportName, msg...)
}

func Warning(msg ...string) {
	log(WARNING, defaultReportName, msg...)
}

func Warningf(format string, v ...interface{}) {
	log(WARNING, defaultReportName, fmt.Sprintf(format, v...))
}

func Warningln(msg ...string) {
	log(WARNING, defaultReportName, msg...)
}

func Error(msg ...string) {
	log(ERROR, defaultReportName, msg...)
}

func Errorf(format string, v ...interface{}) {
	log(ERROR, defaultReportName, fmt.Sprintf(format, v...))
}

func Errorln(msg ...string) {
	log(ERROR, defaultReportName, msg...)
}

// Fatal* makes an Alert-level log and then panics, similar to glog.Fatalf()
func Fatal(msg ...string) {
	log(ALERT, defaultReportName, msg...)
	glog.Fatal(msg)
}

func Fatalf(format string, v ...interface{}) {
	log(ALERT, defaultReportName, fmt.Sprintf(format, v...))
	glog.Fatalf(format, v...)
}

func Fatalln(msg ...string) {
	log(ALERT, defaultReportName, msg...)
	glog.Fatalln(msg)
}

// See documentation on interface.
func (c *logsClient) Log(reportName string, payload *LogPayload) {
	if payload == nil {
		glog.Warningf("Will not log nil log to %s", reportName)
		return
	}

	log := logging.LogEntry{
		// The LogName is the second stage of grouping, after MonitoredResource name.  The first
		// part of the following string is boilerplate to tell cloud logging what project this is.
		// The logs/reportName part basically creates a virtual log file with a given name in the
		// MonitoredResource.  Logs made to the same MonitoredResource with the same LogName will be
		// coalesced, as if they were in the same "virtual log file".
		LogName: "projects/google.com:skia-buildbots/logs/" + reportName,
		// Labels allow for a third stage of grouping, after MonitoredResource name and LogName.
		// These are strictly optional and can be different from LogEntry to LogEntry.  There is no
		// automatic coalescing of logs based on Labels, but they can be filtered upon.
		Labels: map[string]string{
			"hostname": c.hostname,
		},
		TextPayload: payload.Payload,
		Timestamp:   payload.Time.Format(util.RFC3339NanoZeroPad),
		// Required.  See comment in logsClient struct.
		Resource: c.loggingResource,
		Severity: payload.Severity,
	}

	entries := logging.WriteLogEntriesRequest{
		Entries: []*logging.LogEntry{
			&log,
		},
	}

	if resp, err := c.service.Entries.Write(&entries).Do(); err != nil {
		glog.Errorf("Problem writing logs %v %v:\n%s", payload, resp, err)
	} else {
		glog.Infof("Response code %d", resp.HTTPStatusCode)
	}
}
