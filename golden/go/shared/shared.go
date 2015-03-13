package shared

import (
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digeststore"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
	ptypes "go.skia.org/infra/perf/go/types"
)

// Storage is a container struct for the various storage objects we are using.
// It is intended to reduce parameter lists as we pass around storage objects.
type Storage struct {
	DiffStore         diff.DiffStore
	ExpectationsStore expstorage.ExpectationsStore
	IgnoreStore       types.IgnoreStore
	TileStore         ptypes.TileStore
	DigestStore       digeststore.DigestStore
}

// GetTileStreamNow is a utility function that reads tiles from the given
// TileStore in the given interval and sends them on the returned channel.
// The first tile is send immediately.
// Should the call to read a new tile fail it will send that last
// successfully read tile. Thus it guarantees to send a tile in the provided
// interval, assuming at least one tile could be read.
func GetTileStreamNow(tileStore ptypes.TileStore, interval time.Duration) <-chan *ptypes.Tile {
	retCh := make(chan *ptypes.Tile)

	go func() {
		var lastTile *ptypes.Tile = nil

		readOneTile := func() {
			if tile, err := tileStore.Get(0, -1); err != nil {
				// Log the error and send the best tile we have right now.
				glog.Errorf("Error reading tile: %s", err)
				if lastTile != nil {
					retCh <- lastTile
				}
			} else {
				lastTile = tile
				retCh <- tile
			}
		}

		readOneTile()
		for _ = range time.Tick(interval) {
			readOneTile()
		}
	}()

	return retCh
}

// DrainChangeChannel removes everything from the channel thats currently
// buffered or ready to be read.
func DrainChangeChannel(ch <-chan []string) {
Loop:
	for {
		select {
		case <-ch:
		default:
			break Loop
		}
	}
}
