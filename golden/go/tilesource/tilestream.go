package tilesource

import (
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/types"
)

// GetTileStreamNow is a utility function that reads tiles in the given
// interval and sends them on the returned channel.
// The first tile is sent immediately.
// Should the call to read a new tile fail it will send that last
// successfully read tile. Thus it guarantees to send a tile in the provided
// interval, assuming at least one tile could be read.
// The metricsTag allows for us to monitor how long individual tile streams
// take, in the unlikely event there are multiple failures of the tile in a row.
func GetTileStreamNow(ts TileSource, interval time.Duration, metricsTag string) <-chan types.ComplexTile {
	retCh := make(chan types.ComplexTile)
	lastTileStreamed := metrics2.NewLiveness("last_tile_streamed", map[string]string{
		"source": metricsTag,
	})
	go func() {
		var lastTile types.ComplexTile = nil

		readOneTile := func() {
			if tile, err := ts.GetTile(); err != nil {
				// Log the error and send the best tile we have right now.
				sklog.Errorf("Error reading tile: %s", err)
				if lastTile != nil {
					retCh <- lastTile
				}
			} else {
				lastTile = tile
				lastTileStreamed.Reset()
				retCh <- tile
			}
		}

		readOneTile()
		for range time.Tick(interval) {
			readOneTile()
		}
	}()

	return retCh
}
