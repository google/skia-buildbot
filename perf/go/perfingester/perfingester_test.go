package perfingester

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	metrics "github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/filetilestore"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/ingester"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/validator"
)

func init() {
	ingester.Init(nil)
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
	defer testutils.RemoveAll(t, tileDir)

	git, err := gitinfo.NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false, false)
	if err != nil {
		glog.Fatalf("Failed loading Git info: %s\n", err)
	}

	// Construct an Ingestor and have it UpdateCommitInfo.
	i, err := ingester.NewIngester(git, tileDir, config.DATASET_NANO, NewNanoBenchIngester(), 1, time.Second, map[string]string{}, "", "")
	if err != nil {
		t.Fatal("Failed to create ingester:", err)
	}

	if err := i.UpdateCommitInfo(false); err != nil {
		t.Fatal("Failed to ingest commits:", err)
	}

	// Validate the generated Tiles.
	store := filetilestore.NewFileTileStore(tileDir, config.DATASET_NANO, 0)
	if !validator.ValidateDataset(store, false, false) {
		t.Error("Failed to validate the created Tiles:", err)
	}

	// Test TileTracker while were here.
	tt := ingester.NewTileTracker(store, i.HashToNumber())
	err = tt.Move("7a6fe813047d1a84107ef239e81f310f27861473")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := tt.LastTileNum(), 2; got != want {
		t.Errorf("Move failed, wrong tile: Got %d Want %d", got, want)
	}
	err = tt.Move("87709bc360f35de52c2f2bc2fc70962fb234db2d")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := tt.LastTileNum(), 2; got != want {
		t.Errorf("Move failed, wrong tile: Got %d Want %d", got, want)
	}
}

func TestAddBenchDataToTile(t *testing.T) {
	// Load the sample data file as BenchData.
	_, filename, _, _ := runtime.Caller(0)
	r, err := os.Open(filepath.Join(filepath.Dir(filename), "testdata", "nano.json"))
	if err != nil {
		t.Fatal("Failed to open test file: ", err)
	}

	benchData, err := ParseBenchDataFromReader(r)
	if err != nil {
		t.Fatal("Failed to parse test file: ", err)
	}
	metricsProcessed := metrics.NewRegisteredCounter("testing.ingestion.processed", metrics.DefaultRegistry)

	// Create an empty Tile.
	tile := tiling.NewTile()
	tile.Scale = 0
	tile.TileIndex = 0

	offset := 1
	testcases := []struct {
		key       string
		value     float64
		subResult string
	}{
		{
			key:       "x86:GTX660:ShuttleA:Ubuntu12:DeferredSurfaceCopy_discardable_640_480:gpu",
			value:     0.1157132745098039,
			subResult: "min_ms",
		},
		{
			key:       "x86:GTX660:ShuttleA:Ubuntu12:memory_usage_0_0:meta:max_rss_mb",
			value:     858,
			subResult: "max_rss_mb",
		},
		{
			key:       "x86:GTX660:ShuttleA:Ubuntu12:src_pipe_global_weak_symbol:memory:bytes",
			value:     158,
			subResult: "bytes",
		},
		{
			key:       "x86:GTX660:ShuttleA:Ubuntu12:DeferredSurfaceCopy_nonDiscardable_640_480:8888",
			value:     2.855735,
			subResult: "min_ms",
		},
		{
			key:       "x86:GTX660:ShuttleA:Ubuntu12:DeferredSurfaceCopy_nonDiscardable_640_480:8888:bytes",
			value:     298888,
			subResult: "bytes",
		},
		{
			key:       "x86:GTX660:ShuttleA:Ubuntu12:DeferredSurfaceCopy_nonDiscardable_640_480:8888:ops",
			value:     3333,
			subResult: "ops",
		},
	}
	// Do everything twice to ensure that we are idempotent.
	for i := 0; i < 2; i++ {
		// Add the BenchData to the Tile.
		addBenchDataToTile(benchData, tile, offset, metricsProcessed)

		// Test that the Tile has the right data.
		if got, want := len(tile.Traces), 13; got != want {
			t.Errorf("Wrong number of traces: Got %d Want %d", got, want)
		}
		for _, tc := range testcases {
			trace, ok := tile.Traces[tc.key]
			if !ok {
				t.Errorf("Missing expected key: %s", tc.key)
			}
			if got, want := trace.(*types.PerfTrace).Values[offset], tc.value; got != want {
				t.Errorf("Wrong value in trace: Got %v Want %v", got, want)
			}
		}
		trace := tile.Traces[testcases[0].key]

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
			"sub_result":                  "min_ms",
		}
		if got, want := len(trace.Params()), len(expected); got != want {
			t.Errorf("Params wrong length: Got %v Want %v", got, want)
		}
		for k, v := range expected {
			if got, want := trace.Params()[k], v; got != want {
				t.Errorf("Wrong params: Got %v Want %v", got, want)
			}
		}

		// Validate the Tiles ParamSet.
		if got, want := len(tile.ParamSet), len(expected)+2; got != want {
			t.Errorf("Wrong ParamSet length: Got %v Want %v", got, want)
		}
		for k, _ := range expected {
			if _, ok := tile.ParamSet[k]; !ok {
				t.Errorf("Missing from ParamSet: %s", k)
			}
		}
		// The new symbol table size options values should also show up in the ParamSet.
		for _, k := range []string{"path", "symbol"} {
			if _, ok := tile.ParamSet[k]; !ok {
				t.Errorf("Missing from ParamSet: %s", k)
			}
		}

		if got, want := len(tile.ParamSet["source_type"]), 1; got != want {
			t.Errorf("Wrong ParamSet for source_type: Got %v Want %v", got, want)
		}
		if got, want := tile.ParamSet["source_type"][0], "bench"; got != want {
			t.Errorf("Wrong ParamSet value: Got %v Want %v", got, want)
		}
	}

	if got, want := metricsProcessed.Count(), int64(26); got != want {
		t.Errorf("Wrong number of points ingested: Got %v Want %v", got, want)
	}
	// Now update one of the params for a trace and reingest and confirm that the
	// trace params get updated.

	benchData.Options["system"] = "Linux"
	addBenchDataToTile(benchData, tile, offset, metricsProcessed)
	if got, want := "Linux", tile.Traces[testcases[0].key].Params()["system"]; got != want {
		t.Errorf("Failed to update params: Got %v Want %v", got, want)
	}
	if got, want := metricsProcessed.Count(), int64(39); got != want {
		t.Errorf("Wrong number of points ingested: Got %v Want %v", got, want)
	}
}
