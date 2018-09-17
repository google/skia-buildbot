// Package pstracestore is a database for Perf data.
package ptracestore

import (
	"errors"
	"time"

	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/cid"
)

const (
	MAX_CACHED_TILES = 20

	TRACE_VALUES_BUCKET_NAME  = "traces"
	TRACE_SOURCES_BUCKET_NAME = "sources"
	SOURCE_LIST_BUCKET_NAME   = "sourceList"
)

var (
	// tileNotExist is returned from getBoltDB only if 'readonly' is true and
	// the tile doesn't exist.
	tileNotExist = errors.New("Tile does not exist.")
)

// Trace is just a slice of float32s.
type Trace []float32

// NewTrace returns a Trace of length 'traceLen' initialized to vec32.MISSING_DATA_SENTINEL.
func NewTrace(traceLen int) Trace {
	ret := make([]float32, traceLen)
	for i := range ret {
		ret[i] = vec32.MISSING_DATA_SENTINEL
	}
	return ret
}

// TraceSet is a set of Trace's, keyed by trace id.
type TraceSet map[string]Trace

// Progress is a func that is called as Match works, passing in the steps
// completed and the total number of steps, where a 'step' is applying a query
// to a single tile.
type Progress func(step, totalSteps int)

// KeyMatches is a func that returns true if a key matches some criteria.
// Passed to Match().
type KeyMatches func(key string) bool

// PTraceStore is an interface for storing Perf data.
//
// PTraceStore doesn't know anything about git hashes or code review issue IDs,
// that will be handled at a level above this.
//
type PTraceStore interface {
	// Add new values to the datastore at the given commitID.
	//
	// values - A map from the trace id to a float32 value.
	// sourceFile - The full path of the file where this information came from,
	//   usually the Google Storage URL.
	Add(commitID *cid.CommitID, values map[string]float32, sourceFile string, ts time.Time) error

	// Retrieve the source and value for a given measurement in a given trace,
	// and a non-nil error if no such point was found.
	Details(commitID *cid.CommitID, traceID string) (string, float32, error)

	// Match returns TraceSet that match the given Query and slice of cid.CommitIDs.
	//
	// The 'progess' callback will be called as each Tile is processed.
	//
	// The returned TraceSet will contain a slice of Trace, and that list will be
	// empty if there are no matches.
	Match(commitIDs []*cid.CommitID, matches KeyMatches, progress Progress) (TraceSet, error)
}
