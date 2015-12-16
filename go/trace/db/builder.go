package db

import (
	"fmt"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/metrics"
	"go.skia.org/infra/go/tiling"
	"google.golang.org/grpc"
)

const (
	TILE_REFRESH_DURATION = 5 * time.Minute

	NEW_TILE_AVAILABLE_EVENT = "new-tile-available-event"
)

// Builder loads Tiles from a trace/db.DB.
type Builder struct {
	// DB is public to construct Tiles besides the default Tile of tileSize.
	DB DB

	// tile is the last successfully loaded Tile.
	tile *tiling.Tile

	// mutex protects access to tile.
	mutex sync.Mutex

	// tileSize is the number of commits that should be in the default Tile.
	tileSize int

	// git is the Git repo the commits come from.
	git *gitinfo.GitInfo

	// evt is the eventbus where we announce the availability of new tiles.
	evt *eventbus.EventBus
}

// NewBuilder creates a new Builder given the gitinfo, and loads Tiles from the
// traceserver running at the given address. The tiles contain the last
// 'tileSize' commits and are built from Traces of the type that traceBuilder
// returns.
func NewBuilder(git *gitinfo.GitInfo, address string, tileSize int, traceBuilder tiling.TraceBuilder, evt *eventbus.EventBus) (*Builder, error) {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("did not connect: %v", err)
	}

	// Build a tracedb.DB client.
	tracedb, err := NewTraceServiceDB(conn, traceBuilder)
	if err != nil {
		return nil, fmt.Errorf("NewTraceStore: Failed to create DB: %s", err)
	}

	ret := &Builder{
		tileSize: tileSize,
		DB:       tracedb,
		git:      git,
		evt:      evt,
	}
	if err := ret.LoadTile(); err != nil {
		return nil, fmt.Errorf("NewTraceStore: Failed to load initial Tile: %s", err)
	}
	evt.Publish(NEW_TILE_AVAILABLE_EVENT, ret.GetTile())
	go func() {
		liveness := metrics.NewLiveness("perf-tracedb-tile-refresh")
		for _ = range time.Tick(TILE_REFRESH_DURATION) {
			if err := ret.LoadTile(); err != nil {
				glog.Errorf("Failed to refresh tile: %s", err)
			} else {
				liveness.Update()
				evt.Publish(NEW_TILE_AVAILABLE_EVENT, ret.GetTile())
			}
		}
	}()
	return ret, nil
}

// LoadTile loads a Tile from the DB.
//
// Users of Builder should not normally need to call this func, as it is called
// periodically by the Builder to keep the tile fresh.
func (t *Builder) LoadTile() error {
	// Build CommitIDs for the last INITIAL_TILE_SIZE commits to the repo.
	if err := t.git.Update(true, false); err != nil {
		glog.Errorf("Failed to update Git repo: %s", err)
	}
	hashes := t.git.LastN(t.tileSize)
	commitIDs := make([]*CommitID, 0, len(hashes))
	for _, h := range hashes {
		commitIDs = append(commitIDs, &CommitID{
			ID:        h,
			Source:    "master",
			Timestamp: t.git.Timestamp(h).Unix(),
		})
	}

	// Build a Tile from those CommitIDs.
	tile, err := t.DB.TileFromCommits(commitIDs)
	if err != nil {
		return fmt.Errorf("Failed to load tile: %s", err)
	}

	// Now populate the author for each commit.
	for _, c := range tile.Commits {
		details, err := t.git.Details(c.Hash, true)
		if err != nil {
			return fmt.Errorf("Couldn't fill in author info in tile for commit %s: %s", c.Hash, err)
		}
		c.Author = details.Author
	}
	if err != nil {
		return fmt.Errorf("Failed to load initial tile: %s", err)
	}
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.tile = tile
	return nil
}

// GetTile returns the most recently loaded Tile.
// See the tiling.TileBuilder interface.
func (t *Builder) GetTile() *tiling.Tile {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.tile
}
