package slog

import (
	"encoding/json"
	"os"

	_logger "github.com/jcgregorio/logger"
	_slog "github.com/jcgregorio/slog"
	"go.skia.org/infra/go/sklog/sklog_impl"
)

// Adapter implements sklog_impl.Logger using _slog.Logger.
type Adapter struct {
	stdLog _slog.Logger
}

// New returns an Adapter that uses the given _slog.Logger.
func New(l _slog.Logger) *Adapter {
	return &Adapter{
		stdLog: l,
	}
}

// LogMode is the mode logging enum type.
type LogMode int

// Types of logging that NewStdErr supports.
const (
	None LogMode = iota
	Stderr
)

// NewStdErr creates a new Adapter that either logs to stderr, or does no logging, depending upon
// the value of mode. It uses github.com/jcgregorio/logger to implement the _slog.Logger interface.
//
// The return value implements sklog_impl.Logger.
//
// Usage:
//
//			sklog_impl.SetLogger(slog.NewStdErr(logToStdErr))
//
func NewStdErr(mode LogMode) *Adapter {
	if mode == Stderr {
		return New(_logger.NewFromOptions(&_logger.Options{
			SyncWriter: os.Stderr,
			DepthDelta: 3,
		}))
	} else {
		return New(_logger.NewNopLogger())
	}
}

// Log implements sklog_impl.Logger.
func (a *Adapter) Log(_ int, severity sklog_impl.Severity, format string, args ...interface{}) {
	payload := sklog_impl.LogMessageToString(format, args...)
	switch severity {
	case sklog_impl.Debug:
		a.stdLog.Debug(payload)
	case sklog_impl.Info:
		a.stdLog.Info(payload)
	case sklog_impl.Warning:
		a.stdLog.Warning(payload)
	case sklog_impl.Error, sklog_impl.Fatal:
		// _slog.Logger.Fatal always exits, so we can only use it in LogAndDie.
		a.stdLog.Error(payload)
	}
}

// LogAndDie implements sklog_impl.Logger.
func (a *Adapter) LogAndDie(depth int, format string, args ...interface{}) {
	payload := sklog_impl.LogMessageToString(format, args...)
	a.stdLog.Fatal(payload)
}

// StructuredLog implements sklog_impl.Logger.
func (a *Adapter) StructuredLog(obj interface{}) {
	b, err := json.Marshal(obj)
	if err != nil {
		a.Log(0, sklog_impl.Error, "StructuredLog message failed to marshal; %s %v", err, obj)
	} else {
		a.stdLog.Raw(string(b))
	}
}

// Flush implements sklog_impl.Logger.
func (_ *Adapter) Flush() {
	_ = os.Stdout.Sync()
}

// Assert that we implement the sklog_impl.Logger interface:
var _ sklog_impl.Logger = (*Adapter)(nil)
