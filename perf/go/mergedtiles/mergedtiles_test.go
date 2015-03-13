package mergedtiles

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
	"go.skia.org/infra/perf/go/filetilestore"
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

func TestMerging(t *testing.T) {
	randomPath, err := ioutil.TempDir("", "mergedtiles_test")
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
			&types.Commit{
				CommitTime: 43,
				Hash:       "eeeeeeeeeee",
				Author:     "test@test.cz",
			},
			&types.Commit{
				CommitTime: 44,
				Hash:       "aaaaaaaaaaa",
				Author:     "test@test.cz",
			},
		},
		Scale:     0,
		TileIndex: 0,
	})

	ts := filetilestore.NewFileTileStore(randomPath, "test", 10*time.Millisecond)
	m := NewMergedTiles(ts, 2)

	_, err = m.Get(0, 0, 1)
	if err == nil {
		t.Fatalf("Failed to error when requesting a merged tile that doesn't exist: %s", err)
	}

	fileName = filepath.Join(randomFullPath, "0001.gob")
	makeFakeTile(t, fileName, &types.Tile{
		Traces: map[string]types.Trace{},
		ParamSet: map[string][]string{
			"test": []string{"parameter"},
		},
		Commits: []*types.Commit{
			&types.Commit{
				CommitTime: 45,
				Hash:       "0000000000000000000000000000000000000000",
				Author:     "test@test.cz",
			},
		},
		Scale:     0,
		TileIndex: 0,
	})

	_, err = m.Get(0, 0, 1)
	if err != nil {
		t.Fatalf("Failed to error when requesting a merged tile that doesn't exist: %s", err)
	}
}
