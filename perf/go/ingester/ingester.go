// Package ingester provides an interface that all sources of raw Perf data to
// be ingested must implement.
package ingester

import (
	"context"
	"io"
	"time"

	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/tracestore"
)

// SingleFileProcessor is the type of callback that Source takes. It must be threadsafe.
type SingleFileProcessor func(ctx context.Context, store tracestore.TraceStore, vcs vcsinfo.VCS, filename string, r io.Reader, timestamp time.Time, branches []string) error

// Source is the interface that all sources of raw Perf data to be ingested must
// implement.
type Source interface {
	Callback(SingleFileProcessor)
}
