// search contains the core functionality for searching for digests across a tile.
package search

import (
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

// AcceptFn is the callback function used by iterTile to determine whether to
// include a trace into the result or not. The second return value is an
// intermediate result that will be passed to addFn to avoid redundant computation.
// The second return value is application dependent since it will be passed
// into the call to the corresponding AddFn. Determining whether to accept a
// result might require an expensive computation and we want to avoid repeating
// that computation in the 'add' step. So we can return it here and it will
// be passed into the instance of AddFn.
type AcceptFn func(trace *types.GoldenTrace, digests []string) (bool, interface{})

// AddFn is the callback function used by iterTile to add a digest and it's
// trace to the result. acceptResult is the same value returned by the AcceptFn.
type AddFn func(test, digest, traceID string, trace *types.GoldenTrace, acceptResult interface{})

// iterTile is a generic function to extract information from a tile.
// It iterates over the tile and filters against the given query. If calls
// acceptFn to determine whether to keep a trace (after it has already been
// tested against the query) and calls addFn to add a digest and its trace.
// acceptFn == nil equals unconditional acceptance.
func iterTile(query *Query, addFn AddFn, acceptFn AcceptFn, storages *storage.Storage, idx *indexer.SearchIndex) error {
	exp, err := storages.ExpectationsStore.Get()
	if err != nil {
		return err
	}

	tile := idx.GetTile(query.IncludeIgnores)

	if acceptFn == nil {
		acceptFn = func(tr *types.GoldenTrace, digests []string) (bool, interface{}) { return true, nil }
	}

	traceTally := idx.TalliesByTrace(query.IncludeIgnores)
	lastCommitIndex := tile.LastCommitIndex()

	// Iterate through the tile.
	for id, trace := range tile.Traces {
		// Check if the query matches.
		if tiling.Matches(trace, query.Query) {
			tr := trace.(*types.GoldenTrace)
			digests := digestsFromTrace(id, tr, query.Head, lastCommitIndex, traceTally)

			// If there is an acceptFn defined then check whether
			// we should include this trace.
			var acceptRet interface{} = nil
			var ok bool
			if acceptFn != nil {
				if ok, acceptRet = acceptFn(tr, digests); !ok {
					continue
				}
			}

			// Iterate over the digess and filter them.
			test := tr.Params()[types.PRIMARY_KEY_FIELD]
			for _, digest := range digests {
				cl := exp.Classification(test, digest)
				if query.excludeClassification(cl) {
					continue
				}

				// Fix blamer to make this easier.
				if query.BlameGroupID != "" {
					if cl == types.UNTRIAGED {
						b := idx.GetBlame(test, digest, tile.Commits)
						if query.BlameGroupID != blameGroupID(b, tile.Commits) {
							continue
						}
					} else {
						continue
					}
				}

				// Add the digest to the results.
				addFn(test, digest, id, tr, acceptRet)
			}
		}
	}
	return nil
}
