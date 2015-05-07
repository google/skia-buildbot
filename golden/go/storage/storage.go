package storage

import (
	"fmt"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/eventbus"
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
	EventBus          *eventbus.EventBus

	// NCommits is the number of commits we should consider. If NCommits is
	// 0 or smaller all commits in the last tile will be considered.
	NCommits int

	// Internal variables used to cache trimmed tiles.
	lastTrimmedTile        *ptypes.Tile
	lastTrimmedIgnoredTile *ptypes.Tile
	lastBaseTile           *ptypes.Tile
	lastIgnoreRev          int64
	mutex                  sync.Mutex
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
//
// includeIgnores - If true then include ignored digests in the returned tile.
func (s *Storage) GetLastTileTrimmed(includeIgnores bool) (*ptypes.Tile, error) {
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

	currentIgnoreRev := s.IgnoreStore.Revision()

	// Check if the tile hasn't changed and the ignores haven't changed.
	if tile == s.lastBaseTile && s.lastTrimmedTile != nil && s.lastTrimmedIgnoredTile != nil && currentIgnoreRev == s.lastIgnoreRev {
		if includeIgnores {
			return s.lastTrimmedTile, nil
		} else {
			return s.lastTrimmedIgnoredTile, nil
		}
	}

	ignores, err := s.IgnoreStore.List()
	if err != nil {
		return nil, fmt.Errorf("Failed to get ignores to filter tile: %s", err)
	}

	// Build a new trimmed tile and a new trimmed tile with all ingoreable traces removed.
	tileLen := tile.LastCommitIndex() + 1

	// First build the new trimmed tile.
	retTile, err := tile.Trim(util.MaxInt(0, tileLen-s.NCommits), tileLen)
	if err != nil {
		return nil, err
	}

	// Now copy the tile by value.
	retIgnoredTile := retTile.Copy()

	// Then remove traces that should be ignored.
	ignoreQueries, err := ignore.ToQuery(ignores)
	if err != nil {
		return nil, err
	}
	for id, tr := range retIgnoredTile.Traces {
		for _, q := range ignoreQueries {
			if ptypes.Matches(tr, q) {
				delete(retIgnoredTile.Traces, id)
				continue
			}
		}
	}

	// Cache this tile.
	s.lastIgnoreRev = currentIgnoreRev
	s.lastTrimmedTile = retTile
	s.lastTrimmedIgnoredTile = retIgnoredTile
	s.lastBaseTile = tile
	fmt.Printf("Lengths: %d %d\n", len(s.lastTrimmedTile.Traces), len(s.lastTrimmedIgnoredTile.Traces))

	if includeIgnores {
		return s.lastTrimmedTile, nil
	} else {
		return s.lastTrimmedIgnoredTile, nil
	}
}

// GetOrUpdateDigestInfo is a helper function that retrieves the DigestInfo for
// the given test name/digest pair or updates the underlying info if it is not
// in the digest store yet.
func (s *Storage) GetOrUpdateDigestInfo(testName, digest string, commit *ptypes.Commit) (*digeststore.DigestInfo, error) {
	digestInfo, ok, err := s.DigestStore.Get(testName, digest)
	if err != nil {
		return nil, err
	}

	if ok {
		return digestInfo, nil
	}
	digestInfo = &digeststore.DigestInfo{
		TestName: testName,
		Digest:   digest,
		First:    commit.CommitTime,
		Last:     commit.CommitTime,
	}
	err = s.DigestStore.Update([]*digeststore.DigestInfo{digestInfo})
	if err != nil {
		return nil, err
	}

	return digestInfo, nil
}
