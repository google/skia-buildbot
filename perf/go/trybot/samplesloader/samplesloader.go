package samplesloader

import (
	"context"

	"go.skia.org/infra/perf/go/ingest/parser"
)

// SamplesLoader loads all the samples from storage for the given filename.
type SamplesLoader interface {
	// Load loads all the samples from storage for the given filename.
	Load(ctx context.Context, filename string) (parser.SamplesSet, error)
}
