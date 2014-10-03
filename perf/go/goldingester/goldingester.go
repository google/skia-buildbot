package goldingester

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/golang/glog"
	"github.com/rcrowley/go-metrics"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/perf/go/ingester"
	"skia.googlesource.com/buildbot.git/perf/go/types"
)

var (
	goldenMetricsProcessed metrics.Counter
)

func Init() {
	goldenMetricsProcessed = metrics.NewRegisteredCounter("ingester.golden.processed", metrics.DefaultRegistry)
}

// The JSON output from DM looks like this:
//
//  {
//     "build_number" : "20",
//     "gitHash" : "abcd",
//     "key" : {
//        "arch" : "x86",
//        "gpu" : "nvidia",
//        "model" : "z620"
//     },
//     "results" : [
//        {
//           "key" : {
//              "config" : "565",
//              "name" : "ninepatch-stretch"
//           },
//           "md5" : "f78cfafcbabaf815f3dfcf61fb59acc7",
//           "options" : {
//              "source_type" : "GM"
//           }
//        },
//        {
//           "key" : {
//              "config" : "8888",
//              "name" : "ninepatch-stretch"
//           },
//           "md5" : "3e8a42f35a1e76f00caa191e6310d789",
//           "options" : {
//              "source_type" : "GM"
//           }

// DMResults is the top level structure for decoding DM JSON output.
type DMResults struct {
	BuildNumber string            `json:"build_number"`
	GitHash     string            `json:"gitHash"`
	Key         map[string]string `json:"key"`
	Results     []*Result         `json:"results"`
}

type Result struct {
	Key     map[string]string `json:"key"`
	Options map[string]string `json:"options"`
	Digest  string            `json:"md5"`
}

func NewDMResults() *DMResults {
	return &DMResults{
		Key:     map[string]string{},
		Results: []*Result{},
	}
}

// idAndParams constructs the Trace ID and the Trace params from the keys and options.
func idAndParams(dm *DMResults, r *Result) (string, map[string]string) {
	traceIdParts := map[string]string{}
	params := map[string]string{}
	for k, v := range dm.Key {
		traceIdParts[k] = v
		params[k] = v
	}
	for k, v := range r.Key {
		traceIdParts[k] = v
		params[k] = v
	}
	for k, v := range r.Options {
		params[k] = v
	}

	keys := []string{}
	for k, _ := range traceIdParts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	values := []string{}
	for _, k := range keys {
		values = append(values, traceIdParts[k])
	}
	return strings.Join(values, ":"), params
}

// addResultToTile adds the Digests from the DMResults to the tile at the given offset.
func addResultToTile(res *DMResults, tile *types.Tile, offset int) {
	for _, r := range res.Results {
		traceID, params := idAndParams(res, r)

		var trace *types.GoldenTrace
		var ok bool
		needsUpdate := false
		if tr, ok := tile.Traces[traceID]; !ok {
			trace = types.NewGoldenTrace()
			tile.Traces[traceID] = trace
			needsUpdate = true
		} else {
			trace = tr.(*types.GoldenTrace)
			if !util.MapsEqual(params, tile.Traces[traceID].Params()) {
				needsUpdate = true
			}
		}
		trace.Params_ = params

		if needsUpdate {
			// Update the Tile's ParamSet with any new keys or values we see.
			for k, v := range params {
				if _, ok = tile.ParamSet[k]; !ok {
					tile.ParamSet[k] = []string{v}
				} else if !util.In(v, tile.ParamSet[k]) {
					tile.ParamSet[k] = append(tile.ParamSet[k], v)
				}
			}
		}
		if trace.Values[offset] != types.MISSING_DIGEST {
			glog.Infof("Duplicate entry found for %s, hash %s", traceID, res.GitHash)
		}
		trace.Values[offset] = r.Digest
		goldenMetricsProcessed.Inc(1)
	}
}

// GoldenIngester implements perf/go/ingester.IngestResultsFiles for ingesting DM files into GoldenTraces.
func GoldenIngester(tt *ingester.TileTracker, resultsList []*ingester.ResultsFileLocation) error {
	glog.Infof("Ingesting: %v", resultsList)
	for _, resultLocation := range resultsList {
		r, err := resultLocation.Fetch()
		if err != nil {
			glog.Errorf("Failed to fetch: %s: %s", resultLocation.Name, err)
		}
		dec := json.NewDecoder(r)
		res := NewDMResults()
		if err := dec.Decode(res); err != nil {
			glog.Errorf("Failed to decode DM result: %s: %s", resultLocation.Name, err)
			continue
		}
		if res.GitHash != "" {
			glog.Infof("Got Git hash: %s", res.GitHash)
			if err := tt.Move(res.GitHash); err != nil {
				glog.Errorf("Failed to move to correct Tile: %s: %s", res.GitHash, err)
				continue
			}
			addResultToTile(res, tt.Tile(), tt.Offset(res.GitHash))
		} else {
			glog.Warning("Got file with missing hash: %s", resultLocation.Name)
		}
	}
	return nil
}
