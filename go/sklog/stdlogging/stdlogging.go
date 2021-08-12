// Package stdlogging implements sklogimpl.Logger and logs to either stderr or stdout.
package stdlogging

import (
	logger "github.com/jcgregorio/logger"
	"go.skia.org/infra/go/sklog/sklogimpl"
)

type stdlog struct {
	logger *logger.Logger
}

// New returns a sklogimpl.Logger that writes to a SyncWriter, such as
// os.Stdout or os.Stderr.
func New(dst logger.SyncWriter) sklogimpl.Logger {
	l := logger.NewFromOptions(&logger.Options{
		SyncWriter:   dst,
		DepthDelta:   3,
		IncludeDebug: true,
	})
	return &stdlog{
		logger: l,
	}
}

// Log implements sklogimpl.Logger.
func (s stdlog) Log(_ int, severity sklogimpl.Severity, fmt string, args ...interface{}) {
	switch severity {
	case sklogimpl.Debug:
		if fmt == "" {
			s.logger.Debug(args...)
		} else {
			s.logger.Debugf(fmt, args...)
		}
	case sklogimpl.Info:
		if fmt == "" {
			s.logger.Info(args...)
		} else {
			s.logger.Infof(fmt, args...)
		}
	case sklogimpl.Warning:
		if fmt == "" {
			s.logger.Warning(args...)
		} else {
			s.logger.Warningf(fmt, args...)
		}
	case sklogimpl.Error:
		if fmt == "" {
			s.logger.Error(args...)
		} else {
			s.logger.Errorf(fmt, args...)
		}
	case sklogimpl.Fatal:
		if fmt == "" {
			s.logger.Fatal(args...)
		} else {
			s.logger.Fatalf(fmt, args...)
		}
	default:
		s.logger.Errorf(fmt, args...)
	}
}

// flush implements sklogimpl.Logger.
func (s stdlog) Flush() {
	// noop
}
