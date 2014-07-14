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
)

import (
	"github.com/golang/glog"
)

import (
	"types"
)

// FileTileStore implements TileStore by storing Tiles as gobs in the file system.
//
// The directory structure is dir/datasetName/scale/index.gob where
// index is 0 padded so that the file names sort alphabetically.
type FileTileStore struct {
	// The root directory where Tiles should be written.
	dir string

	// Which dataset are we writing, e.g. "skps" or "micro".
	datasetName string
}

func (store FileTileStore) tileFilename(scale, index int) (string, error) {
	if scale < 0 || index < 0 {
		return "", fmt.Errorf("Scale %d and Index %d must both be > 0", scale, index)
	}
	return path.Join(store.dir, store.datasetName, fmt.Sprintf("%d/%04d.gob", scale, index)), nil
}

func (store FileTileStore) Put(scale, index int, tile *types.Tile) error {
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

	return nil
}

func (store FileTileStore) Get(scale, index int) (*types.Tile, error) {
	// -1 means find the last tile for the given scale.
	if index == -1 {
		matches, _ := filepath.Glob(path.Join(store.dir, store.datasetName, fmt.Sprintf("%d/*.gob", scale)))
		if matches != nil {
			sort.Strings(matches)
			lastTileName := filepath.Base(matches[len(matches)-1])
			glog.Infof("Found the last tile: %s", lastTileName)
			tileIndex := strings.Split(lastTileName, ".")[0]
			newIndex, err := strconv.ParseInt(tileIndex, 10, 64)
			if err == nil {
				index = int(newIndex)
			}
		} else {
			return nil, nil
		}
	}
	filename, err := store.tileFilename(scale, index)
	if err != nil {
		return nil, err
	}
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

// NewFileTileStore creates a new TileStore that is backed by the file system,
// where dir is the directory name and datasetName is the name of the dataset.
func NewFileTileStore(dir, datasetName string) types.TileStore {
	return FileTileStore{
		dir:         dir,
		datasetName: datasetName,
	}
}
