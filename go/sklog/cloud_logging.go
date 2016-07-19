package sklog

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/util"
	logging "google.golang.org/api/logging/v2beta1"
)

// This file acts as a wrapper around the Cloud API. Any thing a user wants to send to Cloud
// Logging is a Log Entry. From the docs:
// "In the v2 API, Stackdriver Logging associates every log  entry with a monitored resource using
// the resource field in the v2 LogEntry object. A monitored resource is an abstraction used to
// characterize many kinds of objects in your cloud infrastructure."
// We create a monitored resource of the generic type logging_log, aka "Log stream". This resource
// takes one label: "name", which we supply with "skolo-" + logGrouping [see InitCloudLogging].

// The MonitoredResource name is the first stage of grouping logs. For example, setting the
// MonitoredResource's name based on the hostname (the default) forces the logs to be broken
// apart by physical machine. One case in which this may not be desirable is when there are
// many identical machines. In this case, the logGrouping should be set to something like
// "raspberry-pis", and then logs from all raspberry pis will be grouped together. The name of
// the "virtual log file" (referred to in this api as the reportName) is the second stage of
// grouping and labels on the LogEntry allows a third stage of grouping [see CloudLog()].

// There a variety of Log Entry types, but for simplicity, we have stuck with the
// "text payload" option
const (
	CLOUD_LOGGING_WRITE_SCOPE = logging.LoggingWriteScope
	CLOUD_LOGGING_READ_SCOPE  = logging.LoggingReadScope
)

// InitCloudLogging initializes the module-level logger. logGrouping refers to the
// MonitoredResource's name. If blank, logGrouping defaults to the machine's hostname.
// defaultReportName refers to the default "virtual log file" name that Log Entries will be
// associated with if no other reportName is given.. If an error is returned, cloud logging will not
//  be used, instead glog will.
func InitCloudLogging(c *http.Client, logGrouping, defaultReport string) error {
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

// A CloudLogger interacts with the CloudLogging api
type CloudLogger interface {
	// CloudLog writes creates a Log Entry from the log payload and sends it to Cloud Logging to be
	// associated with the "virtual log file" given by reportName. Log entries will automatically be
	// labeled with the machine's hostname.
	// Most clients will use the convenience methods in sklog.go. The primary use case for using
	// CloudLog directly is if customizing the reportName, the time, or use one of the less common
	// severities.
	CloudLog(reportName string, payload *LogPayload)

	// BatchCloudLog works like CloudLog, but will send multiple logs at a time.
	BatchCloudLog(reportName string, payloads ...*LogPayload)
}

// LogPayload represents the contents of a Log Entry with a text payload.
type LogPayload struct {
	// Payload is the text content of the log file.
	Payload string
	// Time is the when the log happened.
	Time time.Time
	// Severity is one of the strings found in this file.
	Severity string
}

type logsClient struct {
	// An authenticated connection to the cloud logging API.
	service *logging.Service
	// retrieved once and stored here to avoid constant os.Hostname() calls.
	hostname string
	// A MonitoredResource to associate all Log Entries with. See top of file for more information.
	loggingResource *logging.MonitoredResource
}

// CloudLoggingInstance returns the module-level cloud logger.
func CloudLoggingInstance() CloudLogger {
	return logger
}

// SetCloudLogger sets the globally available CloudLogger which will be used for reporting errors.
// This should be used only for mocking.
func SetCloudLogger(c CloudLogger) {
	logger = c
}

// See description on CloudLogger Interface..
func CloudLog(reportName string, payload *LogPayload) {
	if payload == nil {
		return
	}
	if logger == nil {
		logToGlog(2, payload.Severity, payload.Payload)
	} else {
		logger.CloudLog(reportName, payload)
	}
}

// CloudLogError writes an error to CloudLogging if the global logger has been set.
// Otherwise, it just prints it using glog.
func CloudLogError(reportName string, err error) {
	log(ERROR, reportName, err.Error())
}

// See documentation on interface.
func (c *logsClient) CloudLog(reportName string, payload *LogPayload) {
	if payload == nil {
		glog.Warningf("Will not log nil log to %s", reportName)
		return
	}
	c.BatchCloudLog(reportName, payload)
}

// See documentation on interface.
func (c *logsClient) BatchCloudLog(reportName string, payloads ...*LogPayload) {
	if len(payloads) == 0 {
		glog.Warningf("Will not log empty logs to %s", reportName)
		return
	}
	entries := make([]*logging.LogEntry, 0, len(payloads))
	for _, payload := range payloads {
		log := logging.LogEntry{
			// The LogName is the second stage of grouping, after MonitoredResource name. The first
			// part of the following string is boilerplate to tell cloud logging what project this is.
			// The logs/reportName part basically creates a virtual log file with a given name in the
			// MonitoredResource. Logs made to the same MonitoredResource with the same LogName will be
			// coalesced, as if they were in the same "virtual log file".
			LogName: "projects/google.com:skia-buildbots/logs/" + reportName,
			// Labels allow for a third stage of grouping, after MonitoredResource name and LogName.
			// These are strictly optional and can be different from LogEntry to LogEntry. There is no
			// automatic coalescing of logs based on Labels, but they can be filtered upon.
			Labels: map[string]string{
				"hostname": c.hostname,
			},
			TextPayload: payload.Payload,
			Timestamp:   payload.Time.Format(util.RFC3339NanoZeroPad),
			// Required. See comment in logsClient struct.
			Resource: c.loggingResource,
			Severity: payload.Severity,
		}
		entries = append(entries, &log)
	}
	go func() {
		request := logging.WriteLogEntriesRequest{
			Entries: entries,
		}
		glog.Infof("Sending log entry batch of %d", len(entries))
		if resp, err := c.service.Entries.Write(&request).Do(); err != nil {
			// We can't use httputil.DumpResponse, because that doesn't accept *logging.WriteLogEntriesResponse
			glog.Errorf("Problem writing logs \nLogPayloads:\n%v\nLogEntries:\n%v\nResponse:\n%v:\n%s", spew.Sdump(payloads), spew.Sdump(entries), spew.Sdump(resp), err)
		} else {
			glog.Infof("Response code %d", resp.HTTPStatusCode)
		}
	}()

}
