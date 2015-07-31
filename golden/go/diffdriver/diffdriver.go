package diffdriver

import (
	"time"

	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

// Init starts a back ground process that feeds test and digest information
// to the diffstore to continously calculate diffs for new digests.
func Init(storages *storage.Storage) {
	go func() {
		// TODO(stephana): Once we have events that signal that a new tile
		// is available, we want to process new tiles immediately instead
		// of polling every so often.
		tileStream := storages.GetTileStreamNow(2*time.Minute, true)

		for {
			tile := <-tileStream
			tileLen := tile.LastCommitIndex() + 1

			// digestSets is a map [testname] map [diget] bool.
			digestSets := map[string]map[string]bool{}
			for _, trace := range tile.Traces {
				gTrace := trace.(*types.GoldenTrace)
				testName := trace.Params()[types.PRIMARY_KEY_FIELD]
				for _, digest := range gTrace.Values[:tileLen] {
					if digest != types.MISSING_DIGEST {
						if _, ok := digestSets[testName]; !ok {
							digestSets[testName] = map[string]bool{}
						}
						digestSets[testName][digest] = true
					}
				}
			}

			storages.DiffStore.SetDigestSets(digestSets)
		}
	}()
}
