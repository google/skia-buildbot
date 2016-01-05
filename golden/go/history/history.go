package history

import (
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/metrics"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/digeststore"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

// Init initializes the history module and starts background processes to
// continuously update information about digests.
// If nTilesToBackfill is larger than zero, the given number of tiles will
// be traversed to backfill information about digests.
func Init(storages *storage.Storage, nTilesToBackfill int) error {
	var err error
	_, err = newHistorian(storages, nTilesToBackfill)
	return err
}

// CanonicalDigests returns the cannonical digests for a list of test names.
// The canonical digest is the last labeled digest in the current tile that
// is in the canonical trace. If no canonical trace is defined or the
// current tile has only untriaged digests an empty string is returned.
func CanonicalDigests(testNames []string) (map[string]string, error) {
	// TODO(stephana): Implement once the API is defined.
	return nil, nil
}

// historian runs background processes to update information about digests,
// i.e. recording when we encounter a digests for the first and the last time.
type historian struct {
	storages *storage.Storage
}

// Create a new instance of historian.
func newHistorian(storages *storage.Storage, nDaysToBackfill int) (*historian, error) {
	defer timer.New("historian").Stop()

	ret := &historian{
		storages: storages,
	}

	// Start running the background process to gather digestinfo.
	if err := ret.start(); err != nil {
		return nil, err
	}

	// If there is at least one day to backfill then start the process.
	if nDaysToBackfill > 0 {
		ret.backFillDigestInfo(nDaysToBackfill)
	}

	return ret, nil
}

func (h *historian) start() error {
	expChanges := make(chan []string)
	h.storages.EventBus.SubscribeAsync(expstorage.EV_EXPSTORAGE_CHANGED, func(e interface{}) {
		expChanges <- e.([]string)
	})
	tileStream := h.storages.GetTileStreamNow(2*time.Minute, true)

	lastTile := <-tileStream
	if err := h.updateDigestInfo(lastTile); err != nil {
		return err
	}
	liveness := metrics.NewLiveness("digest-history-monitoring")

	// Keep processing tiles and feed them into the process channel.
	go func() {
		for {
			select {
			case tile := <-tileStream:
				if err := h.updateDigestInfo(tile); err != nil {
					glog.Errorf("Error calculating status: %s", err)
					continue
				} else {
					lastTile = tile
				}
			case <-expChanges:
				storage.DrainChangeChannel(expChanges)
				if err := h.updateDigestInfo(lastTile); err != nil {
					glog.Errorf("Error calculating tile after expectation udpate: %s", err)
					continue
				}
			}
			liveness.Update()
		}
	}()

	return nil
}

func (h *historian) updateDigestInfo(tile *tiling.Tile) error {
	return h.processTile(tile)
}

func (h *historian) backFillDigestInfo(nDaysToBackfill int) {
	go func() {
		startTS := time.Now().Add(time.Hour * 24 * time.Duration(nDaysToBackfill))
		endTS := time.Now()
		tile, err := h.storages.GetTileFromTimeRange(startTS, endTS)
		if err != nil {
			glog.Errorf("Error retrieving tile for range %s - %s: %s", startTS, endTS, err)
			return
		}

		// Process the tile, but giving higher priority to digests from the
		// latest tile.
		if err = h.processTile(tile); err != nil {
			glog.Errorf("Error processing tile: %s", err)
		}
	}()
}

func (h *historian) processTile(tile *tiling.Tile) error {
	dStore := h.storages.DigestStore
	tileLen := tile.LastCommitIndex() + 1

	var digestInfo *digeststore.DigestInfo
	var ok bool
	counter := 0
	minMaxTimes := map[string]map[string]*digeststore.DigestInfo{}
	for _, trace := range tile.Traces {
		gTrace := trace.(*types.GoldenTrace)
		testName := trace.Params()[types.PRIMARY_KEY_FIELD]
		for idx, digest := range gTrace.Values[:tileLen] {
			if digest != types.MISSING_DIGEST {
				timeStamp := tile.Commits[idx].CommitTime
				if digestInfo, ok = minMaxTimes[testName][digest]; !ok {
					digestInfo = &digeststore.DigestInfo{
						TestName: testName,
						Digest:   digest,
						First:    timeStamp,
						Last:     timeStamp,
					}

					if testVal, ok := minMaxTimes[testName]; !ok {
						minMaxTimes[testName] = map[string]*digeststore.DigestInfo{digest: digestInfo}
					} else {
						testVal[digest] = digestInfo
					}
					counter++
				} else {
					digestInfo.First = util.MinInt64(digestInfo.First, timeStamp)
					digestInfo.Last = util.MaxInt64(digestInfo.Last, timeStamp)
				}
			}
		}
	}

	digestInfos := make([]*digeststore.DigestInfo, 0, counter)
	for _, digests := range minMaxTimes {
		for _, digestInfo := range digests {
			digestInfos = append(digestInfos, digestInfo)
		}
	}

	return dStore.Update(digestInfos)
}
