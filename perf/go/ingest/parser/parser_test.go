// Package parser has funcs for parsing ingestion files.
package parser

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/file"
	"go.skia.org/infra/perf/go/ingest/format"
)

const goodBranchName = "some-branch-name"

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
	unittest.SmallTest(t)
	// Load the sample data file as BenchData.

	r := testutils.MustGetReader(filepath.Join("legacy", "success.json"))

	benchData, err := format.ParseLegacyFormat(r)
	require.NoError(t, err)

	params, values := getParamsAndValuesFromLegacyFormat(benchData)
	assert.Len(t, values, 4)
	assert.Len(t, params, 4)
	assert.Contains(t, values, float32(858))
	assert.Contains(t, params, expectedGoodParams)
}

func TestGetParamsAndValuesFromFormat_Success(t *testing.T) {
	unittest.SmallTest(t)
	// Load the sample data file as BenchData.
	r := testutils.MustGetReader(filepath.Join("version_1", "success.json"))

	f, err := format.Parse(r)
	require.NoError(t, err)

	params, values := getParamsAndValuesFromVersion1Format(f)
	assert.Len(t, values, 4)
	assert.Len(t, params, 4)
	assert.Contains(t, values, float32(858))
	assert.Contains(t, params, expectedGoodParams)
}

func TestParser(t *testing.T) {
	unittest.SmallTest(t)
	// Loop over all the ingestion formats we support. Parallel test files with
	// the same names are held in subdirectories of 'testdata'.
	for _, ingestionFormat := range []string{"legacy", "version_1"} {
		for name, subTest := range SubTests {
			subTestName := fmt.Sprintf("%s_%s", name, ingestionFormat)
			t.Run(subTestName, func(t *testing.T) {
				p, f := parserForTest(t, ingestionFormat, subTest.filename)
				subTest.SubTestFunction(t, p, f)
			})
		}
	}
}

func parserForTest(t *testing.T, subdir, filename string) (*Parser, file.File) {
	unittest.SmallTest(t)
	instanceConfig := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			Branches: []string{goodBranchName},
		},
	}
	ret := New(instanceConfig)
	ret.parseCounter.Reset()
	ret.parseFailCounter.Reset()

	// Load the sample data file as BenchData.

	r := testutils.MustGetReader(filepath.Join(subdir, filename))

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
	params, values, gitHash, err := p.Parse(f)
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
	assert.Equal(t, "327697", cl)
	assert.Equal(t, "1", patch)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(0), p.parseFailCounter.Get())
}

func parse_NoBranchSpecified_Success(t *testing.T, p *Parser, f file.File) {
	p.branchNames = nil
	params, values, gitHash, err := p.Parse(f)
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
	_, _, _, err := p.Parse(f)
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
	_, _, _, err := p.Parse(f)
	assert.Equal(t, ErrFileShouldBeSkipped, err)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(0), p.parseFailCounter.Get())
}

func parse_SkipIfListedInBranchesButHasNoData(t *testing.T, p *Parser, f file.File) {
	_, _, _, err := p.Parse(f)
	assert.Equal(t, ErrFileShouldBeSkipped, err)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(0), p.parseFailCounter.Get())
}

func parse_OnlyOneMeasurementInfile(t *testing.T, p *Parser, f file.File) {
	params, values, gitHash, err := p.Parse(f)
	require.NoError(t, err)
	assert.Len(t, params, 1)
	assert.Equal(t, "fe4a4029a080bc955e9588d05a6cd9eb490845d4", gitHash)
	assert.Equal(t, values, []float32{12.3})
}

func parse_OnlyOneMeasurementWithValueZeroInfile(t *testing.T, p *Parser, f file.File) {
	params, values, gitHash, err := p.Parse(f)
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
	f.Contents = ioutil.NopCloser(alwaysErrReader{})
	_, _, err := p.ParseTryBot(f)
	require.Error(t, err)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(1), p.parseFailCounter.Get())
}

func parse_ReadErr(t *testing.T, p *Parser, f file.File) {
	f.Contents = ioutil.NopCloser(alwaysErrReader{})
	_, _, _, err := p.Parse(f)
	require.Error(t, err)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(1), p.parseFailCounter.Get())
}
