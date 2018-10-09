package ptraceingest

import (
	"fmt"
	"strings"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/ingestcommon"
)

func addValueAtKey(ret map[string]float32, subResult string, key map[string]string, value float32) error {
	key["sub_result"] = subResult
	keyString, err := query.MakeKey(query.ForceValid(key))
	if err != nil {
		return fmt.Errorf("Invalid structured key %v: %s", key, err)
	}
	ret[keyString] = value
	return nil
}

// getValueMap returns a map[string]float32 of trace keys and their new values
// from the given BenchData.
func getValueMap(b *ingestcommon.BenchData) map[string]float32 {
	ret := make(map[string]float32, len(b.Results))
	for testName, allConfigs := range b.Results {
		for configName, result := range allConfigs {
			key := util.CopyStringMap(b.Key)
			if key == nil {
				key = map[string]string{}
			}
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
				if k == "options" || k == "samples" {
					continue
				}
				key["sub_result"] = k
				floatVal, ok := vi.(float64)
				if !ok {
					sklog.Errorf("Found a non-float64 in %v", result)
					continue
				}
				if err := addValueAtKey(ret, k, key, float32(floatVal)); err != nil {
					sklog.Warningf("Failed to add %s: %s", k, err)
				}
			}
		}
	}
	return ret
}
