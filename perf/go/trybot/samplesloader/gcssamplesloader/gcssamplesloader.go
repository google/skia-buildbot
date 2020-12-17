// Package gcssamplesloader implements samplesloader.SamplesLoader for Google Cloud Storage.
package gcssamplesloader

import (
	"context"
	"net/url"

	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/ingest/parser"
	"go.skia.org/infra/perf/go/trybot/samplesloader"
)

// loader implements samplesloader.SamplesLoader.
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

// Load implements samplesloader.SamplesLoader.
func (l *loader) Load(ctx context.Context, filename string) (parser.SamplesSet, error) {
	// filename is the absolute URL to the file, e.g.
	// "gs://bucket/path/name.json", so we need to parse it since
	// storageClient only takes the path.
	u, err := url.Parse(filename)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse filename: %q", filename)
	}

	// Load the source file.
	rc, err := l.storageClient.FileReader(ctx, u.Path[1:])
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to load from storage: %q", filename)
	}
	defer util.Close(rc)

	benchData, err := format.ParseLegacyFormat(rc)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse samples from file: %q", filename)
	}
	return parser.GetSamplesFromLegacyFormat(benchData), nil
}

// Affirm we implement samplesloader.SamplesLoader.
var _ samplesloader.SamplesLoader = (*loader)(nil)
