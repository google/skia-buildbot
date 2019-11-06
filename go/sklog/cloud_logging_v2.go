package sklog

import (
	"context"

	"cloud.google.com/go/logging"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

// cloudLogger is a CloudLogger which uses the non-deprecated
// cloud.google.com/go/logging package.
type cloudLogger struct {
	logger *logging.Logger
}

// NewCloudLogger returns a CloudLogger instance.
func NewCloudLogger(ctx context.Context, projectId, logId string, ts oauth2.TokenSource, labels map[string]string) (*cloudLogger, error) {
	logsClient, err := logging.NewClient(ctx, projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	logger := logsClient.Logger(logId, logging.CommonLabels(labels))
	Infof("Connected Cloud Logging; logs can be found here: https://pantheon.corp.google.com/logs/viewer?project=%s&resource=gce_instance&logName=projects%%2F%s%%2Flogs%%2F%s", projectId, projectId, logId)
	return &cloudLogger{
		logger: logger,
	}, nil
}

// Return the logging.Logger.
func (cl *cloudLogger) Logger() *logging.Logger {
	return cl.logger
}

// See documentation for CloudLogger interface.
func (cl *cloudLogger) CloudLog(reportName string, payload *LogPayload) {
	cl.logger.Log(logging.Entry{
		Payload:   payload.Payload,
		Timestamp: payload.Time,
		Severity:  logging.ParseSeverity(payload.Severity),
		Labels:    payload.ExtraLabels,
	})
}

// See documentation for CloudLogger interface.
func (cl *cloudLogger) Flush() {
	if err := cl.logger.Flush(); err != nil {
		Errorf("Failed to flush logging.Logger: %s", err)
	}
}
