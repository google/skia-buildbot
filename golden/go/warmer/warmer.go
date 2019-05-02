// warmer makes sure we've pre-warmed the cache for normal queries.
//
// This is a quick fix until filediffstore does full NxM diffs for every
// untriaged digest by default.
package warmer

import (
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digesttools"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/summary"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/types"
)

// TODO(stephana): This should be folded into the rewritten FileDiffStore.

// Warmer continously prefetches images and calculates diffs that are likely to
// be requested by the front-end.
type Warmer struct {
	storages *storage.Storage
}

// New creates an new instance of warmer.
func New(storages *storage.Storage) *Warmer {
	return &Warmer{
		storages: storages,
	}
}

// Run prefetches the digests in tile and calculates differences we'll need.
func (w *Warmer) Run(tile *tiling.Tile, summaries *summary.Summaries, tallies *tally.Tallies) {
	exp, err := w.storages.ExpectationsStore.Get()
	if err != nil {
		sklog.Errorf("warmer: Failed to get expectations: %s", err)
	}

	t := timer.New("warmer one loop")
	for test, sum := range summaries.Get() {
		for _, digest := range sum.UntHashes {
			t := tallies.ByTest()[test]
			if t != nil {
				// Calculate the closest digest for the side effect of filling in the filediffstore cache.
				digesttools.ClosestDigest(test, digest, exp, t, w.storages.DiffStore, types.POSITIVE)
				digesttools.ClosestDigest(test, digest, exp, t, w.storages.DiffStore, types.NEGATIVE)
			}
		}
	}
	t.Stop()

	// Make sure all images are downloaded. This is necessary, because
	// the front-end doesn't get URLs (generated via DiffStore.AbsPath)
	// which ensures that the image has been downloaded.
	// TODO(stephana): Remove this once the new diffstore is in place.
	tileLen := tile.LastCommitIndex() + 1
	traceDigests := make(util.StringSet, tileLen)
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		for _, digest := range gTrace.Digests {
			if digest != types.MISSING_DIGEST {
				traceDigests[digest] = true
			}
		}
	}

	digests := traceDigests.Keys()
	sklog.Infof("FOUND %d digests to fetch.", len(digests))
	w.storages.DiffStore.WarmDigests(diff.PRIORITY_BACKGROUND, digests, false)

	// TODO(stephana): Add warmer for Tryjob digests.
}
