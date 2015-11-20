package perfingestion

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/skia-dev/glog"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/types"
)

const (
	// Configuration option that identifies the address of the traceDB service.
	CONFIG_TRACESERVICE = "TraceService"
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
type BenchResults map[string]BenchResult

// BenchData is the top level struct for decoding the nanobench JSON format.
type BenchData struct {
	Hash     string                  `json:"gitHash"`
	Issue    string                  `json:"issue"`
	PatchSet string                  `json:"patchset"`
	Key      map[string]string       `json:"key"`
	Options  map[string]string       `json:"options"`
	Results  map[string]BenchResults `json:"results"`
}

// keyPrefix returns the prefix that is common to all trace ids in a single
// instance of BenchData.
func (b *BenchData) keyPrefix() string {
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

// getTraceDBEntries returns a map of tracedb.Entry instances.
func (b *BenchData) getTraceDBEntries() map[string]*tracedb.Entry {
	ret := make(map[string]*tracedb.Entry, len(b.Results))
	keyPrefix := b.keyPrefix()
	for testName, allConfigs := range b.Results {
		for configName, result := range allConfigs {
			key := fmt.Sprintf("%s:%s:%s", keyPrefix, testName, configName)

			// Construct the Traces params from all the options.
			params := util.CopyStringMap(b.Key)
			params["test"] = testName
			params["config"] = configName
			util.AddParams(params, b.Options)

			// If there is an options map inside the result add it to the params.
			if resultOptions, ok := result["options"]; ok {
				if opts, ok := resultOptions.(map[string]interface{}); ok {
					for k, vi := range opts {
						if s, ok := vi.(string); ok {
							params[k] = s
						}
					}
				}
			}

			// We used to just pick out only "min_ms" as the only result of a bunch
			// of key, value pairs in the result, such as max_ms, mean_ms, etc. Now
			// nanobench uploads only the metrics we are interested in, so we need to
			// use all the values there, except 'options'.
			for k, vi := range result {
				if k == "options" {
					continue
				}

				floatVal, ok := vi.(float64)
				if !ok {
					glog.Errorf("Found a non-float64 in %s", key)
					continue
				}

				params["sub_result"] = k
				perResultKey := key
				if k != "min_ms" {
					perResultKey = fmt.Sprintf("%s:%s", perResultKey, k)
				}
				paramsCopy := util.CopyStringMap(params)
				ret[perResultKey] = &tracedb.Entry{
					Params: paramsCopy,
					Value:  types.BytesFromFloat64(floatVal),
				}
			}
		}
	}
	return ret
}

// parseBenchDataFromReader parses the stream out of the io.ReadCloser
// into BenchData and closes the reader.
func parseBenchDataFromReader(r io.ReadCloser) (*BenchData, error) {
	defer util.Close(r)

	dec := json.NewDecoder(r)
	benchData := &BenchData{}
	if err := dec.Decode(benchData); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON: %s", err)
	}
	return benchData, nil
}
