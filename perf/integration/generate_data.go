// Generates dummy data to go along with the dummy repo at
// https://github.com/skia-dev/perf-demo-repo. It emits 9 good files and one
// file with an unknown git commit.
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"

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
		"ffffffffffffffffffffffffffffffffffffffff", // Unknown commit.
	}
	for i, hash := range hashes {
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
						"test": "encode",
					},
					Measurements: map[string][]format.SingleMeasurement{
						"ns": {
							{
								Value:       "min",
								Measurement: 10.1 + rand.Float32(),
							},
							{
								Value:       "max",
								Measurement: 12.2 + rand.Float32()*float32(i),
							},
						},
						"alloc": {
							{
								Value:       "kb",
								Measurement: 120,
							},
							{
								Value:       "num",
								Measurement: float32(7 + i/5),
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
