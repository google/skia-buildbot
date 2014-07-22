package filetilestore

import (
        "types"
)

import (
        "encoding/gob"
        "fmt"
        "io/ioutil"
        "strings"
        "testing"
        "time"
        "os"
)

func makeFakeTile(filename string, t *types.Tile) error {
        f, err := os.Create(filename)
        if err != nil {
                return fmt.Errorf("File creation failed before test start, skipping test.\n")
        }
        defer f.Close()

        enc := gob.NewEncoder(f)
        err = enc.Encode(t)
        if err != nil {
                return fmt.Errorf("Tile globbed failed before test start, skipping test.\n")
        }
        f.Sync()
        return nil
}

// TestFileTileGet tests FileTileStore's implementation of TileStore.Get.
// It does so by creating a tile gob and seeing if it is successfully read from,
// then rewriting it and seeing if it reads the new copy.
func TestFileTileGet(t *testing.T) {
        randomPath, err := ioutil.TempDir("", "filestore_test")
        if err != nil {
                t.Skip("Failing to create temporary directory, skipping test.")
                return
        }
        defer os.RemoveAll(randomPath);
        // The test file needs to be created in the 0/ subdirectory of the path.
        randomFullPath:= strings.Join([]string{randomPath, "test", "0"}, string(os.PathSeparator))

        if err := os.MkdirAll(randomFullPath, 0775); err != nil {
                t.Skip("Failing to create temporary subdirectory, skipping test.")
                return
        }

        // NOTE: This needs to match what's created by tileFilename
        fileName := strings.Join([]string{randomFullPath, "0000.gob"}, string(os.PathSeparator))
        err = makeFakeTile(fileName, &types.Tile{
                Traces: []*types.Trace{
                        &types.Trace{
                                Key: "test_key",
                                Values: []float64{0.0, 1.4, -2},
                                Params: map[string]string{"test": "parameter"},
                                Trybot: false,
                        },
                },
                ParamSet: map[string]types.Choices{
                        "test": types.Choices([]string{"parameter"}),
                },
                Commits: []*types.Commit{
                        &types.Commit{
                                CommitTime: 42,
                                Hash: "ffffffffffffffffffffffffffffffffffffffff",
                                GitNumber: -1,
                                Author: "test@test.cz",
                                CommitMessage: "This commit doesn't actually exist",
                                TailCommits: []*types.Commit{},
                        },
                },
                Scale: 0,
                TileIndex: 0,
        })
        if err != nil {
                t.Skip("Failed to create fake tile, skipping test: %s\n", err)
                return
        }
        ts := NewFileTileStore(randomPath, "test", 1*time.Second)
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

        err = makeFakeTile(fileName, &types.Tile{
                Traces: []*types.Trace{},
                ParamSet: map[string]types.Choices{
                        "test": types.Choices([]string{"parameter",}),
                },
                Commits: []*types.Commit{
                        &types.Commit{
                                CommitTime: 42,
                                Hash: "0000000000000000000000000000000000000000",
                                GitNumber: -1,
                                Author: "test@test.cz",
                                CommitMessage: "This commit doesn't actually exist",
                                TailCommits: []*types.Commit{},
                        },
                },
                Scale: 0,
                TileIndex: 0,
        })

        if err != nil {
                t.Skip("Failed to create fake tile, skipping test: %s\n", err)
                return
        }
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
                        t.Errorf("FileTileStore.Get failed: traces not matching\n")
                        dumpTile(getValue2, t)
                }
        }
        t.Log("Second test set completed.")
        // Test its ability to find the last tile, which should be the same as
        // the 0th tile, since there's only one tile in storage.
        t.Log("Third test set started. Testing last tile Get().")
        // Sleep for a few seconds to allow the lastTile updater to run.
        time.Sleep(3*time.Second);
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
        if err == nil {
                t.Errorf("FileTileStore.Get succeeded on nonexistent tile: %s\n", err)
                dumpTile(getValue4, t)
        }
        t.Log("Fourth test set completed.")
}

func dumpTile(tile *types.Tile, t *testing.T) {
        if tile != nil {
                t.Log(*tile)
                t.Log("Traces: ", tile.Traces)
                for _, trace := range tile.Traces {
                        if trace != nil {
                                t.Log(*trace)
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
