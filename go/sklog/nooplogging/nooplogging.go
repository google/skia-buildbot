// Package nooplogging has an sklogimpl.Logger that does no logging.
package nooplogging

import (
	"os"

	"go.skia.org/infra/go/sklog/sklogimpl"
)

type nooplog struct {
}

// New returns a sklogimpl.Logger that emits no logs.
//
// It does exit on a Fatal log.
func New() sklogimpl.Logger {
	return nooplog{}
}

// Log implements sklogimpl.Logger.
func (s nooplog) Log(_ int, severity sklogimpl.Severity, fmt string, args ...interface{}) {
	if severity == sklogimpl.Fatal {
		os.Exit(255)
	}
}

// flush implements sklogimpl.Logger.
func (s nooplog) Flush() {
	// noop
}
