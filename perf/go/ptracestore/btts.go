package ptracestore

import (
	"time"

	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/cid"
)

// BTStoreShim implements PTraceStore using btts.BigTableTraceStore.
//
// It is a shim that we'll use as we migrate from Bolt to BigTable.
type BTStoreShim struct {
	b *btts.BigTableTraceStore
}

// Add new values to the datastore at the given commitID.
//
// values - A map from the trace id to a float32 value.
// sourceFile - The full path of the file where this information came from,
//   usually the Google Storage URL.
func (shim *BTStoreShim) Add(commitID *cid.CommitID, values map[string]float32, sourceFile string, ts time.Time) error {
	return shim.b.WriteTraces(commitID.Offset, values, sourceFile, ts)
}

// Retrieve the source and value for a given measurement in a given trace,
// and a non-nil error if no such point was found.
//
// This implementation always returns vec32.MISSING_DATA_SENTINEL, which is fine since
// in Perf we always ignore that part of the return value.
func (shim *BTStoreShim) Details(commitID *cid.CommitID, traceID string) (string, float32, error) {
	tileKey := shim.b.TileKey(commitID.Offset)
	source, err := shim.b.GetSource(commitID.Offset, tileKey)
	return source, vec32.MISSING_DATA_SENTINEL, err
}

// Match returns TraceSet that match the given Query and slice of cid.CommitIDs.
//
// The 'progess' callback will be called as each Tile is processed.
//
// The returned TraceSet will contain a slice of Trace, and that list will be
// empty if there are no matches.
func (shim *BTStoreShim) Match(commitIDs []*cid.CommitID /* AAAAAAAARRGH!!! */, matches KeyMatches, progress Progress) (TraceSet, error) {
}
