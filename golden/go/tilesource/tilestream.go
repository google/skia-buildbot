package tilesource

import (
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/tiling"
)

// GetTileStreamNow is a utility function that reads tiles in the given
// interval and sends them on the returned channel.
// The first tile is sent immediately.
// Should the call to read a new tile fail it will send that last
// successfully read tile. Thus it guarantees to send a tile in the provided
// interval, assuming at least one tile could be read.
// The metricsTag allows for us to monitor how long individual tile streams
// take, in the unlikely event there are multiple failures of the tile in a row.
func GetTileStreamNow(ts TileSource, interval time.Duration, metricsTag string) <-chan tiling.ComplexTile {
	retCh := make(chan tiling.ComplexTile)
	lastTileStreamed := metrics2.NewLiveness("last_tile_streamed", map[string]string{
		"source": metricsTag,
	})
	go func() {
		var lastTile tiling.ComplexTile = nil

		readOneTile := func() {
			tile := ts.GetTile()
			if lastTile != tile {
				lastTile = tile
				lastTileStreamed.Reset()
				retCh <- tile
			} else {
				sklog.Debugf("Tile hasn't changed for tile stream")
			}
		}

		readOneTile()
		for range time.Tick(interval) {
			readOneTile()
		}
	}()

	return retCh
}
