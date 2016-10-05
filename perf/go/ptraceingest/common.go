package ptraceingest

import (
	"strings"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/ingestcommon"
)

// getValueMap returns a map[string]float32 of trace keys and their new values
// from the given BenchData.
func getValueMap(b *ingestcommon.BenchData) map[string]float32 {
	ret := make(map[string]float32, len(b.Results))
	for testName, allConfigs := range b.Results {
		for configName, result := range allConfigs {
			key := util.CopyStringMap(b.Key)
			key["test"] = testName
			key["config"] = configName
			util.AddParams(key, b.Options)

			// If there is an options map inside the result add it to the params.
			if resultOptions, ok := result["options"]; ok {
				if opts, ok := resultOptions.(map[string]interface{}); ok {
					for k, vi := range opts {
						// Ignore the very long and not useful GL_ values, we can retrieve
						// them later via ptracestore.Details.
						if strings.HasPrefix(k, "GL_") {
							continue
						}
						if s, ok := vi.(string); ok {
							key[k] = s
						}
					}
				}
			}

			for k, vi := range result {
				if k == "options" {
					continue
				}
				key["sub_result"] = k
				floatVal, ok := vi.(float64)
				if !ok {
					glog.Errorf("Found a non-float64 in %v", result)
					continue
				}
				keyString, err := query.MakeKey(query.ForceValid(key))
				if err != nil {
					glog.Errorf("Invalid structured key %v: %s", key, err)
					continue
				}
				ret[keyString] = float32(floatVal)
			}
		}
	}
	return ret
}
