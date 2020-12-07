// Package gcssources implements Sources.
package gcssources

import (
	"context"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/trybot/sources"
	"go.skia.org/infra/perf/go/types"
)

// impl implements sources.Sources.
type impl struct {
	traceStore tracestore.TraceStore

	// TODO(jcgregorio) Migrate this to a generic filesystem interface once available.
	storageClient *storage.Client
}

// New returns a new instance of a sources.Sources implementation.
func New(traceStore tracestore.TraceStore, storageClient *storage.Client) *impl {
	return &impl{
		traceStore:    traceStore,
		storageClient: storageClient,
	}
}

// Load implements sources.Sources.
func (s *impl) Load(ctx context.Context, traceIDs []string, n int) ([]string, error) {
	ret := []string{}
	remainingTraceIDs := make([]string, len(traceIDs))
	copy(remainingTraceIDs, traceIDs)

	tileNumber, err := s.traceStore.GetLatestTile(ctx)
	if err != nil {
		return ret, skerr.Wrap(err)
	}
	lastCommit := types.CommitNumber(int32(tileNumber)*s.traceStore.TileSize() - 1)

	for {
		if len(remainingTraceIDs) == 0 {
			break
		}
		nextTrace := remainingTraceIDs[0]
		remainingTraceIDs = remainingTraceIDs[1:]

		readLength := s.traceStore.TileSize()

		commitNumbers := []types.CommitNumber{}

		for {
			begin := types.CommitNumber(int32(lastCommit) - readLength - 1)
			end := lastCommit
			traceSet, err := s.traceStore.ReadTracesForCommitRange(ctx, []string{nextTrace}, begin, end)
			if err != nil {
				return ret, skerr.Wrap(err)
			}
			trace, ok := traceSet[nextTrace]
			if !ok {
				continue
			}
			for i := len(trace) - 1; i > 0; i-- {
				if trace[i] != vec32.MissingDataSentinel {
					commitNumbers = append(commitNumbers, begin+types.CommitNumber(i))
					if len(commitNumbers) >= n {
						break
					}
				}
			}

			// Now find the sources for each commitNumber for traceID = nextTrace.

			// We should also loop and increase readLength and modify lastCommit
			// if we dont' find enough commits.
		}
	}

	return ret, nil
}

// Confirm sources implements sources.Sources.
var _ sources.Sources = (*impl)(nil)
