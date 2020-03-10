// Package parser has funcs for parsing ingestion files.
package parser

import (
	"io"
	"strings"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ingestcommon"
)

type Parser struct {
	instanceConfig *config.InstanceConfig
}

// getLegacyParamsAndValues returns two parallel slices, each slice contains the
// params and then the float for a single value of a trace.
func getLegacyParamsAndValues(b *ingestcommon.BenchData) ([]paramtools.Params, []float64) {
	params := []paramtools.Params{}
	values := []float64{}
	for testName, allConfigs := range b.Results {
		for configName, result := range allConfigs {
			key := paramtools.Params(b.Key).Copy()
			key["test"] = testName
			key["config"] = configName
			key.Add(paramtools.Params(b.Options))
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
				key = query.ForceValid(key)
				params = append(params, key.Copy())
				values = append(values, floatVal)
			}
		}
	}
	return params, values
}

func (p Parser) extractFromLegacyFile(r io.Reader, filename string) ([]paramtools.Params, []float64, string, error) {
	benchData, err := ingestcommon.ParseBenchDataFromReader(r)
	if err != nil {
		return nil, nil, "", err
	}
	branch, ok := benchData.Key["branch"]
	if ok {
		if len(g.config.IngestionConfig.Branches) > 0 {
			if !util.In(branch, g.config.IngestionConfig.Branches) {
				return nil, nil, "", skerr.Fmt("Data ")
			}
		}
	} else {
		sklog.Infof("No branch name.")
	}
	params, values := getLegacyParamsAndValues(benchData)
	if len(params) == 0 {
		metrics2.GetCounter("perf_ingest_no_data_in_file", map[string]string{"branch": branch}).Inc(1)
		sklog.Infof("No data in: %q", filename)
	}
	return params, values, benchData.Hash, nil
}

func ParseSourceFile(r io.Reader, filename string) ([]paramtools.Params, []float64, string, error) {
	defer util.Close(r)
	params, values, hash, err = g.extractFromLegacyFile(buf, filename)
	if err != nil {
		return nil, nil, "", err
	}
	// Don't do any more work if there's no data to ingest.
	if len(params) == 0 {
		return nil, nil, "", skerr.Fmt("No data in file: %q", filename)
	}
	return params, values, hash, nil
}
