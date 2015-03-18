package goldingester

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	metrics "github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/androidbuild"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ingester"
	"go.skia.org/infra/perf/go/types"
)

// Init registers the GoldIngester and the Android specific GoldIngester.
func Init(client *http.Client) error {
	gitHashLookup, err := androidbuild.New(client)
	if err != nil {
		return err
	}

	// Generate the pre-ingestion hook and register the ingester.
	preIngestHook := getAndroidGoldPreIngestHook(gitHashLookup)
	ingester.Register(config.CONSTRUCTOR_ANDROID_GOLD, func() ingester.ResultIngester { return NewGoldIngester(preIngestHook) })
	ingester.Register(config.CONSTRUCTOR_GOLD, func() ingester.ResultIngester { return NewGoldIngester(nil) })
	return nil
}

// The JSON output from DM looks like this:
//
//  {
//     "build_number" : "20",
//     "gitHash" : "abcd",
//     "key" : {
//        "arch" : "x86",
//        "configuration" : "Debug",
//        "gpu" : "nvidia",
//        "model" : "z620",
//        "os" : "Ubuntu13.10"
//     },
//     "results" : [
//        {
//           "key" : {
//              "config" : "565",
//              "name" : "ninepatch-stretch",
//              "source_type" : "gm"
//           },
//           "md5" : "f78cfafcbabaf815f3dfcf61fb59acc7",
//           "options" : {
//              "ext" : "png"
//           }
//        },
//        {
//           "key" : {
//              "config" : "8888",
//              "name" : "ninepatch-stretch",
//              "source_type" : "gm"
//           },
//           "md5" : "3e8a42f35a1e76f00caa191e6310d789",
//           "options" : {
//              "ext" : "png"
//           }

// DMResults is the top level structure for decoding DM JSON output.
type DMResults struct {
	BuildNumber string            `json:"build_number"`
	GitHash     string            `json:"gitHash"`
	Key         map[string]string `json:"key"`
	Results     []*Result         `json:"results"`
}

// PreIngestionHook is a function that changes a DMResult after it was parsed
// from Json, but before it is ingested into the tile.
type PreIngestionHook func(dmResult *DMResults) error

type Result struct {
	Key     map[string]string `json:"key"`
	Options map[string]string `json:"options"`
	Digest  string            `json:"md5"`
}

// ResultAlias is a type alias to avoid recursive calls to UnmarshalJSON.
type ResultAlias Result

func (r *Result) UnmarshalJSON(data []byte) error {
	ret := ResultAlias{
		Key:     map[string]string{},
		Options: map[string]string{},
	}
	err := json.Unmarshal(data, &ret)
	if err != nil {
		return err
	}
	*r = Result(ret)

	// Make sure ext is set.
	if ext, ok := r.Options["ext"]; !ok || ext == "" {
		r.Options["ext"] = "png"
	}
	return nil
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
func addResultToTile(res *DMResults, tile *types.Tile, offset int, counter metrics.Counter) {
	for _, r := range res.Results {
		if ext, ok := r.Options["ext"]; !ok || ext != "png" {
			continue // Temporarily skip non-PNG results until we know how to ingest them.
		}

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
		trace.Values[offset] = r.Digest
		counter.Inc(1)
	}
}

// GoldIngester implements the ingester.ResultIngester interface.
type GoldIngester struct {
	preIngestionHook PreIngestionHook
}

func NewGoldIngester(hook PreIngestionHook) ingester.ResultIngester {
	return GoldIngester{
		preIngestionHook: hook,
	}
}

// See the ingester.ResultIngester interface.
func (i GoldIngester) Ingest(tt *ingester.TileTracker, opener ingester.Opener, fname string, counter metrics.Counter) error {
	r, err := opener()
	if err != nil {
		return err
	}
	defer util.Close(r)

	dec := json.NewDecoder(r)
	res := NewDMResults()
	if err := dec.Decode(res); err != nil {
		return fmt.Errorf("Failed to decode DM result: %s", err)
	}

	// Run the pre-ingestion hook.
	if i.preIngestionHook != nil {
		if err := i.preIngestionHook(res); err != nil {
			return fmt.Errorf("Error running pre-ingestion hook: %s", err)
		}
	}

	if res.GitHash != "" {
		glog.Infof("Got Git hash: %s", res.GitHash)
		if err := tt.Move(res.GitHash); err != nil {
			return fmt.Errorf("Failed to move to correct Tile: %s: %s", res.GitHash, err)
		}
		addResultToTile(res, tt.Tile(), tt.Offset(res.GitHash), counter)
	} else {
		return fmt.Errorf("Missing hash.")
	}

	return nil
}

// See the ingester.ResultIngester interface.
func (i GoldIngester) BatchFinished(counter metrics.Counter) error {
	return nil
}
