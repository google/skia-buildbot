package filetilestore

import (
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
)

// TODO(stephana): Replace with github.com/hashicorp/golang-lru

const (
	MAX_CACHE_TILES = 10

	TEMP_TILE_DIR_NAME = "_temp"
)

// CacheEntry stores a single tile with the data describing it.
type CacheEntry struct {
	tile         *tiling.Tile
	lastModified time.Time
}

// CacheKey is used as a key to the lru cache and must be a 'comparable'.
// http://golang.org/ref/spec#Comparison_operators
type CacheKey struct {
	startIndex int
	scale      int
}

// FileTileStore implements TileStore by storing Tiles as gobs in the file system.
//
// The directory structure is dir/datasetName/scale/index.gob where
// index is 0 padded so that the file names sort alphabetically.
type FileTileStore struct {
	// The root directory where Tiles should be written.
	dir string

	// Which dataset are we writing, e.g. "skps" or "micro".
	datasetName string

	// Cache for recently used tiles.
	cache *lru.Cache

	// Mutex for ensuring safe access to the cache and lastTile.
	lock sync.Mutex
}

// tileFilename creates the filename for the given tile scale and index for the
// given FileTileStore.
func (store *FileTileStore) tileFilename(scale, index int) (string, error) {
	if scale < 0 || index < 0 {
		return "", fmt.Errorf("Scale %d and Index %d must both be >= 0", scale, index)
	}
	return path.Join(store.dir, store.datasetName, fmt.Sprintf("%d/%04d.gob", scale, index)), nil
}

// fileTileTemp creates a unique temporary filename for the given tile scale and
// index for the given FileTileStore. Used during Put() so that writes update
// atomically.
func (store *FileTileStore) fileTileTemp(scale, index int) (*os.File, error) {
	if scale < 0 || index < 0 {
		return nil, fmt.Errorf("Scale %d and Index %d must both be >= 0", scale, index)
	}
	dir := path.Join(store.dir, TEMP_TILE_DIR_NAME)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("Error creating directory for temp tile %s: %s", dir, err)
	}
	return ioutil.TempFile(dir, fmt.Sprintf("%d-%04d-gob-", scale, index))
}

// Put writes a tile to the drive, and also updates the cache entry for it
// if one exists. It uses the mutex to ensure thread safety.
func (store *FileTileStore) Put(scale, index int, tile *tiling.Tile) error {
	sklog.Info("Put()")
	// Make sure the scale and tile index are correct.
	if tile.Scale != scale || tile.TileIndex != index {
		return fmt.Errorf("Tile scale %d and index %d do not match real tile scale %d and index %d", scale, index, tile.Scale, tile.TileIndex)
	}

	if index < 0 {
		return fmt.Errorf("Can't write Tiles with an index < 0: %d", index)
	}

	// Begin by writing the Tile out into a temporary location.
	f, err := store.fileTileTemp(scale, index)
	if err != nil {
		return err
	}
	enc := gob.NewEncoder(f)
	if err := enc.Encode(tile); err != nil {
		return fmt.Errorf("Failed to encode tile %s: %s", f.Name(), err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("Failed to close temporary file: %v", err)
	}

	// Now rename the completed file to the real tile name. This is atomic and
	// doesn't affect current readers of the old tile contents.
	targetName, err := store.tileFilename(scale, index)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetName), 0755); err != nil {
		return fmt.Errorf("Error creating directory for tile %s: %s", targetName, err)
	}
	sklog.Infof("Renaming: %q %q", f.Name(), targetName)
	if err := os.Rename(f.Name(), targetName); err != nil {
		return fmt.Errorf("Failed to rename tile: %s", err)
	}
	filedata, err := os.Stat(targetName)
	if err != nil {
		return fmt.Errorf("Failed to stat new tile: %s", err)

	}
	store.lock.Lock()
	defer store.lock.Unlock()

	entry := &CacheEntry{
		tile:         tile,
		lastModified: filedata.ModTime(),
	}
	key := CacheKey{
		startIndex: index,
		scale:      scale,
	}
	store.cache.Add(key, entry)

	return nil
}

// getLastTile gets a copy of the last tile for the given scale from disk. Its
// thread safety comes from not using the tile store cache at all.
func (store *FileTileStore) getLastTile(scale int) (*tiling.Tile, error) {
	tilePath := path.Join(store.dir, store.datasetName, fmt.Sprintf("%d/*.gob", scale))
	matches, _ := filepath.Glob(tilePath)
	if matches == nil {
		return nil, fmt.Errorf("Failed to find any tiles in %s", tilePath)
	}
	sort.Strings(matches)
	lastTileName := filepath.Base(matches[len(matches)-1])
	sklog.Infof("Found the last tile: %s", lastTileName)
	tileIndex := strings.Split(lastTileName, ".")[0]
	newIndex, err := strconv.ParseInt(tileIndex, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Unable to get last tile index for scale %d", scale)
	}
	index := int(newIndex)
	filename, err := store.tileFilename(scale, index)
	if err != nil {
		return nil, fmt.Errorf("Unable to get filename for scale %d, index %d", scale, index)
	}
	tileData, err := openTile(filename)
	if err != nil {
		return nil, fmt.Errorf("Unable to open last tile file %s", lastTileName)
	}
	// If possible, merge with the previous tile.
	if index > 0 {
		prevFilename, err := store.tileFilename(scale, index-1)
		if err != nil {
			return nil, fmt.Errorf("Unable to get filename for scale %d, index %d", scale, index)
		}
		prevTile, err := openTile(prevFilename)
		if err != nil {
			return nil, fmt.Errorf("Unable to open prev tile file %s", prevFilename)
		}
		tileData = tiling.Merge(prevTile, tileData)
	}

	return tileData, nil
}

// openTile opens the tile file passed in and returns the decoded contents.
func openTile(filename string) (*tiling.Tile, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to open tile %s for reading: %s", filename, err)
	}
	defer util.Close(f)
	t := tiling.NewTile()
	dec := gob.NewDecoder(f)
	if err := dec.Decode(t); err != nil {
		return nil, fmt.Errorf("Failed to decode tile %s: %s", filename, err)
	}
	return t, nil
}

// Get returns a tile from the file tile store, storing it into cache if it is
// not already there. It is threadsafe because it locks the tile store's mutex
// before accessing the cache.
// NOTE: Assumes the caller does not modify the copy it returns
func (store *FileTileStore) Get(scale, index int) (*tiling.Tile, error) {
	store.lock.Lock()
	defer store.lock.Unlock()

	key := CacheKey{
		startIndex: index,
		scale:      scale,
	}

	// Retrieve the tile, if any, from the cache.
	var tile *tiling.Tile
	var cacheLastModified time.Time
	if val, ok := store.cache.Get(key); ok {
		cacheEntry := val.(*CacheEntry)
		tile = cacheEntry.tile
		cacheLastModified = cacheEntry.lastModified
	}
	if index == -1 {
		if tile == nil {
			var err error
			tile, err = store.getLastTile(scale)
			if err != nil {
				return nil, fmt.Errorf("Failed to Get the last tile: %s", err)
			}
		}
		return tile, nil
	}

	// Compare to the tile on disk.
	filename, err := store.tileFilename(scale, index)
	filedata, err := os.Stat(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("Tile %d,%d retrieval caused error : %s.", scale, index, err)
		} else {
			return nil, nil
		}
	}
	fileLastModified := filedata.ModTime()

	// If the file on disk is newer, or there wasn't anything in the cache, read
	// the tile from disk.
	if tile == nil || fileLastModified.After(cacheLastModified) {
		tile, err = openTile(filename)
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve tile %s: %s", filename, err)
		}
		entry := &CacheEntry{
			tile:         tile,
			lastModified: fileLastModified,
		}
		store.cache.Add(key, entry)
	}

	return tile, nil
}

// GetModifiable returns a tile from disk.
// This ensures the tile can be modified without affecting the cache.
// NOTE: Currently relies on getLastTile returning a new copy in all cases.
func (store *FileTileStore) GetModifiable(scale, index int) (*tiling.Tile, error) {
	store.lock.Lock()
	defer store.lock.Unlock()
	// -1 means find the last tile for the given scale.
	if index == -1 {
		return store.getLastTile(scale)
	}
	filename, err := store.tileFilename(scale, index)
	if err != nil {
		return nil, fmt.Errorf("Unable to create a file name for the tile %d, %d: %s\n", scale, index, err)
	}
	_, err = os.Stat(filename)
	// File probably isn't there, so return a new tile.
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("Tile %d,%d retrieval caused error : %s.", scale, index, err)
		} else {
			newTile := tiling.NewTile()
			newTile.Scale = scale
			newTile.TileIndex = index
			return newTile, nil
		}
	}
	t, err := openTile(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve tile %s: %s", filename, err)
	}
	return t, nil
}

// refreshLastTiles reloads the last (-1) tile.
func (store *FileTileStore) refreshLastTiles() {
	// Read tile -1.
	tile, err := store.getLastTile(0)
	if err != nil {
		sklog.Warningf("Unable to retrieve last tile for scale %d: %s", 0, err)
		return
	}
	store.lock.Lock()
	defer store.lock.Unlock()

	entry := &CacheEntry{
		tile:         tile,
		lastModified: time.Now(),
	}
	key := CacheKey{
		startIndex: -1,
		scale:      0,
	}
	store.cache.Add(key, entry)
}

// NewFileTileStore creates a new TileStore that is backed by the file system,
// where dir is the directory name and datasetName is the name of the dataset.
// checkEvery sets how often the cache for the last tile should be updated,
// with a zero or negative duration meaning to never update the last tile entry.
func NewFileTileStore(dir, datasetName string, checkEvery time.Duration) tiling.TileStore {
	store := &FileTileStore{
		dir:         dir,
		datasetName: datasetName,
		cache:       lru.New(MAX_CACHE_TILES),
	}
	store.refreshLastTiles()
	if checkEvery > 0 {
		// NOTE: This probably stops the tilestore from being garbage
		// collected. Not an issue as far as I can tell, but should
		// we try to handle this correctly?

		// Refresh the lastTile entries periodically.
		go func() {
			for range time.Tick(checkEvery) {
				store.refreshLastTiles()
			}
		}()
	}
	return store
}
