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
                "min_ms":7.9920480,
                "options":{
                    "GL_RENDERER":"Quadro K600/PCIe/SSE2",
                    "GL_SHADING_LANGUAGE_VERSION":"4.40 NVIDIA via Cg compiler",
                    "GL_VENDOR":"NVIDIA Corporation",
                    "GL_VERSION":"4.4.0 NVIDIA 331.38"
                }
            },
            "nvprmsaa4":{
                "min_ms":16.7961230,
                "options":{
                    "GL_RENDERER":"Quadro K600/PCIe/SSE2",
                    "GL_SHADING_LANGUAGE_VERSION":"4.40 NVIDIA via Cg compiler",
                    "GL_VENDOR":"NVIDIA Corporation",
                    "GL_VERSION":"4.4.0 NVIDIA 331.38"
                }
            }
        },
        "memory_usage_0_0" : {
           "meta" : {
              "max_rss_mb" : 858
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

   If Traces have data beyond min_ms then the keys become:

     "x86:GTX660:Ubuntu12:ShuttleA:DeferredSurfaceCopy_discardable_640_480:gpu"
     "x86:GTX660:Ubuntu12:ShuttleA:DeferredSurfaceCopy_discardable_640_480:gpu:median_ms"

   There is a synthetic parameter added to the Params for each Trace, so you
   can select out those different type of Traces in the UI.

     "sub_result": ("min_ms"|"median_ms")
*/
package ingester

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/types"

	metrics "github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
)

// BenchResult represents a single test result.
//
// Used in BenchData.
//
// Expected to be a map of strings to float64s, with the
// exception of the "options" entry which should be a
// map[string]string.
type BenchResult map[string]interface{}

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

// Iter defines a callback function used in BenchData.ForEach().
type Iter func(key string, value float64, params map[string]string)

// ForEach takes a callback of type Iter that is called once for every data point
// found in the BenchData.
func (b BenchData) ForEach(f Iter) {
	keyPrefix := b.KeyPrefix()
	for testName, allConfigs := range b.Results {
		for configName, result := range *allConfigs {
			key := fmt.Sprintf("%s:%s:%s", keyPrefix, testName, configName)

			// Construct the Traces params from all the options.
			params := map[string]string{
				"test":   testName,
				"config": configName,
			}
			for k, v := range b.Key {
				params[k] = v
			}
			for k, v := range b.Options {
				params[k] = v
			}
			if options, ok := (*result)["options"]; ok {
				for k, vi := range options.(map[string]interface{}) {
					if s, ok := vi.(string); ok {
						params[k] = s
					}
				}
			}

			// We used to just pick out only "min_ms" as the only result of a bunch
			// of key, value pairs in the result, such as max_ms, mean_ms, etc. Now
			// nanobench uploads only the metrics we are interested in, so we need to
			// use all the values there, except 'options'.
			for k, vi := range *result {
				if k == "options" {
					continue
				}
				if _, ok := vi.(float64); !ok {
					glog.Errorf("Found a non-float64 in %s: %v", key, vi)
					continue
				}
				params["sub_result"] = k
				perResultKey := key
				if k != "min_ms" {
					perResultKey = fmt.Sprintf("%s:%s", perResultKey, k)
				}
				paramsCopy := make(map[string]string, len(params))
				for k, v := range params {
					paramsCopy[k] = v
				}

				f(perResultKey, vi.(float64), paramsCopy)
			}
		}
	}
}

// ParseBenchDataFromReader parses the stream out of the io.ReadCloser
// into BenchData and closes the reader.
func ParseBenchDataFromReader(r io.ReadCloser) (*BenchData, error) {
	defer r.Close()

	dec := json.NewDecoder(r)
	benchData := &BenchData{}
	if err := dec.Decode(benchData); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON: %s", err)
	}
	return benchData, nil
}

// addBenchDataToTile adds BenchData to a Tile.
//
// See the description at the top of this file for how the mapping works.
func addBenchDataToTile(benchData *BenchData, tile *types.Tile, offset int, counter metrics.Counter) {

	// cb is the anonymous closure we'll pass over all the trace values found in benchData.
	cb := func(key string, value float64, params map[string]string) {
		needsUpdate := false
		var trace *types.PerfTrace
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
		trace.Values[offset] = value
		counter.Inc(1)

		if needsUpdate {
			// Update the Tile's ParamSet with any new keys or values we see.
			//
			// TODO(jcgregorio) Maybe defer this until we are about to Put the Tile
			// back to disk and rebuild ParamSet from scratch over all the Traces.
			for k, v := range params {
				if _, ok := tile.ParamSet[k]; !ok {
					tile.ParamSet[k] = []string{v}
				} else if !util.In(v, tile.ParamSet[k]) {
					tile.ParamSet[k] = append(tile.ParamSet[k], v)
				}
			}
		}
	}

	benchData.ForEach(cb)
}

// NanoBenchIngester implements the ingester.ResultIngester interface.
type NanoBenchIngester struct{}

func NewNanoBenchIngester() ResultIngester {
	return NanoBenchIngester{}
}

func init() {
	Register("nano", NewNanoBenchIngester)
}

// See the ingester.ResultIngester interface.
func (i NanoBenchIngester) Ingest(tt *TileTracker, opener Opener, fname string, counter metrics.Counter) error {
	r, err := opener()
	if err != nil {
		return err
	}

	benchData, err := ParseBenchDataFromReader(r)
	if err != nil {
		return err
	}

	// Move to the correct Tile for the Git hash.
	hash := benchData.Hash
	if hash == "" {
		return fmt.Errorf("Found invalid hash: %s", hash)
	}

	if err := tt.Move(hash); err != nil {
		return fmt.Errorf("UpdateCommitInfo Move(%s) failed with: %s", hash, err)
	}

	// Add the parsed data to the Tile.
	addBenchDataToTile(benchData, tt.Tile(), tt.Offset(hash), counter)
	return nil
}

// See the ingester.ResultIngester interface.
func (i NanoBenchIngester) BatchFinished(counter metrics.Counter) error {
	return nil
}
