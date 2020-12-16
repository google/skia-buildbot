package fileloader

import (
	"context"

	"go.skia.org/infra/perf/go/ingest/parser"
)

// FileLoader loads all the samples from storage for the given filename.
type FileLoader interface {
	// GetSamples loads all the samples from storage for the given filename.
	GetSamples(ctx context.Context, filename string) (parser.SamplesSet, error)
}
