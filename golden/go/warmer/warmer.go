// warmer makes sure we've pre-warmed the cache for normal queries.
//
// This is a quick fix until filediffstore does full NxM diffs for every
// untriaged digest by default.
package warmer

import (
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/golden/go/digesttools"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/summary"
	"go.skia.org/infra/golden/go/types"
)

func Init(storages *storage.Storage, summaries *summary.Summaries) error {
	exp, err := storages.ExpectationsStore.Get()
	if err != nil {
		return err
	}
	go func() {
		for _ = range time.Tick(time.Minute) {
			t := timer.New("warmer one loop")
			for test, sum := range summaries.Get() {
				for _, digest := range sum.UntHashes {
					// Calculate the closest digest for the side effect of filling in the filediffstore cache.
					digesttools.ClosestDigest(test, digest, exp, storages.DiffStore, types.POSITIVE)
					digesttools.ClosestDigest(test, digest, exp, storages.DiffStore, types.NEGATIVE)
				}
			}
			t.Stop()
			if newExp, err := storages.ExpectationsStore.Get(); err != nil {
				glog.Errorf("warmer: Failed to get expectations: %s", err)
			} else {
				exp = newExp
			}
		}
	}()
	return nil
}
