package db

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/serialize"
)

const (
	TILE_REFRESH_DURATION = 5 * time.Minute

	NEW_TILE_AVAILABLE_EVENT = "new-tile-available-event"
)

// MasterTileBuilder continously loads Tiles from a trace/db.DB.
type MasterTileBuilder interface {
	// GetTile returns the most recently loaded Tile.
	GetTile() *tiling.Tile
}

// Impelementation of MasterTilebuilder
type masterTileBuilder struct {
	// db is used to construct  tiles.
	db DB

	// tile is the last successfully loaded Tile.
	tile *tiling.Tile

	// mutex protects access to tile.
	mutex sync.Mutex

	// tileSize is the number of commits that should be in the default Tile.
	tileSize int

	// vcs is an interface to the Git repo where commits come from.
	vcs vcsinfo.VCS

	// evt is the eventbus where we announce the availability of new tiles.
	evt eventbus.EventBus

	// cachePath is the path to the file where tiles are cahed for quick restarts.
	cachePath string
}

// NewBuilder creates a new Builder given the gitinfo, and loads Tiles from the
// traceserver running at the given address. The tiles contain the last
// 'tileSize' commits and are built from Traces of the type that traceBuilder
// returns.
func NewMasterTileBuilder(ctx context.Context, db DB, vcs vcsinfo.VCS, tileSize int, evt eventbus.EventBus, cachePath string) (MasterTileBuilder, error) {
	ret := &masterTileBuilder{
		tileSize:  tileSize,
		tile:      nil,
		db:        db,
		vcs:       vcs,
		evt:       evt,
		cachePath: cachePath,
	}

	var err error
	if cachePath != "" {
		// Load tile from disk cache. No mutex needed since there is no concurrent
		// access at this point.
		if ret.tile, err = serialize.LoadCachedTile(cachePath); err != nil {
			return nil, fmt.Errorf("Error loading tile from cache: %s", err)
		}
	}

	// Load the tile if it was not in the cache.
	initialTileLoaded := false
	if ret.tile == nil {
		initialTileLoaded = true
		if err := ret.LoadTile(ctx); err != nil {
			return nil, fmt.Errorf("NewTraceStore: Failed to load initial Tile: %s", err)
		}
	}

	evt.Publish(NEW_TILE_AVAILABLE_EVENT, ret.GetTile(), false)
	go func() {
		if !initialTileLoaded {
			// Load the initial tile from disk if it came from the disk cache.
			if err := ret.LoadTile(ctx); err != nil {
				sklog.Errorf("Failed to refresh tile: %s", err)
			}
		}

		liveness := metrics2.NewLiveness("tile_refresh", map[string]string{"module": "tracedb"})
		for range time.Tick(TILE_REFRESH_DURATION) {
			if err := ret.LoadTile(ctx); err != nil {
				sklog.Errorf("Failed to refresh tile: %s", err)
			} else {
				liveness.Reset()
				evt.Publish(NEW_TILE_AVAILABLE_EVENT, ret.GetTile(), false)
			}
		}
	}()
	return ret, nil
}

// LoadTile loads a Tile from the db.
//
// Users of Builder should not normally need to call this func, as it is called
// periodically by the Builder to keep the tile fresh.
func (t *masterTileBuilder) LoadTile(ctx context.Context) error {
	// Build CommitIDs for the last INITIAL_TILE_SIZE commits to the repo.
	if err := t.vcs.Update(ctx, true, false); err != nil {
		sklog.Errorf("Failed to update Git repo: %s", err)
	}
	indexCommits := t.vcs.LastNIndex(t.tileSize)
	last := indexCommits[len(indexCommits)-1]
	sklog.Infof("Loaded tile with last commit: %#v", *last)
	commitIDs := make([]*CommitID, 0, len(indexCommits))
	for _, ic := range indexCommits {
		commitIDs = append(commitIDs, &CommitID{
			ID:        ic.Hash,
			Source:    "master",
			Timestamp: ic.Timestamp.Unix(),
		})
	}

	// Build a Tile from those CommitIDs.
	tile, _, err := t.db.TileFromCommits(commitIDs)
	if err != nil {
		return fmt.Errorf("Failed to load tile: %s", err)
	}

	// Now populate the author for each commit.
	for _, c := range tile.Commits {
		details, err := t.vcs.Details(ctx, c.Hash, false)
		if err != nil {
			return fmt.Errorf("Couldn't fill in author info in tile for commit %s: %s", c.Hash, err)
		}
		c.Author = details.Author
	}
	if err != nil {
		return fmt.Errorf("Failed to load initial tile: %s", err)
	}

	// Cache the file to disk if a path was set.
	if t.cachePath != "" {
		go func() {
			if err := serialize.CacheTile(tile, t.cachePath); err != nil {
				sklog.Errorf("Error writing tile to cache: %s", err)
			}
		}()
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.tile = tile
	return nil
}

// See the MasterTileBuilder interface.
func (t *masterTileBuilder) GetTile() *tiling.Tile {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.tile
}
