package ingester

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"skia.googlesource.com/buildbot.git/perf/go/filetilestore"
	"skia.googlesource.com/buildbot.git/perf/go/types"
	"skia.googlesource.com/buildbot.git/perf/go/util"
	"skia.googlesource.com/buildbot.git/perf/go/validator"
)

func init() {
	Init()
}

func TestIngestCommits(t *testing.T) {
	// Get a known Git repo with 34 commits in it setup.
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	// Create a temporary place for a filetilestore.
	tileDir, err := ioutil.TempDir("", "skiaperf")
	if err != nil {
		t.Fatal("Failed to create testing Tile dir: ", err)
	}
	defer os.RemoveAll(tileDir)

	// Construct an Ingestor and have it UpdateCommitInfo.
	i, err := NewIngester(filepath.Join(tr.Dir, "testrepo"), tileDir, false, NanoBenchIngestion, "")
	if err != nil {
		t.Fatal("Failed to create ingester:", err)
	}

	if err := i.UpdateCommitInfo(false); err != nil {
		t.Fatal("Failed to ingest commits:", err)
	}

	// Validate the generated Tiles.
	store := filetilestore.NewFileTileStore(tileDir, "nano", 0)
	if !validator.ValidateDataset(store, false, false) {
		t.Error("Failed to validate the created Tiles:", err)
	}

	// Test TileTracker while were here.
	tt := NewTileTracker(store, i.hashToNumber)
	tt.Move("7a6fe813047d1a84107ef239e81f310f27861473")
	if got, want := tt.lastTileNum, 1; got != want {
		t.Errorf("Move failed, wrong tile: Got %d Want %d", got, want)
	}
	tt.Move("87709bc360f35de52c2f2bc2fc70962fb234db2d")
	if got, want := tt.lastTileNum, 0; got != want {
		t.Errorf("Move failed, wrong tile: Got %d Want %d", got, want)
	}
}

func TestAddBenchDataToTile(t *testing.T) {
	// Load the sample data file as BenchData.
	_, filename, _, _ := runtime.Caller(0)
	f, err := os.Open(filepath.Join(filepath.Dir(filename), "testdata", "nano.json"))
	if err != nil {
		t.Fatal("Failed to open test file: ", err)
	}
	defer f.Close()
	benchData, err := ParseBenchDataFromReader(f)
	if err != nil {
		t.Fatal("Failed to parse test file: ", err)
	}

	// Create an empty Tile.
	tile := types.NewTile()
	tile.Scale = 0
	tile.TileIndex = 0

	offset := 1
	key := "x86:GTX660:ShuttleA:Ubuntu12:DeferredSurfaceCopy_discardable_640_480:gpu"
	// Do everything twice to ensure that we are idempotent.
	for i := 0; i < 2; i++ {
		// Add the BenchData to the Tile.
		addBenchDataToTile(benchData, tile, offset)

		// Test that the Tile has the right data.
		if got, want := len(tile.Traces), 9; got != want {
			fmt.Errorf("Wrong number of traces: Got %d Want %d", got, want)
		}
		trace, ok := tile.Traces[key]
		if !ok {
			fmt.Errorf("Missing expected key: %s", key)
		}
		if got, want := trace.(*types.PerfTrace).Values[offset], 0.1157132745098039; got != want {
			fmt.Errorf("Wrong value in trace: Got %v Want %v", got, want)
		}

		// Validate the traces Params.
		expected := map[string]string{
			"arch":                        "x86",
			"gpu":                         "GTX660",
			"model":                       "ShuttleA",
			"os":                          "Ubuntu12",
			"system":                      "UNIX",
			"test":                        "DeferredSurfaceCopy_discardable_640_480",
			"config":                      "gpu",
			"GL_RENDERER":                 "GeForce GTX 660/PCIe/SSE2",
			"GL_SHADING_LANGUAGE_VERSION": "4.40 NVIDIA via Cg compiler",
			"GL_VENDOR":                   "NVIDIA Corporation",
			"GL_VERSION":                  "4.4.0 NVIDIA 331.49",
			"source_type":                 "bench",
		}
		if got, want := len(trace.Params()), len(expected); got != want {
			fmt.Errorf("Params wrong length: Got %v Want %v", got, want)
		}
		for k, v := range expected {
			if got, want := trace.Params()[k], v; got != want {
				fmt.Errorf("Wrong params: Got %v Want %v", got, want)
			}
		}

		// Validate the Tiles ParamSet.
		if got, want := len(tile.ParamSet), len(expected); got != want {
			fmt.Errorf("Wrong ParamSet length: Got %v Want %v", got, want)
		}
		if got, want := len(tile.ParamSet["source_type"]), 1; got != want {
			fmt.Errorf("Wrong ParamSet for source_type: Got %v Want %v", got, want)
		}
		if got, want := tile.ParamSet["source_type"][0], "bench"; got != want {
			fmt.Errorf("Wrong ParamSet value: Got %v Want %v", got, want)
		}
	}

	// Now update one of the params for a trace and reingest and confirm that the
	// trace params get updated.

	benchData.Options["system"] = "Linux"
	addBenchDataToTile(benchData, tile, offset)
	if got, want := "Linux", tile.Traces[key].Params()["system"]; got != want {
		t.Errorf("Failed to update params: Got %v Want %v", got, want)
	}
}
