package mergedtiles

import (
	"fmt"
	"sync"

	// TODO(stephana): Replace with github.com/hashicorp/golang-lru
	"github.com/golang/groupcache/lru"
	"go.skia.org/infra/go/tiling"
)

type key struct {
	scale      int
	startIndex int
	endIndex   int
}

// MergedTiles produces merged tiles.
//
// The results are cached since merging is a time consuming operation.
type MergedTiles struct {
	store tiling.TileStore
	cache *lru.Cache
	mutex sync.Mutex
}

// getFromCache returns a merged tile from the cache, or nil on a miss.
func (m *MergedTiles) getFromCache(key key) *tiling.Tile {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if val, ok := m.cache.Get(key); ok {
		return val.(*tiling.Tile)
	}
	return nil
}

func (m *MergedTiles) addToCache(key key, tile *tiling.Tile) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.cache.Add(key, tile)
}

// Get returns a tile that is the merged tiles from startIndex to endIndex
// inclusive.
func (m *MergedTiles) Get(scale, startIndex, endIndex int) (*tiling.Tile, error) {
	k := key{
		scale:      scale,
		startIndex: startIndex,
		endIndex:   endIndex,
	}

	tile := m.getFromCache(k)
	if tile != nil {
		return tile, nil
	}

	var err error
	tile, err = m.store.Get(scale, startIndex)
	if err != nil || tile == nil {
		return nil, fmt.Errorf("Failed retrieving tile to merge: %s.", err)
	}
	for i := startIndex + 1; i <= endIndex; i++ {
		// Look for a previously cached Tile that represents [i:end].
		// If found, just merge tile with it and be done.
		rKey := key{
			scale:      scale,
			startIndex: i,
			endIndex:   endIndex,
		}
		if rTile := m.getFromCache(rKey); rTile != nil {
			tile = tiling.Merge(tile, rTile)
			break
		}

		// Otherwise continue building the merged tile on a tile-by-tile basis.
		tile2, err := m.store.Get(scale, i)
		if err != nil || tile2 == nil {
			return nil, fmt.Errorf("Failed retrieving tile to merge: %s.", err)
		}
		tile = tiling.Merge(tile, tile2)
	}

	m.addToCache(k, tile)

	return tile, nil
}

// NewMergedTileCache creates a new MergedTileCache.
func NewMergedTiles(tilestore tiling.TileStore, maxEntries int) *MergedTiles {
	return &MergedTiles{
		store: tilestore,
		cache: lru.New(maxEntries),
	}
}
