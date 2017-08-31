package gtracestore

// type DB interface {
// 	Add(commitID *CommitID, values map[string]*Entry) error

// 	// Remove all info for the given commit.
// 	Remove(commitID *CommitID) error

// 	// List returns all the CommitID's between begin and end.
// 	List(begin, end time.Time) ([]*CommitID, error)

// 	// Create a Tile for the given commit ids. Will build the Tile using the
// 	// commits in the order they are provided.
// 	//
// 	// Note that the Commits in the Tile will only contain the commit id and
// 	// the timestamp, the Author will not be populated.
// 	//
// 	// The Tile's Scale and TileIndex will be set to 0.
// 	//
// 	// The md5 hashes for each commitid are also returned.
// 	TileFromCommits(commitIDs []*CommitID) (*tiling.Tile, []string, error)

// 	// ListMD5 returns the md5 hashes of the data stored for each commitid.
// 	ListMD5(commitIDs []*CommitID) ([]string, error)

// 	// Close the datastore.
// 	Close() error
// }

type CommitID struct {
	Timestamp int64  `json:"ts"`
	ID        string `json:"id"`     // Normally a git hash, but could also be Rietveld patch id.
	Source    string `json:"source"` // The branch name, e.g. "master", or the Reitveld issue id.
}

type Entry struct {
	Params map[string]string
	Value  []byte
}
