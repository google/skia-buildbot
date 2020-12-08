// Package sources defines the Sources interface.
package sources

import "context"

// Sources defines an interface for finding all the source filenames from the
// previous N commits for the given samples.
//
// That is, for each point in a trace we also store the 'source' filename in the
// database, which is the JSON file that was ingested to produce that point.
// Once we know that source file we can go back and load the 'samples' from that
// file by reloading it from storage.
type Sources interface {
	// Load returns the source filenames of the 'n' most recent commits that
	// contain samples for the given trace ids.
	//
	// Starting with one trace id, load up the last N commits for that trace,
	// and then find the name of the source JSON files that contains those
	// points. Repeat until all the traceIDs passed in are accounted for by 'n'
	// source files.
	Load(ctx context.Context, traceIDs []string, n int) ([]string, error)
}
