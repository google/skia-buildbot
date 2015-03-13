package filetilestore

import (
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/types"
)

func makeFakeTile(t *testing.T, filename string, tile *types.Tile) {
	f, err := os.Create(filename)
	assert.Nil(t, err, fmt.Sprintf("File creation failed before test start: %s", err))
	defer testutils.AssertCloses(t, f)
	enc := gob.NewEncoder(f)
	assert.Nil(t, enc.Encode(tile), fmt.Sprintf("Tile globbed failed before test start: %s", err))
	assert.Nil(t, f.Sync())
}

// TestFileTileGet tests FileTileStore's implementation of TileStore.Get.
// It does so by creating a tile gob and seeing if it is successfully read from,
// then rewriting it and seeing if it reads the new copy.
func TestFileTileGet(t *testing.T) {
	randomPath, err := ioutil.TempDir("", "filestore_test")
	if err != nil {
		t.Fatalf("Failing to create temporary directory: %s", err)
		return
	}
	defer testutils.RemoveAll(t, randomPath)
	// The test file needs to be created in the 0/ subdirectory of the path.
	randomFullPath := filepath.Join(randomPath, "test", "0")

	if err := os.MkdirAll(randomFullPath, 0775); err != nil {
		t.Fatalf("Failing to create temporary subdirectory: %s", err)
		return
	}

	// NOTE: This needs to match what's created by tileFilename
	fileName := filepath.Join(randomFullPath, "0000.gob")
	makeFakeTile(t, fileName, &types.Tile{
		Traces: map[string]types.Trace{
			"test": &types.PerfTrace{
				Values:  []float64{0.0, 1.4, -2},
				Params_: map[string]string{"test": "parameter"},
			},
		},
		ParamSet: map[string][]string{
			"test": []string{"parameter"},
		},
		Commits: []*types.Commit{
			&types.Commit{
				CommitTime: 42,
				Hash:       "ffffffffffffffffffffffffffffffffffffffff",
				Author:     "test@test.cz",
			},
		},
		Scale:     0,
		TileIndex: 0,
	})

	ts := NewFileTileStore(randomPath, "test", 10*time.Millisecond)
	t.Log("First test set started. Testing basic tile Get().")
	getValue, err := ts.Get(0, 0)
	if err != nil {
		t.Errorf("FileTileStore.Get failed: %s\n", err)
	} else {
		if got, want := getValue.Scale, 0; got != want {
			t.Errorf("FileTileStore.Get failed: scale values not matching\n")
			dumpTile(getValue, t)
		}
		if got, want := getValue.TileIndex, 0; got != want {
			t.Errorf("FileTileStore.Get failed: tile index values not matching\n")
			dumpTile(getValue, t)
		}
		if got, want := len(getValue.Traces), 1; got != want {
			t.Errorf("FileTileStore.Get failed: traces not matching\n")
			dumpTile(getValue, t)
		}
	}
	t.Log("First test set completed.")

	makeFakeTile(t, fileName, &types.Tile{
		Traces: map[string]types.Trace{},
		ParamSet: map[string][]string{
			"test": []string{"parameter"},
		},
		Commits: []*types.Commit{
			&types.Commit{
				CommitTime: 42,
				Hash:       "0000000000000000000000000000000000000000",
				Author:     "test@test.cz",
			},
		},
		Scale:     0,
		TileIndex: 0,
	})

	// Test to see if changing the disk copy of it changes the copy
	// we get back as well.
	t.Log("Second test set started. Testing changed tile Get().")
	getValue2, err := ts.Get(0, 0)
	if err != nil {
		t.Errorf("FileTileStore.Get failed: %s\n", err)
	} else {
		if got, want := getValue2.Scale, 0; got != want {
			t.Errorf("FileTileStore.Get failed: scale values not matching\n")
			dumpTile(getValue2, t)
		}
		if got, want := getValue2.TileIndex, 0; got != want {
			t.Errorf("FileTileStore.Get failed: tile index values not matching\n")
			dumpTile(getValue2, t)
		}
		if got, want := len(getValue2.Traces), 0; got != want {
			t.Errorf("FileTileStore.Get failed: traces not matching: Got %d Want %d\n", got, want)
			dumpTile(getValue2, t)
		}
	}
	t.Log("Second test set completed.")
	// Test its ability to find the last tile, which should be the same as
	// the 0th tile, since there's only one tile in storage.
	t.Log("Third test set started. Testing last tile Get().")
	// Sleep for a few milliseconds to allow the lastTile updater to run.
	time.Sleep(30 * time.Millisecond)
	getValue3, err := ts.Get(0, -1)
	if err != nil {
		t.Errorf("FileTileStore.Get failed: %s\n", err)
	} else {
		if got, want := getValue3.Scale, 0; got != want {
			t.Errorf("FileTileStore.Get failed: scale values not matching\n")
			dumpTile(getValue3, t)
		}
		if got, want := getValue3.TileIndex, 0; got != want {
			t.Errorf("FileTileStore.Get failed: tile index values not matching\n")
			dumpTile(getValue3, t)
		}
		if got, want := len(getValue3.Traces), 0; got != want {
			t.Errorf("FileTileStore.Get failed: traces not matching\n")
			dumpTile(getValue3, t)
		}
	}
	t.Log("Third test set completed.")
	// Test if it returns an error when there's a request for a nonexistent
	// tile.
	t.Log("Fourth test set started. Testing non existent tile Get().")
	getValue4, err := ts.Get(0, 42)
	if err != nil {
		t.Errorf("FileTileStore.Get failed on nonexistent tile: %s\n", err)
	}
	if getValue4 != nil {
		t.Errorf("FileTileStore.Get returns nonnil on nonexistent tile: %s\n", err)
		dumpTile(getValue4, t)
	}
	t.Log("Fourth test set completed.")
	t.Log("Fifth test start. Testing GetModifiable().")
	getValue5, err := ts.GetModifiable(0, 0)
	if err != nil {
		t.Errorf("FileTileStore.GetModifiable failed: %s\n", err)
	} else {
		getValue5.TileIndex = 7
		getValue5, err := ts.Get(0, 0)
		if err != nil {
			t.Errorf("FileTileStore.Get failed: %s\n", err)
		} else {
			if got, want := getValue5.TileIndex, 0; got != want {
				t.Errorf("FileTileStore.Get failed: tile index modified via GetModifiable call\n")
			}
		}
	}
	t.Log("Fifth test set completed.")
	t.Log("Sixth test set start. Testing empty tile Put() and Get().")
	if err := ts.Put(0, 0, types.NewTile()); err != nil {
		t.Fatalf("Failed to Put(): %s", err)
	}

	fi, err := os.Stat(filepath.Join(randomPath, TEMP_TILE_DIR_NAME))
	if err != nil {
		t.Fatalf("Failed to stat directory where temp files are created: %s", err)
	}
	if !fi.IsDir() {
		t.Fatalf("Should be a directory.")
	}
	_, err = ts.Get(0, 0)
	if err != nil {
		t.Errorf("FileTileStore.GetModifiable failed: %s\n", err)
	}
	t.Log("Sixth test set completed.")
}

func dumpTile(tile *types.Tile, t *testing.T) {
	if tile != nil {
		t.Log(*tile)
		t.Log("Traces: ", tile.Traces)
		for _, trace := range tile.Traces {
			if trace != nil {
				t.Log(trace)
			} else {
				t.Log("nil trace")
			}
		}
		t.Log("ParamSet: ", tile.ParamSet)
		for key, val := range tile.ParamSet {
			if val != nil {
				t.Log(key, ": ", val)
			} else {
				t.Log(key, ": nil value")
			}
		}
		t.Log("Commits: ", tile.Commits)
		for _, commit := range tile.Commits {
			if commit != nil {
				t.Log(*commit)
			} else {
				t.Log("nil commit")
			}
		}
	} else {
		t.Log("nil tile")
	}
}
