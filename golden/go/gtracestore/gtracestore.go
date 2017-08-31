package gtracestore

import (
	"time"

	"go.skia.org/infra/go/tiling"
)

func NewTraceStoreFromAddress(address string, engine GTSEngine) TraceStore {
	return &traceStoreImpl{
		engine: engine,
	}
}

type GTSEngine interface {
}

type traceStoreImpl struct {
	engine GTSEngine
}

func (t *traceStoreImpl) Add(commitID *CommitID, values map[string]*Entry) error {
	// Translate the new record to an integer representation.

	// Add it to the datastore.

	return nil
}

// Remove all info for the given commit.
func (t *traceStoreImpl) Remove(commitID *CommitID) error {
	return nil
}

// List returns all the CommitID's between begin and end.
func (t *traceStoreImpl) List(begin, end time.Time) ([]*CommitID, error) {
	return nil, nil
}

// Create a Tile for the given commit ids. Will build the Tile using the
// commits in the order they are provided.
//
// Note that the Commits in the Tile will only contain the commit id and
// the timestamp, the Author will not be populated.
//
// The Tile's Scale and TileIndex will be set to 0.
//
// The md5 hashes for each commitid are also returned.
func (t *traceStoreImpl) TileFromCommits(commitIDs []*CommitID) (*tiling.Tile, []string, error) {
	return nil, nil, nil
}

// Close the datastore.
func (t *traceStoreImpl) Close() error {
	return nil
}

// CompTile is a compressed tile.
type CompTile struct {
	nTraces  int32 // Number of traces.
	nCommits int32 // Number of commits.
	nParams  int32 // Number of parameters.
	nStrings int32 // Number of strings.

	// Parameters names as fixed with. 0 indicates an empty string. The length is the
	// maximum number of parameters.
	params          []int32 // This contains nParams elements.
	commits         []int32 // This contains nCommits elements.
	paramValuesBlob []int32 // This contains nParams x nTraces elements.
	tracesBlob      []int32 // This contains nTraces x nCommits elements.
	stringBounds    []int32 // Starting points of strings in stringsBlob. The size of this is nStrings.
	stringsBlob     []byte  // This contains an arbitrary number of bytes. And
	// is a concatenation of all strings. The values in
	// the other variables are indices in this slice.
}

func (c *CompTile) Tile() *tiling.Tile {
	return nil
}
