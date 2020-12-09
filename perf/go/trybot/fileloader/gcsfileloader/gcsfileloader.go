// Package gcsfileloader implements fileloader.FileLoader for Google Cloud Storage.
package gcsfileloader

import (
	"context"
	"net/url"

	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/ingest/parser"
	"go.skia.org/infra/perf/go/trybot/fileloader"
)

// loader implements fileloader.FileLoader.
type loader struct {
	storageClient gcs.GCSClient
	parser        *parser.Parser
}

// New returns a new loader instance.
func New(storageClient gcs.GCSClient, parser *parser.Parser) *loader {
	return &loader{
		storageClient: storageClient,
		parser:        parser,
	}
}

// GetSamples loads all the samples from storage for the given filename.
func (l *loader) GetSamples(ctx context.Context, filename string) (parser.SamplesSet, error) {
	// filename is the absolute URL to the file, e.g.
	// "gs://bucket/path/name.json", so we need to parse it since
	// storageClient only takes the path.
	u, err := url.Parse(filename)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse filename")
	}

	// Load the source file.
	rc, err := l.storageClient.FileReader(ctx, u.Path)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to load from storage")
	}
	defer util.Close(rc)

	benchData, err := format.ParseLegacyFormat(rc)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse samples from file")
	}
	return parser.GetSamplesFromLegacyFormat(benchData), nil
}

// Affirm we implement fileloader.FileLoader.
var _ fileloader.FileLoader = (*loader)(nil)
