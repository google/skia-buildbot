// Package parser has funcs for parsing ingestion files.
package parser

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/config/validate"
	"go.skia.org/infra/perf/go/file"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/types"
)

const goodBranchName = "some-branch-name"
const legacyVersionName = "legacy"
const versionOneName = "version_1"

var (
	expectedGoodParams = paramtools.Params{
		"arch":       "x86",
		"branch":     "some-branch-name",
		"config":     "meta",
		"gpu":        "GTX660",
		"model":      "ShuttleA",
		"os":         "Ubuntu12",
		"sub_result": "max_rss_mb",
		"system":     "UNIX",
		"test":       "memory_usage_0_0",
	}
)

func TestGetParamsAndValuesFromLegacyFormat_Success(t *testing.T) {
	// Load the sample data file as BenchData.

	r := testutils.GetReader(t, filepath.Join(legacyVersionName, "success.json"))

	benchData, err := format.ParseLegacyFormat(r)
	require.NoError(t, err)

	params, values := getParamsAndValuesFromLegacyFormat(benchData)
	assert.Len(t, values, 4)
	assert.Len(t, params, 4)
	assert.Contains(t, values, float32(858))
	assert.Contains(t, params, expectedGoodParams)
}

func TestGetParamsAndValuesFromFormat_Success(t *testing.T) {
	// Load the sample data file as BenchData.
	r := testutils.GetReader(t, filepath.Join(versionOneName, "success.json"))

	f, err := format.Parse(r)
	require.NoError(t, err)

	params, values := getParamsAndValuesFromVersion1Format(f, query.InvalidChar)
	assert.Len(t, values, 4)
	assert.Len(t, params, 4)
	assert.Contains(t, values, float32(858))
	assert.Contains(t, params, expectedGoodParams)
}

// TestGetParamsAndValuesFromVersion1Format_Sanitization checks that
// sanitization logic works correctly for both default and custom regexes,
// without checking for log output.
func TestGetParamsAndValuesFromVersion1Format_Sanitization(t *testing.T) {

	customRegex := regexp.MustCompile(`([^a-zA-Z0-9!~@#$%^&*()+ :\\._\\-])`)
	defaultRegex := query.InvalidChar

	testCases := []struct {
		name           string
		inputFormat    format.Format
		regex          *regexp.Regexp
		expectedParams paramtools.Params
	}{
		{
			name: "Default Regex, Clean Data",
			inputFormat: format.Format{
				// Data must be in the Results slice, which the function iterates over.
				Results: []format.Result{
					{
						Key: map[string]string{"clean_key": "clean-value.1"},
						// Measurements is empty, so it hits the simple sanitization path.
					},
				},
			},
			regex: defaultRegex,
			expectedParams: paramtools.Params{
				"clean_key": "clean-value.1",
			},
		},
		{
			name: "Default Regex, Dirty Data (forbids space and @)",
			inputFormat: format.Format{
				Results: []format.Result{
					{
						Key: map[string]string{"dirty key": "value@default"},
					},
				},
			},
			regex: defaultRegex,
			expectedParams: paramtools.Params{
				"dirty_key": "value_default", // ' ' and '@' are replaced
			},
		},
		{
			name: "Custom Regex, Clean Data (allows space and @)",
			inputFormat: format.Format{
				Results: []format.Result{
					{
						Key: map[string]string{"clean key": "value@custom"},
					},
				},
			},
			regex: customRegex,
			expectedParams: paramtools.Params{
				"clean key": "value@custom", // ' ' and '@' are NOT replaced
			},
		},
		{
			name: "Custom Regex, Dirty Data (still forbids ; and ?)",
			inputFormat: format.Format{
				Results: []format.Result{
					{
						Key: map[string]string{"dirty;key": "value?custom"},
					},
				},
			},
			regex: customRegex,

			expectedParams: paramtools.Params{
				"dirty_key": "value_custom", // ';' and '?' ARE replaced
			},
		},
		{
			name: "Default Regex, Multiple Keys",
			inputFormat: format.Format{
				Results: []format.Result{
					{
						Key: map[string]string{
							"key1": "value1",
							"key2": "value2",
							"key3": "value3",
						},
					},
				},
			},
			regex: defaultRegex,
			expectedParams: paramtools.Params{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		},
		{
			name: "Default Regex, Multiple Keys, Dirty Data",
			inputFormat: format.Format{
				Results: []format.Result{
					{
						Key: map[string]string{
							"key 1": "value 1",
							"key@2": "value@2",
							"key,3": "value,3",
						},
					},
				},
			},
			regex: defaultRegex,
			expectedParams: paramtools.Params{
				"key_1": "value_1",
				"key_2": "value_2",
				"key_3": "value_3",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Run the function under test
			params, _ := getParamsAndValuesFromVersion1Format(tc.inputFormat, tc.regex)

			// This assertion should now pass.
			require.Len(t, params, 1)
			assert.Equal(t, tc.expectedParams, params[0])
		})
	}
}

func TestParser(t *testing.T) {
	// Loop over all the ingestion formats we support. Parallel test files with
	// the same names are held in subdirectories of 'testdata'.
	for _, ingestionFormat := range []string{legacyVersionName, versionOneName} {
		for name, subTest := range SubTests {
			subTestName := fmt.Sprintf("%s_%s", name, ingestionFormat)
			t.Run(subTestName, func(t *testing.T) {
				p, f := parserForTest(t, ingestionFormat, subTest.filename)
				subTest.SubTestFunction(t, p, f)
			})
		}
	}

	for name, subTest := range V1OnlySubTests {
		t.Run(name, func(t *testing.T) {
			p, f := parserForTest(t, versionOneName, subTest.filename)
			subTest.SubTestFunction(t, p, f)
		})
	}
}

func parserForTest(t *testing.T, subdir, filename string) (*Parser, file.File) {
	instanceConfig := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			Branches: []string{goodBranchName},
		},
	}
	ret, err := New(context.Background(), instanceConfig)
	require.NoError(t, err)
	ret.parseCounter.Reset()
	ret.parseFailCounter.Reset()

	// Load the sample data file as BenchData.

	r := testutils.GetReader(t, filepath.Join(subdir, filename))

	return ret, file.File{
		Name:     filename,
		Contents: r,
	}
}

// SubTestFunction is a func we will call to test one aspect of Parser.Parse.
type SubTestFunction func(t *testing.T, p *Parser, f file.File)

// SubTests are all the subtests we have for Parser.Parse
var SubTests = map[string]struct {
	SubTestFunction SubTestFunction
	filename        string
}{
	"parse_Success":                               {parse_Success, "success.json"},
	"parse_NoBranchSpecified_Success":             {parse_NoBranchSpecified_Success, "success.json"},
	"parse_MalformedJSONError":                    {parse_MalformedJSONError, "invalid.json"},
	"parse_SkipIfNotListedInBranches":             {parse_SkipIfNotListedInBranches, "unknown_branch.json"},
	"parse_SkipIfListedInBranchesButHasNoData":    {parse_SkipIfListedInBranchesButHasNoData, "no_results.json"},
	"parse_OnlyOneMeasurementInfile":              {parse_OnlyOneMeasurementInfile, "one_measurement.json"},
	"parse_OnlyOneMeasurementWithValueZeroInfile": {parse_OnlyOneMeasurementWithValueZeroInfile, "zero_measurement.json"},
	"parse_ReadErr":                               {parse_ReadErr, "success.json"},
	"parseTryBot_Success":                         {parseTryBot_Success, "success.json"},
	"parseTryBot_MalformedJSONError":              {parseTryBot_MalformedJSONError, "invalid.json"},
	"parseTryBot_ReadErr":                         {parseTryBot_ReadErr, "success.json"},
}

func parse_Success(t *testing.T, p *Parser, f file.File) {
	params, values, gitHash, _, err := p.Parse(context.Background(), f)
	require.NoError(t, err)
	assert.Equal(t, "fe4a4029a080bc955e9588d05a6cd9eb490845d4", gitHash)
	assert.Len(t, values, 4)
	assert.Len(t, params, 4)
	assert.Contains(t, values, float32(858))
	assert.Contains(t, params, expectedGoodParams)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(0), p.parseFailCounter.Get())
}

func parseTryBot_Success(t *testing.T, p *Parser, f file.File) {
	cl, patch, err := p.ParseTryBot(f)
	require.NoError(t, err)
	assert.Equal(t, types.CL("327697"), cl)
	assert.Equal(t, "1", patch)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(0), p.parseFailCounter.Get())
}

func parse_NoBranchSpecified_Success(t *testing.T, p *Parser, f file.File) {
	p.branchNames = nil
	params, values, gitHash, _, err := p.Parse(context.Background(), f)
	require.NoError(t, err)
	assert.Equal(t, "fe4a4029a080bc955e9588d05a6cd9eb490845d4", gitHash)
	assert.Len(t, values, 4)
	assert.Len(t, params, 4)
	assert.Contains(t, values, float32(858))
	assert.Contains(t, params, expectedGoodParams)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(0), p.parseFailCounter.Get())
}

func parse_MalformedJSONError(t *testing.T, p *Parser, f file.File) {
	_, _, _, _, err := p.Parse(context.Background(), f)
	require.Error(t, err)
	assert.NotEqual(t, ErrFileShouldBeSkipped, err)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(1), p.parseFailCounter.Get())
}

func parseTryBot_MalformedJSONError(t *testing.T, p *Parser, f file.File) {
	_, _, err := p.ParseTryBot(f)
	require.Error(t, err)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(1), p.parseFailCounter.Get())
}

func parse_SkipIfNotListedInBranches(t *testing.T, p *Parser, f file.File) {
	_, _, _, _, err := p.Parse(context.Background(), f)
	assert.Equal(t, ErrFileShouldBeSkipped, err)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(0), p.parseFailCounter.Get())
}

func parse_SkipIfListedInBranchesButHasNoData(t *testing.T, p *Parser, f file.File) {
	_, _, _, _, err := p.Parse(context.Background(), f)
	assert.Equal(t, ErrFileShouldBeSkipped, err)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(0), p.parseFailCounter.Get())
}

func parse_OnlyOneMeasurementInfile(t *testing.T, p *Parser, f file.File) {
	params, values, gitHash, _, err := p.Parse(context.Background(), f)
	require.NoError(t, err)
	assert.Len(t, params, 1)
	assert.Equal(t, "fe4a4029a080bc955e9588d05a6cd9eb490845d4", gitHash)
	assert.Equal(t, values, []float32{12.3})
}

func parse_OnlyOneMeasurementWithValueZeroInfile(t *testing.T, p *Parser, f file.File) {
	params, values, gitHash, _, err := p.Parse(context.Background(), f)
	require.NoError(t, err)
	assert.Len(t, params, 1)
	assert.Equal(t, "fe4a4029a080bc955e9588d05a6cd9eb490845d4", gitHash)
	assert.Equal(t, values, []float32{0.0})
}

type alwaysErrReader struct{}

func (a alwaysErrReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("Error reading.")
}

func parseTryBot_ReadErr(t *testing.T, p *Parser, f file.File) {
	f.Contents = io.NopCloser(alwaysErrReader{})
	_, _, err := p.ParseTryBot(f)
	require.Error(t, err)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(1), p.parseFailCounter.Get())
}

func parse_ReadErr(t *testing.T, p *Parser, f file.File) {
	f.Contents = io.NopCloser(alwaysErrReader{})
	_, _, _, _, err := p.Parse(context.Background(), f)
	require.Error(t, err)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(1), p.parseFailCounter.Get())
}

// V1OnlySubTests are all the subtests we have for version_1 input of Parser.Parse
var V1OnlySubTests = map[string]struct {
	SubTestFunction SubTestFunction
	filename        string
}{
	"parse_WithCommitNumberSpecified_Success": {parse_WithCommitNumberSpecified_Success, "with_commit_number.json"},
	"parse_InvalidCommitNumber_Error":         {parse_InvalidCommitNumber_Error, "invalid_commit_number.json"},
}

func parse_WithCommitNumberSpecified_Success(t *testing.T, p *Parser, f file.File) {
	params, values, gitHash, links, err := p.Parse(context.Background(), f)
	require.NoError(t, err)
	commitNumber, err := p.ParseCommitNumberFromGitHash(gitHash)
	require.NoError(t, err)
	assert.Equal(t, types.CommitNumber(727901), commitNumber)
	assert.Len(t, values, 4)
	assert.Len(t, params, 4)
	assert.Contains(t, values, float32(858))
	assert.Contains(t, params, expectedGoodParams)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(0), p.parseFailCounter.Get())
	assert.Equal(t, map[string]string{
		"link1": "www.google.com",
	}, links)
}

func parse_InvalidCommitNumber_Error(t *testing.T, p *Parser, f file.File) {
	_, _, gitHash, _, err := p.Parse(context.Background(), f)
	require.NoError(t, err)
	commitNumber, err := p.ParseCommitNumberFromGitHash(gitHash)
	require.Error(t, err)
	assert.Equal(t, types.BadCommitNumber, commitNumber)
}

func TestGetSamplesFromLegacyFormat_GoodData_Success(t *testing.T) {
	// Load the sample data file as BenchData.

	r := testutils.GetReader(t, filepath.Join(legacyVersionName, "samples_success.json"))

	b, err := format.ParseLegacyFormat(r)
	require.NoError(t, err)

	res := GetSamplesFromLegacyFormat(b)
	require.Equal(t, 2, len(res))
	expected := SamplesSet{
		",bench_type=micro,config=8888,name=writepix_pm_bgra_srgb,source_type=bench,sub_result=min_ms,test=writepix_pm_bgra_srgb_640_480,": {
			Params: paramtools.Params{
				"bench_type":  "micro",
				"config":      "8888",
				"name":        "writepix_pm_bgra_srgb",
				"source_type": "bench",
				"sub_result":  "min_ms",
				"test":        "writepix_pm_bgra_srgb_640_480",
			},
			Values: []float64{
				0.1828799247741699, 0.1826989650726318, 0.1827061176300049, 0.1827809810638428, 0.1825799942016602, 0.1826908588409424, 0.1829240322113037, 0.182894229888916, 0.1915080547332764, 0.1832039356231689, 0.1829140186309814, 0.1829030513763428, 0.1829860210418701, 0.182703971862793, 0.1829319000244141, 0.1827821731567383, 0.1828160285949707, 0.1827318668365479, 0.1829230785369873, 0.1828629970550537},
		},

		",bench_type=micro,config=8888,name=writepix_um_bgra_srgb,source_type=bench,sub_result=min_ms,test=writepix_um_bgra_srgb_640_480,": {
			Params: paramtools.Params{
				"bench_type":  "micro",
				"config":      "8888",
				"name":        "writepix_um_bgra_srgb",
				"source_type": "bench",
				"sub_result":  "min_ms",
				"test":        "writepix_um_bgra_srgb_640_480",
			},
			Values: []float64{
				0.2057559490203857, 0.1993551254272461, 0.198721170425415, 0.1989059448242188, 0.1990170478820801, 0.1992101669311523, 0.1991400718688965, 0.1987521648406982, 0.1994550228118896, 0.199286937713623, 0.1990699768066406, 0.1992120742797852, 0.198800802230835, 0.1989920139312744, 0.199099063873291, 0.2042169570922852, 0.1992931365966797, 0.1988308429718018, 0.1994678974151611, 0.1989710330963135,
			},
		}}
	assert.Equal(t, expected, res)
}

func TestGetSamplesFromLegacyFormat_EmptyData_Success(t *testing.T) {

	// Load the sample data file as BenchData.
	r := testutils.GetReader(t, filepath.Join(legacyVersionName, "samples_no_results.json"))

	b, err := format.ParseLegacyFormat(r)
	require.NoError(t, err)

	res := GetSamplesFromLegacyFormat(b)
	assert.Empty(t, res)
}

func TestSamplesSetAdd_EmptySamples_Success(t *testing.T) {
	a := SamplesSet{}
	b := SamplesSet{}
	a.Add(b)
	require.Empty(t, a)
}

func TestSamplesSetAdd_NonEmptySamples_Success(t *testing.T) {
	a := SamplesSet{
		",config=8888,": Samples{
			Params: paramtools.Params{"config": "8888"},
			Values: []float64{1.0, 2.0},
		},
		",config=565,": {
			Params: paramtools.Params{"config": "565"},
			Values: []float64{},
		},
	}
	b := SamplesSet{
		",config=8888,": {
			Params: paramtools.Params{"config": "8888"},
			Values: []float64{2.0, 3.0},
		},
		",config=gles,": {
			Params: paramtools.Params{"config": "gles"},
			Values: []float64{4.0},
		},
	}
	a.Add(b)
	expected := SamplesSet{
		",config=8888,": {
			Params: paramtools.Params{"config": "8888"},
			Values: []float64{1.0, 2.0, 2.0, 3.0},
		},
		",config=565,": {
			Params: paramtools.Params{"config": "565"},
			Values: []float64{},
		},
		",config=gles,": {
			Params: paramtools.Params{"config": "gles"},
			Values: []float64{4.0},
		},
	}
	require.Equal(t, expected, a)
}

func TestParseWithInvalidCharRegex_Success(t *testing.T) {
	instanceConfig := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			Branches: []string{goodBranchName},
		},
		InvalidParamCharRegex: "([^a-zA-Z0-9!~@#$%^&*()+ :\\._\\-])",
	}
	p, err := New(context.Background(), instanceConfig)
	require.NoError(t, err)
	p.parseCounter.Reset()
	p.parseFailCounter.Reset()

	fileName := "with_special_chars.json"
	f := file.File{
		Name:     fileName,
		Contents: testutils.GetReader(t, filepath.Join(versionOneName, fileName)),
	}

	params, values, gitHash, _, err := p.Parse(context.Background(), f)
	require.NoError(t, err)
	assert.Equal(t, "fe4a4029a080bc955e9588d05a6cd9eb490845d4", gitHash)
	assert.Len(t, values, 4)
	assert.Len(t, params, 4)
	assert.Contains(t, values, float32(858))
	expectedParams1 := paramtools.Params{
		"arch":        "x!~@#$%^&*():86",
		"branch":      "some-branch-name",
		"config":      "meta",
		"gpu":         "GTX660",
		"model":       "ShuttleA",
		"os":          "Ubuntu 12",
		"system":      "UNIX",
		"test":        "DeferredSurfaceCopy_x!~@#$%^&*():discardable_640_480",
		"source_type": "bench",
		"stat":        "value",
		"sub_result":  "min+ms",
	}
	expectedParams2 := paramtools.Params{
		"arch":       "x!~@#$%^&*():86",
		"branch":     "some-branch-name",
		"config":     "meta",
		"gpu":        "GTX660",
		"model":      "ShuttleA",
		"os":         "Ubuntu 12",
		"system":     "UNIX",
		"test":       "memory+usage_0_0",
		"sub_result": "max+rss+mb",
	}
	assert.Contains(t, params, expectedParams1)
	assert.Contains(t, params, expectedParams2)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(0), p.parseFailCounter.Get())
}

func TestParseWithConfigFile_InvalidCharRegex_NoEqual_NoComma(t *testing.T) {
	allExistingConfigs, err := filepath.Glob("../../../configs/*.json")
	require.Greater(t, len(allExistingConfigs), 0)
	require.NoError(t, err)
	for _, filename := range allExistingConfigs {
		instanceConfig, schemaErrors, err := validate.InstanceConfigFromFile(filename)
		require.Len(t, schemaErrors, 0)
		require.NoError(t, err, filename)

		if len(instanceConfig.InvalidParamCharRegex) > 0 {
			p, err := New(context.Background(), instanceConfig)
			require.NoError(t, err)
			p.parseCounter.Reset()
			p.parseFailCounter.Reset()

			expectedParams := paramtools.Params{
				"config": "meta_x",
				"test":   "memory+usage_0_0",
			}

			fileName := "with_equal_in_param.json"
			f := file.File{
				Name:     fileName,
				Contents: testutils.GetReader(t, filepath.Join(versionOneName, fileName)),
			}
			params, _, _, _, err := p.Parse(context.Background(), f)
			require.NoError(t, err)
			assert.Contains(t, params, expectedParams)
			assert.Equal(t, int64(1), p.parseCounter.Get())
			assert.Equal(t, int64(0), p.parseFailCounter.Get())

			fileName = "with_comma_in_param.json"
			f = file.File{
				Name:     fileName,
				Contents: testutils.GetReader(t, filepath.Join(versionOneName, fileName)),
			}
			params, _, _, _, err = p.Parse(context.Background(), f)
			require.NoError(t, err)
			assert.Contains(t, params, expectedParams)
			assert.Equal(t, int64(2), p.parseCounter.Get())
			assert.Equal(t, int64(0), p.parseFailCounter.Get())
		}
	}
}
