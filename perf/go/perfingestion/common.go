package perfingestion

import (
	"fmt"
	"sort"
	"strings"

	"github.com/skia-dev/glog"

	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/ingestcommon"
	"go.skia.org/infra/perf/go/types"
)

const (
	// Configuration option that identifies the address of the traceDB service.
	CONFIG_TRACESERVICE = "TraceService"
)

// getTraceDBEntries returns a map of tracedb.Entry instances.
func getTraceDBEntries(b *ingestcommon.BenchData) map[string]*tracedb.Entry {
	ret := make(map[string]*tracedb.Entry, len(b.Results))
	keyPrefix := keyPrefix(b)
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

// keyPrefix returns the prefix that is common to all trace ids in a single
// instance of BenchData.
func keyPrefix(b *ingestcommon.BenchData) string {
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
