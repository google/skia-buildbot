package sklog

import (
	"os"

	_logger "github.com/jcgregorio/logger"
	"github.com/jcgregorio/slog"
)

// CloudLoggerSLogImpl implements CloudLogger using slog.Logger.
type CloudLoggerSLogImpl struct {
	stdLog slog.Logger
}

// NewSLogCloudLogger returns a CloudLoggerSLogImpl that uses the given slog.Logger.
func NewSLogCloudLogger(l slog.Logger) *CloudLoggerSLogImpl {
	return &CloudLoggerSLogImpl{
		stdLog: l,
	}
}

// SLogLogMode is the mode logging enum type.
type SLogLogMode int

// Types of logging that NewSLogLogger supports.
const (
	SLogNone SLogLogMode = iota
	SLogStderr
)

// NewSLogLogger creates a new CloudLoggerSLogImpl that either logs to stderr, or does
// no logging, depending upon the value of mode. It uses github.com/jcgregorio/logger
// to implement the slog.Logger interface.
//
// The return value implements CloudLogger.
//
// Usage:
//
//			sklog.SetLogger(sklog.NewStdErrCloudLogger(logToStdErr))
//
func NewStdErrCloudLogger(mode SLogLogMode) *CloudLoggerSLogImpl {
	if mode == SLogStderr {
		return NewSLogCloudLogger(_logger.NewFromOptions(&_logger.Options{
			SyncWriter: os.Stderr,
			DepthDelta: 3,
		}))
	} else {
		return NewSLogCloudLogger(_logger.NewNopLogger())
	}
}

// See CloudLogger.
func (c *CloudLoggerSLogImpl) CloudLog(reportName string, payload *LogPayload) {
	switch payload.Severity {
	case DEBUG:
		c.stdLog.Debug(payload.Payload)
	case INFO, NOTICE:
		c.stdLog.Info(payload.Payload)
	case WARNING:
		c.stdLog.Warning(payload.Payload)
	case ERROR:
		c.stdLog.Error(payload.Payload)
	case CRITICAL, ALERT:
		c.stdLog.Fatal(payload.Payload)
	}
}

// See CloudLogger.
func (c *CloudLoggerSLogImpl) Flush() {
	_ = os.Stdout.Sync()
}

// Assert that we implement the ClougLogger interface:
var _ CloudLogger = (*CloudLoggerSLogImpl)(nil)
