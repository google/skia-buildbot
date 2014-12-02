/* ingester loads JSON data from Google Storage and uses it to update the TileStore.

For example, in the case of nanobench data the file format looks like this:

  {
    "gitHash": "d1830323662ae8ae06908b97f15180fd25808894",
    "key": {
      "arch": "x86",
      "gpu": "GTX660",
      "os": "Ubuntu12",
      "model": "ShuttleA",
    },
    "options":{
        "system":"UNIX"
    },
    "results":{
        "DeferredSurfaceCopy_discardable_640_480":{
            "gpu":{
                "max_ms":7.9920480,
                "mean_ms":7.9920480,
                "median_ms":7.9920480,
                "min_ms":7.9920480,
                "options":{
                    "GL_RENDERER":"Quadro K600/PCIe/SSE2",
                    "GL_SHADING_LANGUAGE_VERSION":"4.40 NVIDIA via Cg compiler",
                    "GL_VENDOR":"NVIDIA Corporation",
                    "GL_VERSION":"4.4.0 NVIDIA 331.38"
                }
            },
            "nvprmsaa4":{
                "max_ms":16.7961230,
                "mean_ms":16.7961230,
                "median_ms":16.7961230,
                "min_ms":16.7961230,
                "options":{
                    "GL_RENDERER":"Quadro K600/PCIe/SSE2",
                    "GL_SHADING_LANGUAGE_VERSION":"4.40 NVIDIA via Cg compiler",
                    "GL_VENDOR":"NVIDIA Corporation",
                    "GL_VERSION":"4.4.0 NVIDIA 331.38"
                }
            }
        },
        ...

   Ingester converts that structure into Traces in Tiles.

   The key for a Trace is constructed from the "key" dictionary, along with the
   test name, the configuration name and the value being store. So, for
   example, the first value above will be store in the Trace with a key of:

     "x86:GTX660:Ubuntu12:ShuttleA:DeferredSurfaceCopy_discardable_640_480:gpu"

   Note that since we only record one value (min_ms for now) then we don't need
   to add that to the key.

   The Params for such a Trace will be the union of the "key" and all the
   related "options" dictionaries. Again, for the first value:

     "params": {
       "arch": "x86",
       "gpu": "GTX660",
       "os": "Ubuntu12",
       "model": "ShuttleA",
       "system":"UNIX"
       "GL_RENDERER":"Quadro K600/PCIe/SSE2",
       "GL_SHADING_LANGUAGE_VERSION":"4.40 NVIDIA via Cg compiler",
       "GL_VENDOR":"NVIDIA Corporation",
       "GL_VERSION":"4.4.0 NVIDIA 331.38"
     }

   If in the future we wanted to have Traces for both min_ms and median_ms
   then the keys would become:

     "x86:GTX660:Ubuntu12:ShuttleA:DeferredSurfaceCopy_discardable_640_480:gpu:min_ms"
     "x86:GTX660:Ubuntu12:ShuttleA:DeferredSurfaceCopy_discardable_640_480:gpu:median_ms"

   N.B. That would also require adding a synthetic option
   "value_type": ("min"|"median") to the Params for each Trace, so you could
   select out those different type of Traces in the UI.
*/
package ingester

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/types"

	"github.com/golang/glog"
	metrics "github.com/rcrowley/go-metrics"
)

// BenchResult represents a single test result.
//
// Used in BenchData.
type BenchResult struct {
	Min     float64           `json:"min_ms"`
	Options map[string]string `json:"options"`
}

// BenchResults is the dictionary of individual BenchResult structs.
//
// Used in BenchData.
type BenchResults map[string]*BenchResult

// BenchData is the top level struct for decoding the nanobench JSON format.
type BenchData struct {
	Hash    string                   `json:"gitHash"`
	Key     map[string]string        `json:"key"`
	Options map[string]string        `json:"options"`
	Results map[string]*BenchResults `json:"results"`
}

// KeyPrefix makes the first part of a Trace key by joining the parts of the
// BenchData Key value in sort order, i.e.
//
//   {"arch":"x86","model":"ShuttleA","gpu":"GTX660","os":"Ubuntu12"}
//
// should return:
//
//   "x86:GTX660:ShuttleA:Ubuntu12"
//
func (b BenchData) KeyPrefix() string {
	keys := make([]string, 0, len(b.Key))
	retval := make([]string, 0, len(b.Key))

	for k, _ := range b.Key {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		retval = append(retval, b.Key[k])
	}
	return strings.Join(retval, ":")
}

// ParseBenchDataFromReader parses the stream out of the io.ReadCloser into BenchData.
func ParseBenchDataFromReader(r io.ReadCloser) (*BenchData, error) {
	dec := json.NewDecoder(r)
	benchData := &BenchData{}
	defer r.Close()
	if err := dec.Decode(benchData); err != nil {
		glog.Warningf("Failed to decode JSON: %s", err)
		return nil, err
	}
	return benchData, nil
}

// addBenchDataToTile adds BenchData to a Tile.
//
// See the description at the top of this file for how the mapping works.
func addBenchDataToTile(benchData *BenchData, tile *types.Tile, offset int, counter metrics.Counter) {
	keyPrefix := benchData.KeyPrefix()
	for testName, allConfigs := range benchData.Results {
		for configName, result := range *allConfigs {
			key := fmt.Sprintf("%s:%s:%s", keyPrefix, testName, configName)

			// Construct the Traces params from all the options.
			params := map[string]string{
				"test":   testName,
				"config": configName,
			}
			for k, v := range benchData.Key {
				params[k] = v
			}
			for k, v := range benchData.Options {
				params[k] = v
			}
			for k, v := range result.Options {
				params[k] = v
			}

			var trace *types.PerfTrace
			var ok bool
			needsUpdate := false
			if tr, ok := tile.Traces[key]; !ok {
				trace = types.NewPerfTrace()
				tile.Traces[key] = trace
				needsUpdate = true
			} else {
				trace = tr.(*types.PerfTrace)
				if !util.MapsEqual(params, tile.Traces[key].Params()) {
					needsUpdate = true
				}
			}
			trace.Params_ = params

			if needsUpdate {
				// Update the Tile's ParamSet with any new keys or values we see.
				//
				// TODO(jcgregorio) Maybe defer this until we are about to Put the Tile
				// back to disk and rebuild ParamSet from scratch over all the Traces.
				for k, v := range params {
					if _, ok = tile.ParamSet[k]; !ok {
						tile.ParamSet[k] = []string{v}
					} else if !util.In(v, tile.ParamSet[k]) {
						tile.ParamSet[k] = append(tile.ParamSet[k], v)
					}
				}
			}
			if trace.Values[offset] != config.MISSING_DATA_SENTINEL {
				glog.Infof("Duplicate entry found for %s, hash %s", key, benchData.Hash)
			}
			trace.Values[offset] = result.Min
			counter.Inc(1)
		}
	}
}

func NanoBenchIngestion(tt *TileTracker, resultsFiles []*ResultsFileLocation, counter metrics.Counter) error {
	for _, b := range resultsFiles {
		// Load and parse the JSON.
		r, err := b.Fetch()
		if err != nil {
			// Don't fall over for a single failed HTTP request.
			continue
		}
		benchData, err := ParseBenchDataFromReader(r)
		if err != nil {
			// Don't fall over for a single corrupt file.
			continue
		}
		// Move to the correct Tile for the Git hash.
		hash := benchData.Hash
		if err := tt.Move(hash); err != nil {
			return fmt.Errorf("UpdateCommitInfo Move(%s) failed with: %s", hash, err)
		}
		// Add the parsed data to the Tile.
		addBenchDataToTile(benchData, tt.Tile(), tt.Offset(hash), counter)
	}
	return nil
}
