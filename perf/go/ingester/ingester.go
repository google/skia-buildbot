// Package ingester provides an interface that all sources of raw Perf data to
// be ingested must implement.
package ingester

import (
	"context"
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/perf/go/types"
)

// File represents the parsed measurements found in a single file being
// ingested.
//
// Params and Values are parallel slices that map 1:1 to each other. That is,
// Params[n] contains all the key=value pairs that make up the trace name of a
// measurement, and Values[n] is the value of that measurement.
type File struct {
	CommitNumber types.CommitNumber
	Params       []paramtools.Params
	Values       []float64
	Filename     string    // The full name of the file, can be a URL, e.g. gs://bucket/foo.json.
	Timestamp    time.Time // The timestamp of the file, i.e. when it was created.
}

// Source is the interface that all sources of raw Perf data to be ingested must
// implement.
//
// The channel returned provides a stream of Files for each file ingested.
type Source interface {
	Start(context.Context) (<-chan File, error)
}
