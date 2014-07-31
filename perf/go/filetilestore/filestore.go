package filetilestore

import (
	"encoding/gob"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

import (
	"github.com/golang/glog"
)

import (
	"skia.googlesource.com/buildbot.git/perf/go/types"
)

const (
	MAX_CACHE_TILES = 64
	MAX_CACHE_SCALE = 0
)

// CacheEntry stores a single tile with the data describing it.
type CacheEntry struct {
	tile         *types.Tile
	lastModified time.Time
	index        int
	scale        int
	countUsed    int
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

	// Cache for recently used tiles, eviction based on LFU.
	cache []CacheEntry
	// Special case for -1, since this one will both be changed often and
	// be accessed often.
	lastTile map[int]*types.Tile

	// Mutex for ensuring safe access to the cache and lastTile.
	lock sync.Mutex
}

// tileFilename creates the filename for the given tile scale and index for the
// given FileTileStore.
func (store FileTileStore) tileFilename(scale, index int) (string, error) {
	if scale < 0 || index < 0 {
		return "", fmt.Errorf("Scale %d and Index %d must both be > 0", scale, index)
	}
	return path.Join(store.dir, store.datasetName, fmt.Sprintf("%d/%04d.gob", scale, index)), nil
}

// Put writes a tile to the drive, and also updates the cache entry for it
// if one exists. It uses the mutex to ensure thread safety.
func (store *FileTileStore) Put(scale, index int, tile *types.Tile) error {
	// Make sure the scale and tile index are correct.
	if tile.Scale != scale || tile.TileIndex != index {
		return fmt.Errorf("Tile scale %d and index %d do not match real tile scale %d and index %d", scale, index, tile.Scale, tile.TileIndex)
	}

	if index < 0 {
		return fmt.Errorf("Can't write Tiles with an index < 0: %d", index)
	}
	filename, err := store.tileFilename(scale, index)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return fmt.Errorf("Error creating directory for tile %s: %s", filename, err)
	}
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Failed to open tile %s for writing: %s", filename, err)
	}
	defer f.Close()
	enc := gob.NewEncoder(f)
	if err := enc.Encode(tile); err != nil {
		return fmt.Errorf("Failed to encode tile %s: %s", filename, err)
	}

	store.lock.Lock()
	defer store.lock.Unlock()
	for i, entry := range store.cache {
		if entry.index == index && entry.scale == scale {
			filedata, err := os.Stat(filename)
			if err != nil {
				break
			}
			store.cache[i].tile = tile
			store.cache[i].lastModified = filedata.ModTime()
		}
	}

	return nil
}

// getLastTile gets a copy of the last tile for the given scale from disk. Its
// thread safety comes from not using the tile store cache at all.
func (store *FileTileStore) getLastTile(scale int) (*types.Tile, error) {
	tilePath := path.Join(store.dir, store.datasetName, fmt.Sprintf("%d/*.gob", scale))
	matches, _ := filepath.Glob(tilePath)
	if matches == nil {
		return nil, fmt.Errorf("Failed to find any tiles in %s", tilePath)
	}
	sort.Strings(matches)
	lastTileName := filepath.Base(matches[len(matches)-1])
	glog.Infof("Found the last tile: %s", lastTileName)
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
	return tileData, nil
}

// openTile opens the tile file passed in and returns the decoded contents.
func openTile(filename string) (*types.Tile, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to open tile %s for reading: %s", filename, err)
	}
	defer f.Close()
	t := types.NewTile()
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
func (store *FileTileStore) Get(scale, index int) (*types.Tile, error) {
	store.lock.Lock()
	defer store.lock.Unlock()
	// -1 means find the last tile for the given scale.
	if index == -1 {
		if _, ok := store.lastTile[scale]; ok {
			return store.lastTile[scale], nil
		} else {
			return nil, fmt.Errorf("Last tile not available at scale %d", scale)
		}
	}
	filename, err := store.tileFilename(scale, index)
	fmt.Println(filename)
	fileData, err := os.Stat(filename)
	// File probably isn't there, so return nil
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("Tile %d,%d retrieval caused error : %s.", scale, index, err)
		} else {
			return nil, nil
		}
	}
	fileLastModified := fileData.ModTime()
	for i, entry := range store.cache {
		if entry.scale != scale || entry.index != index {
			continue
		}
		if !entry.lastModified.Equal(fileLastModified) {
			// Replace the tile in the cache entry with the new one
			glog.Infof("FileTileStore: cache miss: %d, %d", scale, index)
			newEntry, err := openTile(filename)
			if err != nil {
				return nil, fmt.Errorf("Failed to retrieve tile %s: %s", filename, err)
			}
			store.cache[i].tile = newEntry
			store.cache[i].lastModified = fileLastModified
		}
		glog.Infof("FileTileStore: cache hit: %d, %d", scale, index)
		// Increment the frequency counter and return the tile
		store.cache[i].countUsed += 1
		return store.cache[i].tile, nil
	}
	// Not in cache
	glog.Infof("FileTileStore: cache miss: %d, %d", scale, index)
	t, err := openTile(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve tile %s: %s", filename, err)
	}
	store.addToCache(CacheEntry{
		tile:         t,
		lastModified: fileLastModified,
		countUsed:    1,
		scale:        scale,
		index:        index,
	})
	return t, nil
}

// GetModifiable returns a tile from disk.
// This ensures the tile can be modified without affecting the cache.
// NOTE: Currently relies on getLastTile returning a new copy in all cases.
func (store *FileTileStore) GetModifiable(scale, index int) (*types.Tile, error) {
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
	//fmt.Println(filename)
	_, err = os.Stat(filename)
	// File probably isn't there, so return a new tile.
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("Tile %d,%d retrieval caused error : %s.", scale, index, err)
		} else {
			newTile := types.NewTile()
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

// addToCache adds a cache entry to the cache, evicting the least frequently
// used entry if the cache is full. NOTE: It's not thread safe, and relies on
// Get() and Put() locking the lock Mutex before using it.
func (store *FileTileStore) addToCache(c CacheEntry) {
	// Put the file in the cache.
	if len(store.cache) < cap(store.cache) {
		store.cache = append(store.cache, c)
	} else {
		// Evict an entry from the cache.
		idxLeastUsed := 0
		leastUsed := store.cache[idxLeastUsed].countUsed
		for idx, entry := range store.cache {
			if entry.countUsed < leastUsed {
				idxLeastUsed, leastUsed = idx, entry.countUsed
			}
		}
		store.cache[idxLeastUsed] = c
	}
}

// refreshLastTiles checks all the versions of the last tile to see if any of them
// where updated on disk, and updates the version on cache if needed.
func (store *FileTileStore) refreshLastTiles() {
	store.lock.Lock()
	defer store.lock.Unlock()
	for scale := 0; scale <= MAX_CACHE_SCALE; scale++ {
		newLastTile, err := store.getLastTile(scale)
		if err != nil {
			glog.Errorf("Unable to retrieve last tile for scale %d: %s", scale, err)
			continue
		}
		store.lastTile[scale] = newLastTile
	}
}

// NewFileTileStore creates a new TileStore that is backed by the file system,
// where dir is the directory name and datasetName is the name of the dataset.
// checkEvery sets how often the cache for the last tile should be updated,
// with a zero or negative duration meaning to never update the last tile entry.
func NewFileTileStore(dir, datasetName string, checkEvery time.Duration) types.TileStore {
	store := &FileTileStore{
		dir:         dir,
		datasetName: datasetName,
		cache:       make([]CacheEntry, MAX_CACHE_TILES)[:0],
		lastTile:    make(map[int]*types.Tile),
		lock:        sync.Mutex{},
	}
	if checkEvery > 0 {
		// NOTE: This probably stops the tilestore from being garbage
		// collected. Not an issue as far as I can tell, but should
		// we try to handle this correctly?

		store.refreshLastTiles()
		// Refresh the lastTile entries periodically.
		go func() {
			for _ = range time.Tick(checkEvery) {
				store.refreshLastTiles()
			}
		}()
	}
	return store
}
