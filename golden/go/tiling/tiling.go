package tiling

import (
	"time"

	"go.skia.org/infra/golden/go/types"
)

// TraceID helps document when strings should represent ids of traces
type TraceID string

// Commit is information about each Git commit.
// TODO(kjlubick) Why does this need to have its own type? Can't it use one of the other Commit
//   types?
type Commit struct {
	CommitTime time.Time
	Hash       string
	Author     string
	Subject    string
}

// Tile is a config.TILE_SIZE commit slice of data.
//
// The length of the Commits array is the same length as all of the Values
// arrays in all of the Traces.
type Tile struct {
	Traces   map[TraceID]*Trace
	ParamSet map[string][]string
	Commits  []Commit
}

// LastCommitIndex returns the index of the last valid Commit.
func (t *Tile) LastCommitIndex() int {
	for i := len(t.Commits) - 1; i > 0; i-- {
		if !t.Commits[i].CommitTime.IsZero() {
			return i
		}
	}
	return 0
}

const (
	// MissingDigest is a sentinel value meaning no digest is available at the given commit.
	MissingDigest = types.Digest("")
)

// TracePair represents a single Golden trace and its ID. A slice of TracePair is faster to
// iterate over than a map of TraceID -> Trace
type TracePair struct {
	ID    TraceID
	Trace *Trace
}
