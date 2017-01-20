package sklog

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/skia-dev/glog"
	"golang.org/x/net/context"
	logging "google.golang.org/api/logging/v2beta1"
)

// This file acts as a wrapper around the Cloud API. Any thing a user wants to send to Cloud
// Logging is a Log Entry. From the docs:
// "In the v2 API, Stackdriver Logging associates every log  entry with a monitored resource using
// the resource field in the v2 LogEntry object. A monitored resource is an abstraction used to
// characterize many kinds of objects in your cloud infrastructure."
// We create a monitored resource of the generic type logging_log, aka "Log stream". This resource
// takes one label: "name", which we supply with logGrouping [see InitCloudLogging].

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

	// RFC3339NanoZeroPad fixes time.RFC3339Nano which only uses as many
	// sub-second digits are required to represent the time, which makes it
	// unsuitable for sorting.  This format ensures that all 9 nanosecond digits
	// are used, padding with zeroes if necessary.
	RFC3339NanoZeroPad = "2006-01-02T15:04:05.000000000Z07:00"

	// CLOUD_LOGGING_URL_FORMAT is the URL, where first string is defaultReport, second is logGrouping
	CLOUD_LOGGING_URL_FORMAT = "https://console.cloud.google.com/logs/viewer?logName=projects%%2Fgoogle.com:skia-buildbots%%2Flogs%%2F%s&resource=logging_log%%2Fname%%2F%s"

	// WRITE_LOG_ENTRIES_REQUEST_TIMEOUT is the Timeout for making the WriteLogEntriesRequest request.
	WRITE_LOG_ENTRIES_REQUEST_TIMEOUT = time.Second

	// MAX_QPS_LOG is the max number of log lines we expect to generate per second.
	MAX_QPS_LOG = 10000

	// LOG_WRITE_SECONDS is the time between batch writes to cloud logging, in seconds.
	LOG_WRITE_SECONDS = 5
)

// PreInitCloudLogging does the first step in initializing cloud logging.
//
// CLIENTS SHOULD NOT CALL PreInitCloudLogging directly. Instead use common.InitWith().
func PreInitCloudLogging(logGrouping, defaultReport string) error {
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
			"name": logGrouping,
		},
	}
	defaultReportName = defaultReport
	logger = newLogsClient(nil, hostname, r)
	return nil
}

// PostInitCloudLogging finishes initializing cloud logging.
//
// CLIENTS SHOULD NOT CALL PostInitCloudLogging directly. Instead use common.InitWith().
func PostInitCloudLogging(c *http.Client, metricsCallback MetricsCallback) error {
	lc, err := logging.New(c)
	if err != nil {
		return fmt.Errorf("Problem setting up logging.Service: %s", err)
	}
	logger.(*logsClient).service = lc
	if metricsCallback != nil {
		sawLogWithSeverity = metricsCallback
	}
	url := fmt.Sprintf(CLOUD_LOGGING_URL_FORMAT, defaultReportName, logger.(*logsClient).loggingResource.Labels["name"])
	glog.Infof(`=====================================================
Cloud logging configured, see %s for rest of logs. This file will only contain errors involved with cloud logging/metrics.
=====================================================`, url)
	// Make first cloud logging entry.
	Info("Cloud logging configured.")
	return nil
}

// CLIENTS SHOULD NOT CALL InitCloudLogging directly. Instead use common.InitWithCloudLogging.
// InitCloudLogging initializes the module-level logger. logGrouping refers to the
// MonitoredResource's name. If blank, logGrouping defaults to the machine's hostname.
// defaultReportName refers to the default "virtual log file" name that Log Entries will be
// associated with if no other reportName is given. If an error is returned, cloud logging will not
//  be used, instead glog will.
// metricsCallback should not call any sklog.* methods, to avoid infinite recursion.
// InitCloudLogging should be called before the program creates any go routines
// such that all subsequent logs are properly sent to the Cloud.
func InitCloudLogging(c *http.Client, logGrouping, defaultReport string, metricsCallback MetricsCallback) error {
	if err := PreInitCloudLogging(logGrouping, defaultReport); err != nil {
		return err
	}
	return PostInitCloudLogging(c, metricsCallback)
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

	// Wait until any outstanding writes to Cloud Logging have finished.
	Flush()
}

// LogPayload represents the contents of a Log Entry with a text payload.
type LogPayload struct {
	// Payload is the text content of the log file.
	Payload string
	// Time is the when the log happened.
	Time time.Time
	// Severity is one of the strings found in this package, such as DEBUG, ALERT, etc.
	Severity string
	// Any additional labels to be added to this Log Entry. hostname is already included.
	// These labels can be searched on.
	ExtraLabels map[string]string
}

// logsClient implements the CloudLogger interface.
type logsClient struct {
	// An authenticated connection to the cloud logging API.
	service *logging.Service
	// retrieved once and stored here to avoid constant os.Hostname() calls.
	hostname string
	// A MonitoredResource to associate all Log Entries with. See top of file for more information.
	loggingResource *logging.MonitoredResource

	// flush is a channel used to indicate that the outgoing logs in buffer
	// should be sent. The passed in channel is then closed when the flush is
	// complete.
	flush chan chan struct{}

	// A buffered input channel of LogEntry.
	payloadCh chan *logging.LogEntry

	// A local buffer of *logging.LogEntry's that we keep around and then push
	// to cloud logging every LOG_WRITE_SECONDS.
	buffer []*logging.LogEntry
}

func newLogsClient(service *logging.Service, hostname string, loggingResource *logging.MonitoredResource) *logsClient {
	logger := &logsClient{
		flush:           make(chan chan struct{}),
		service:         service,
		payloadCh:       make(chan *logging.LogEntry, MAX_QPS_LOG),
		buffer:          make([]*logging.LogEntry, 0, LOG_WRITE_SECONDS*MAX_QPS_LOG),
		loggingResource: loggingResource,
		hostname:        hostname,
	}
	go logger.background()
	return logger
}

// CloudLoggingInstance returns the module-level cloud logger.
func CloudLoggingInstance() CloudLogger {
	return logger
}

// SetCloudLoggerForTesting sets the globally available CloudLogger which will be used for
// reporting errors. This should be used only for mocking.
func SetCloudLoggerForTesting(c CloudLogger) {
	logger = c
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

	for _, payload := range payloads {
		labels := map[string]string{
			"hostname": c.hostname,
		}
		for k, v := range payload.ExtraLabels {
			labels[k] = v
		}
		c.payloadCh <- &logging.LogEntry{
			// The LogName is the second stage of grouping, after MonitoredResource name. The first
			// part of the following string is boilerplate to tell cloud logging what project this is.
			// The logs/reportName part basically creates a virtual log file with a given name in the
			// MonitoredResource. Logs made to the same MonitoredResource with the same LogName will be
			// coalesced, as if they were in the same "virtual log file".
			LogName: "projects/google.com:skia-buildbots/logs/" + reportName,
			// Labels allow for a third stage of grouping, after MonitoredResource name and LogName.
			// These are strictly optional and can be different from LogEntry to LogEntry. There is no
			// automatic coalescing of logs based on Labels, but they can be filtered upon.
			Labels:      labels,
			TextPayload: payload.Payload,
			Timestamp:   payload.Time.Format(RFC3339NanoZeroPad),
			// Required. See comment in logsClient struct.
			Resource: c.loggingResource,
			Severity: payload.Severity,
		}
	}
}

// Flush waits until all outstanding cloud logging pushes are done.
// See comment in BatchCloudLog
func (c *logsClient) Flush() {
	ch := make(chan struct{})
	c.flush <- ch
	<-ch
}

func (c *logsClient) pushBatch() {
	// Bail out if the cloud logging service is not finished initializing.
	if c.service == nil {
		glog.Infof("Logging service is still nil.")
		return
	}
	request := logging.WriteLogEntriesRequest{
		Entries: c.buffer,
	}
	if len(c.buffer) > 0 {
		glog.Infof("Sending log entry batch of %d", len(c.buffer))
	}
	ctx, cancel := context.WithTimeout(context.Background(), WRITE_LOG_ENTRIES_REQUEST_TIMEOUT)
	defer cancel()
	if resp, err := c.service.Entries.Write(&request).Context(ctx).Do(); err != nil {
		// We can't use httputil.DumpResponse, because that doesn't accept *logging.WriteLogEntriesResponse
		glog.Errorf("Problem writing logs \nResponse:\n%v:\n%s", spew.Sdump(resp), err)
	} else if resp.HTTPStatusCode != http.StatusOK {
		glog.Errorf("Response code %d", resp.HTTPStatusCode)
	}
	c.buffer = c.buffer[:0]
}

func (c *logsClient) background() {
	ticker := time.NewTicker(LOG_WRITE_SECONDS * time.Second)
	for {
		select {
		case <-ticker.C:
			c.pushBatch()
		case ch := <-c.flush:
			c.pushBatch()
			close(ch)
		case logPayload := <-c.payloadCh:
			c.buffer = append(c.buffer, logPayload)
		}
	}
}

// Validate that the concrete structs faithfully implement their respective interfaces.
var _ CloudLogger = (*logsClient)(nil)
