package tracestore

import (
	"context"
	"time"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

// Entry is one digests and related params to be added to the TraceStore.
type Entry struct {
	// Params describe the configuration that produced the digest/image.
	Params map[string]string

	// Digest references the images that were generate by the test.
	Digest types.Digest
}

// TraceStore is the interface to store trace data.
type TraceStore interface {
	// Put writes the given entries to the TraceStore at the given commit hash. The timestamp is
	// assumed to be the time when the entries were generated.
	Put(ctx context.Context, commitHash string, entries []*Entry, ts time.Time) error

	// GetTile reads the last n commits and returns them as a tile. If isSparse is true
	// empty commits are omitted.
	GetTile(ctx context.Context, nCommits int, isSparse bool) (*tiling.Tile, []*tiling.Commit, []int, error)
}
