// Package gcssources implements Sources.
package gcssources

import (
	"context"
	"net/url"

	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/file"
	"go.skia.org/infra/perf/go/ingest/parser"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/trybot/sources"
	"go.skia.org/infra/perf/go/types"
)

// Only search back this number of iterations looking for values in the
// tracestore.
const maxIterations = 5

// srcs implements sources.Sources.
type srcs struct {
	traceStore    tracestore.TraceStore
	storageClient gcs.GCSClient
	parser        *parser.Parser
}

// New returns a new instance of a sources.Sources implementation.
func New(traceStore tracestore.TraceStore, storageClient gcs.GCSClient, parser *parser.Parser) *srcs {
	return &srcs{
		traceStore:    traceStore,
		storageClient: storageClient,
		parser:        parser,
	}
}

// Load implements sources.Sources.
func (s *srcs) Load(ctx context.Context, traceIDs []string, n int) ([]string, error) {
	filenames := util.StringSet{}

	// Make a copy of traceIDs since we'll be modifying it.
	remainingTraceIDs := util.StringSet{}
	for _, traceID := range traceIDs {
		remainingTraceIDs[traceID] = true
	}

	// Determine the last possible commit stored in the tracestore.
	tileNumber, err := s.traceStore.GetLatestTile(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	lastCommit := types.CommitNumber(int32(tileNumber)*s.traceStore.TileSize() - 1)

	for {
		// Keep looping until we've tried to find a set of source files for each
		// traceID in remainingTraceIDs.
		if len(remainingTraceIDs) == 0 {
			break
		}

		// Pop off the next trace id from remainingTraces.
		currentTraceID := remainingTraceIDs.Keys()[0]
		delete(remainingTraceIDs, currentTraceID)

		// Find the n most recent commits for that traceid.
		commitNumbers := []types.CommitNumber{}

		// Cast back further and further back into the trace values looking for
		// commits that have data. We do this because some instances of Perf
		// have sparse data, i.e. data only arrives on some commits.
		it, err := newIter(lastCommit, s.traceStore.TileSize(), maxIterations)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		for it.Next() {
			begin, end := it.Range()
			traceSet, err := s.traceStore.ReadTracesForCommitRange(ctx, []string{currentTraceID}, begin, end)
			if err != nil {
				return nil, skerr.Wrap(err)
			}

			// Find commits that have data.
			trace, ok := traceSet[currentTraceID]
			if !ok {
				continue
			}

			// Search the trace values in reverse, because we want to use newer
			// commits over older commits.
			for i := len(trace) - 1; i > 0; i-- {
				if trace[i] != vec32.MissingDataSentinel {
					commitNumbers = append(commitNumbers, begin.Add(int32(i)))
					if len(commitNumbers) >= n {
						break
					}
				}
			}
		}

		firstSourceFileName := ""
		// Now find the source filenames for each commitNumber and keep track of
		// the first source file name we encounter, which will be the most
		// recent commit with data.
		for i, commitNumber := range commitNumbers {
			sourceFilename, err := s.traceStore.GetSource(ctx, commitNumber, currentTraceID)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			filenames[sourceFilename] = true
			if i == 0 {
				firstSourceFileName = sourceFilename
			}
		}

		// If we found firstSourceFileName then load that file and remove all
		// the trace ids found in it from remainingTraceIDs.
		if firstSourceFileName != "" {
			// source is the absolute URL to the file, e.g.
			// "gs://bucket/path/name.json", so we need to parse it since
			// storageClient only takes the path.
			u, err := url.Parse(firstSourceFileName)
			if err != nil {
				return nil, skerr.Wrap(err)
			}

			// Load the source file.
			rc, err := s.storageClient.FileReader(ctx, u.Path)
			if err != nil {
				return nil, skerr.Wrap(err)
			}

			// Parse it.
			params, _, _, err := s.parser.Parse(file.File{
				Name:     firstSourceFileName,
				Contents: rc,
			})
			if err != nil {
				return nil, skerr.Wrap(err)
			}

			// Remove all the traceids it contains from remainingTraces, since
			// the firstSourceFileName we have covers those traceids also.
			for _, p := range params {
				traceID, err := query.MakeKeyFast(p)
				if err != nil {
					return nil, skerr.Wrap(err)
				}
				delete(remainingTraceIDs, traceID)
			}
		}
	}

	return filenames.Keys(), nil
}

// Confirm sources implements sources.Sources.
var _ sources.Sources = (*srcs)(nil)
