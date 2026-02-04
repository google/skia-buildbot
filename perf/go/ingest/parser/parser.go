// Package parser has funcs for parsing ingestion files.
package parser

import (
	"bytes"
	"context"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/file"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/types"
)

var (
	// ErrFileShouldBeSkipped is returned if a file should be skipped.
	ErrFileShouldBeSkipped = errors.New("File should be skipped.")
)

// Parser parses file.Files contents into a form suitable for writing to trace.Store.
type Parser struct {
	parseCounter          metrics2.Counter
	parseFailCounter      metrics2.Counter
	branchNames           map[string]bool
	invalidParamCharRegex *regexp.Regexp
}

// New creates a new instance of Parser for the given branch names
// and invalid chars of parameter key/value
func New(ctx context.Context, instanceConfig *config.InstanceConfig) (parser *Parser, err error) {
	branches := instanceConfig.IngestionConfig.Branches

	invalidParamCharRegex := query.InvalidChar
	if instanceConfig.InvalidParamCharRegex != "" {
		invalidParamCharRegex, err = regexp.Compile(instanceConfig.InvalidParamCharRegex)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	ret := &Parser{
		parseCounter:          metrics2.GetCounter("perf_ingest_parser_parse", nil),
		parseFailCounter:      metrics2.GetCounter("perf_ingest_parser_parse_failed", nil),
		branchNames:           map[string]bool{},
		invalidParamCharRegex: invalidParamCharRegex,
	}
	for _, branchName := range branches {
		ret.branchNames[branchName] = true
	}

	return ret, nil
}

// buildInitialParams returns a Params for the given BenchResult.
func buildInitialParams(testName, configName string, b *format.BenchData, result format.BenchResult) paramtools.Params {
	ret := paramtools.Params(b.Key).Copy()
	ret["test"] = testName
	ret["config"] = configName
	ret.Add(paramtools.Params(b.Options))
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
					ret[k] = s
				}
			}
		}
	}
	return ret
}

// getParamsAndValuesFromLegacyFormat returns two parallel slices, each slice
// contains the params and then the float for a single value of a trace.
func getParamsAndValuesFromLegacyFormat(b *format.BenchData) ([]paramtools.Params, []float32) {
	params := []paramtools.Params{}
	values := []float32{}
	for testName, allConfigs := range b.Results {
		for configName, result := range allConfigs {
			key := buildInitialParams(testName, configName, b, result)
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
				values = append(values, float32(floatVal))
			}
		}
	}
	return params, values
}

// Samples contain multiple runs of the same test, where Params describes the
// test.
type Samples struct {
	Params paramtools.Params
	Values []float64
}

// SamplesSet maps trace names to Samples for that trace.
type SamplesSet map[string]Samples

// Add all the Samples from 'in'.
func (s SamplesSet) Add(in SamplesSet) {
	for key, samples := range in {
		existingSamples, ok := s[key]
		if !ok {
			existingSamples = Samples{
				Params: samples.Params.Copy(),
				Values: []float64{},
			}
		}
		existingSamples.Values = append(existingSamples.Values, samples.Values...)
		s[key] = existingSamples
	}
}

// GetSamplesFromLegacyFormat returns a map from trace id to the slice of
// samples for that test.
func GetSamplesFromLegacyFormat(b *format.BenchData) SamplesSet {
	ret := SamplesSet{}
	for testName, allConfigs := range b.Results {
		for configName, result := range allConfigs {
			params := buildInitialParams(testName, configName, b, result)
			iSamples, ok := result["samples"]
			if !ok {
				continue
			}
			// We only collect samples for min_ms.
			params["sub_result"] = "min_ms"

			params = query.ForceValid(params)
			traceID, err := query.MakeKeyFast(params)
			if err != nil {
				continue
			}
			iSlice := iSamples.([]interface{})
			values := make([]float64, 0, len(iSlice))
			for _, ix := range iSlice {
				x, ok := ix.(float64)
				if !ok {
					continue
				}
				values = append(values, x)
			}
			ret[traceID] = Samples{
				Params: params,
				Values: values,
			}
		}
	}
	return ret
}

// getParamsAndValuesFromVersion1Format returns two parallel slices, each slice contains
// the params and then the float for a single value of a trace.
func getParamsAndValuesFromVersion1Format(f format.Format, invalidParamCharRegex *regexp.Regexp) ([]paramtools.Params, []float32) {
	paramSlice := []paramtools.Params{}
	keyParams := paramtools.Params(f.Key)
	measurementSlice := []float32{}
	for _, result := range f.Results {
		p := keyParams.Copy()
		p.Add(result.Key)
		if len(result.Measurements) == 0 {
			paramSlice = append(paramSlice, query.ForceValidWithRegex(p, invalidParamCharRegex))
			measurementSlice = append(measurementSlice, result.Measurement)
		} else {
			for key, measurements := range result.Measurements {
				for _, measurement := range measurements {
					singleParam := p.Copy()
					singleParam[key] = measurement.Value
					paramSlice = append(paramSlice, query.ForceValidWithRegex(singleParam, invalidParamCharRegex))
					measurementSlice = append(measurementSlice, measurement.Measurement)
				}
			}

		}
	}
	return paramSlice, measurementSlice
}

// checkBranchName returns the branch name and true if the file should continue
// to be processed. Note that if the 'params' don't contain a key named 'branch'
// then the file should be processed, in which case the returned branch name is
// "".
func (p *Parser) checkBranchName(params map[string]string) (string, bool) {
	if len(p.branchNames) == 0 {
		return "", true
	}
	branch, ok := params["branch"]
	if ok {
		return branch, p.branchNames[branch]
	}
	return "", true
}

func (p *Parser) extractFromLegacyFile(r io.Reader) ([]paramtools.Params, []float32, string, map[string]string, error) {
	benchData, err := format.ParseLegacyFormat(r)
	if err != nil {
		return nil, nil, "", nil, err
	}
	params, values := getParamsAndValuesFromLegacyFormat(benchData)
	return params, values, benchData.Hash, benchData.Key, nil
}

func (p *Parser) extractFromVersion1File(r io.Reader, filename string) ([]paramtools.Params, []float32, string, map[string]string, map[string]string, bool, error) {
	f, err := format.Parse(r)
	if err != nil {
		sklog.Warningf("Failed to parse the version one file: %s, got error: %s", filename, err)
		return nil, nil, "", nil, nil, true, err
	}
	params, values := getParamsAndValuesFromVersion1Format(f, p.invalidParamCharRegex)
	return params, values, f.GitHash, f.Key, f.Links, true, nil
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
func (p *Parser) Parse(ctx context.Context, file file.File) ([]paramtools.Params, []float32, string, map[string]string, error) {
	_, span := trace.StartSpan(ctx, "ingest.parser.Parse")
	defer span.End()

	defer util.Close(file.Contents)
	p.parseCounter.Inc(1)

	// Read the whole content into bytes.Reader since we may take more than one
	// pass at the data.
	sklog.Infof("About to read.")
	b, err := io.ReadAll(file.Contents)
	sklog.Infof("Finished readall.")
	if err != nil {
		p.parseFailCounter.Inc(1)
		return nil, nil, "", nil, skerr.Wrap(err)
	}
	r := bytes.NewReader(b)

	// Expect the file to be in format.FileFormat.
	sklog.Info("About to extract")
	params, values, hash, commonKeys, links, proceed, err := p.extractFromVersion1File(r, file.Name)
	if err != nil {
		// Fallback to the legacy format.
		if _, err := r.Seek(0, io.SeekStart); err != nil {
			return nil, nil, "", nil, skerr.Wrap(err)
		}
		sklog.Info("About to extract from legacy.")
		params, values, hash, commonKeys, err = p.extractFromLegacyFile(r)
	}
	if err != nil && err != ErrFileShouldBeSkipped {
		p.parseFailCounter.Inc(1)
	}
	if err != nil {
		return nil, nil, "", nil, err
	}
	if !proceed {
		return nil, nil, "", nil, nil
	}

	branch, ok := p.checkBranchName(commonKeys)
	if !ok {
		return nil, nil, "", nil, ErrFileShouldBeSkipped
	}
	if len(params) == 0 {
		metrics2.GetCounter("perf_ingest_parser_no_data_in_file", map[string]string{"branch": branch}).Inc(1)
		sklog.Infof("No data in: %q", file.Name)
		return nil, nil, "", nil, ErrFileShouldBeSkipped
	}
	return params, values, hash, links, nil
}

// ParseCommitNumberFromGitHash parse commit number from git hash.
// this method will be used to get integer commit number from string git hash.
// For example: "git_hash": "CP:727901", the commit number will be 727901
func (p *Parser) ParseCommitNumberFromGitHash(gitHash string) (types.CommitNumber, error) {
	gitHashContent := strings.SplitN(gitHash, "CP:", -1)

	if len(gitHashContent) != 2 {
		return types.BadCommitNumber, skerr.Fmt("Failed to parse commit number string from git hash: %q", gitHash)
	}

	commitNumber, err := strconv.Atoi(gitHashContent[1])
	if err != nil {
		return types.BadCommitNumber, skerr.Wrapf(err, "Failed to parse commit number integer from git hash: %q", gitHash)
	}

	return types.CommitNumber(commitNumber), nil
}
