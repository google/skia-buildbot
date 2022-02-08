// Generates demo data to go along with the demo repo at
// https://github.com/skia-dev/perf-demo-repo.
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"runtime"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/ingest/format"
)

func main() {
	// These git hashes come from the https://github.com/skia-dev/perf-demo-repo
	// repo.
	hashes := []string{
		"fcd63691360443c852ab3bd832d0a9be7596e2d5",
		"a872080a357d0c8a4d5608c5dbb79c6132248c94",
		"6286eccdf042751401f54696ad38de9f6849284d",
		"04cfbf7e7ce2139ed3fd58a368e80f72a967d57e",
		"bc8f0a0b48efd7e90522f78b603aa0cbb24a39b0",
		"6d65a77667b810f159f689a1d6838f0c443322ff",
		"38485885d3d3c5de086d4e67f68879e9456f551e",
		"977e0ef44bec17659faf8c5d4025c5a068354817",
		"6079a7810530025d9877916895dd14eb8bb454c0",
	}
	_, filename, _, _ := runtime.Caller(0)
	err := os.MkdirAll(path.Join(path.Dir(filename), "data"), 0755)
	if err != nil {
		sklog.Fatal(err)
	}

	for i, hash := range hashes {
		encode := 50 + 3*rand.Float32()
		multiplier := float32(1.0)
		if i >= 5 {
			multiplier = 1.2
		}
		decode := 10.0*multiplier + rand.Float32()
		encodeMemory := 237 - multiplier*30
		f := format.Format{
			Version: format.FileFormatVersion,
			GitHash: hash,
			Key: map[string]string{
				"arch":   "x86",
				"config": "8888",
			},
			Results: []format.Result{
				{
					Key: map[string]string{
						"units": "ms",
					},
					Measurements: map[string][]format.SingleMeasurement{
						"test": {
							{
								Value:       "encode",
								Measurement: encode,
							},
							{
								Value:       "decode",
								Measurement: decode,
							},
						},
					},
				},
				{
					Key: map[string]string{
						"units": "kb",
					},
					Measurements: map[string][]format.SingleMeasurement{
						"test": {
							{
								Value:       "encode",
								Measurement: encodeMemory,
							},
							{
								Value:       "decode",
								Measurement: 65,
							},
						},
					},
				},
			},
		}
		b, err := json.MarshalIndent(f, "", "  ")
		if err != nil {
			sklog.Fatal(err)
		}
		if err := ioutil.WriteFile(fmt.Sprintf("./data/demo_data_commit_%d.json", i+1), b, 0644); err != nil {
			sklog.Fatal(err)
		}
	}
}
