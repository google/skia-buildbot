package tracestore

import (
	"context"

	"go.skia.org/infra/go/paramtools"
)

// TraceParamStore provides an interface for operating on the TraceParams table.
type TraceParamStore interface {
	// WriteTraceParams writes the given trace params into the table. The key for the
	// traceParams is the traceId (hex-encoded form of the md5 hash of the trace name)
	// and the value is the corresponding params.
	WriteTraceParams(ctx context.Context, traceParams map[string]paramtools.Params) error

	// ReadParams reads the parameters for the given set of traceIds.
	ReadParams(ctx context.Context, traceIds []string) (map[string]paramtools.Params, error)
}
