package storage

import (
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digeststore"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	ptypes "go.skia.org/infra/perf/go/types"
)

// Storage is a container struct for the various storage objects we are using.
// It is intended to reduce parameter lists as we pass around storage objects.
type Storage struct {
	DiffStore         diff.DiffStore
	ExpectationsStore expstorage.ExpectationsStore
	IgnoreStore       ignore.IgnoreStore
	TileStore         ptypes.TileStore
	DigestStore       digeststore.DigestStore

	// NCommits is the number of commits we should consider. If NCommits is
	// 0 or smaller all commits in the last tile will be considered.
	NCommits int

	// Internal variables used to cache trimmed tiles.
	lastTrimmedTile *ptypes.Tile
	lastBaseTile    *ptypes.Tile
	mutex           sync.Mutex
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

// GetLastTrimmed returns the last tile as read-only trimmed to contain at
// most NCommits. It caches trimmed tiles as long as the underlying tiles
// do not change.
func (s *Storage) GetLastTileTrimmed() (*ptypes.Tile, error) {
	// Get the last (potentially cached) tile.
	tile, err := s.TileStore.Get(0, -1)
	if err != nil {
		return nil, err
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.NCommits <= 0 {
		return tile, err
	}

	// Check if the tile has changed.
	if tile == s.lastBaseTile {
		return s.lastTrimmedTile, nil
	}

	tileLen := tile.LastCommitIndex() + 1
	retTile, err := tile.Trim(util.MaxInt(0, tileLen-s.NCommits), tileLen)
	if err != nil {
		return nil, err
	}

	// Cache this tile.
	s.lastTrimmedTile = retTile
	s.lastBaseTile = tile

	return retTile, err
}
