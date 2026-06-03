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

	// UpdateVisibility updates the is_public flag for the given traceIds.
	UpdateVisibility(ctx context.Context, traceIds []string, isPublic bool) error

	// GetInternalTraceIDsForParam returns a list of trace IDs that match a given parameter key and value, and are currently marked private (is_public = false).
	GetInternalTraceIDsForParam(ctx context.Context, paramName string, paramValue string) ([]string, error)

	// GetPublicTraces returns a map of all trace IDs that are currently public to their parameters.
	GetPublicTraces(ctx context.Context) (map[string]paramtools.Params, error)

	// ReadParams reads the parameters for the given set of traceIds.
	ReadParams(ctx context.Context, traceIds []string) (map[string]paramtools.Params, error)
}
