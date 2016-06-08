package gcl

import (
	"fmt"
	"net/http"
	"time"

	"github.com/skia-dev/glog"
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
)

type CloudLogger interface {
	// Log writes the log payload to the cloud log with a given reportName. ReportName is the name
	// of the "file" that the logs belong to. Logs of the same reportName will be grouped together.
	// In addition to the reportName, logs will automatically be tagged with the machine's hostname.
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
}

func New(c *http.Client) (CloudLogger, error) {
	lc, err := logging.New(c)
	if err != nil {
		return nil, fmt.Errorf("Problem setting up logging.Service: %s", err)
	}
	return &logsClient{
		service: lc,
	}, nil
}

func (c *logsClient) Log(reportName string, payload *LogPayload) {
	// TODO(kjlubick) in a future CL
	if payload == nil {
		glog.Infof("Will not log nil log to %s", reportName)
		return
	}
	glog.Infof("Should log to %s: %q @ %s", reportName, payload.Payload, payload.Time)
}

// The errorLogger allows for deeply buried pieces to log an error to Cloud logging, without
// needing to pipe a CloudLogger all the way into it.
var errorLogger CloudLogger

// SetErrorLogger sets the globally available CloudLogger which will be used for reporting errors.
func SetErrorLogger(c CloudLogger) {
	errorLogger = c
}

// LogError writes an error to CloudLogging if the global errorLogger has been set.
// Otherwise, it just prints it using glog.
func LogError(reportName string, err error) {
	glog.Errorf("[Meta-%s]%s", reportName, err)
	if errorLogger != nil {
		errorLogger.Log(reportName, &LogPayload{
			Payload:  err.Error(),
			Time:     time.Now(),
			Severity: ERROR,
		})
	}

}
