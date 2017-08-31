package gtracestore

import (
	"time"

	"go.skia.org/infra/go/tiling"
)

type TraceStore interface {
	Add(*CommitID, map[string]*Entry) error

	// List returns all the CommitID's between begin and end.
	List(begin, end time.Time) ([]*CommitID, error)

	// Create a Tile for the given commit ids. Will build the Tile using the
	// commits in the order they are provided.
	//
	// Note that the Commits in the Tile will only contain the commit id and
	// the timestamp, the Author will not be populated.
	//
	// The Tile's Scale and TileIndex will be set to 0.
	//
	// The md5 hashes for each commitid are also returned.
	TileFromCommits(commitIDs []*CommitID) (*tiling.Tile, []string, error)

	// Close the datastore.
	Close() error
}

func NewTraceStoreFromAddress(address string) (TraceStore, error) {
	return nil, nil
}

type BoltTraceStore struct{}

func (t *BoltTraceStore) Add(commitID CommitID, values map[string]*Entry) error {
	return nil
}

// Remove all info for the given commit.
func (t *BoltTraceStore) Remove(commitID *CommitID) error {
	return nil
}

// List returns all the CommitID's between begin and end.
func (t *BoltTraceStore) List(begin, end time.Time) ([]*CommitID, error) {
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
func (t *BoltTraceStore) TileFromCommits(commitIDs []*CommitID) (*tiling.Tile, []string, error) {
	return nil, nil, nil
}

// ListMD5 returns the md5 hashes of the data stored for each commitid.
func (t *BoltTraceStore) ListMD5(commitIDs []*CommitID) ([]string, error) {
	return nil, nil
}

// Close the datastore.
func (t *BoltTraceStore) Close() error {
	return nil
}
