// Package parser has funcs for parsing ingestion files.
package parser

import (
	"errors"
	"io"
	"strings"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/file"
	"go.skia.org/infra/perf/go/ingest/format"
)

var (
	// ErrFileShouldBeSkipped is returned if a file should be skipped.
	ErrFileShouldBeSkipped = errors.New("File should be skipped.")
)

// Parser parses file.Files contents into a form suitable for writing to trace.Store.
type Parser struct {
	instanceConfig   *config.InstanceConfig
	parseCounter     metrics2.Counter
	parseFailCounter metrics2.Counter
}

// New creates a new instance of Parser.
func New(instanceConfig *config.InstanceConfig) *Parser {
	return &Parser{
		instanceConfig:   instanceConfig,
		parseCounter:     metrics2.GetCounter("perf_ingest_parser_parse", nil),
		parseFailCounter: metrics2.GetCounter("perf_ingest_parser_parse_failed", nil),
	}
}

// getParamsAndValues returns two parallel slices, each slice contains the
// params and then the float for a single value of a trace.
func getParamsAndValues(b *format.BenchData) ([]paramtools.Params, []float64) {
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

func (p *Parser) extractFromFile(r io.Reader, filename string) ([]paramtools.Params, []float64, string, error) {
	benchData, err := format.ParseBenchDataFromReader(r)
	if err != nil {
		return nil, nil, "", err
	}
	branch, ok := benchData.Key["branch"]
	if ok {
		if len(p.instanceConfig.IngestionConfig.Branches) > 0 {
			if !util.In(branch, p.instanceConfig.IngestionConfig.Branches) {
				return nil, nil, "", ErrFileShouldBeSkipped
			}
		}
	} else {
		sklog.Infof("No branch name.")
	}
	params, values := getParamsAndValues(benchData)
	if len(params) == 0 {
		metrics2.GetCounter("perf_ingest_parser_no_data_in_file", map[string]string{"branch": branch}).Inc(1)
		sklog.Infof("No data in: %q", filename)
		return nil, nil, "", ErrFileShouldBeSkipped
	}
	return params, values, benchData.Hash, nil
}

// Parse the given file.File contents.
//
// Returns two parallel slices, each slice contains the params and then the
// float for a single value of a trace.
//
// The returned error will be ErrFileShouldBeSkipped if the file should not be
// processed any further.
//
// The File.Contents will be closed when this func returns.
func (p *Parser) Parse(file file.File) ([]paramtools.Params, []float64, string, error) {
	p.parseCounter.Inc(1)
	defer util.Close(file.Contents)
	params, values, hash, err := p.extractFromFile(file.Contents, file.Name)
	if err != nil && err != ErrFileShouldBeSkipped {
		p.parseFailCounter.Inc(1)
	}
	if err != nil {
		return nil, nil, "", err
	}

	// Don't do any more work if there's no data to ingest.
	if len(params) == 0 {
		return nil, nil, "", ErrFileShouldBeSkipped
	}
	return params, values, hash, nil
}
