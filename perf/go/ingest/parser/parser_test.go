// Package parser has funcs for parsing ingestion files.
package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
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
	r, err := os.Open(filepath.Join("testdata", "legacy", "success.json"))
	require.NoError(t, err)

	benchData, err := format.ParseLegacyFormat(r)
	require.NoError(t, err)

	params, values := getParamsAndValuesFromLegacyFormat(benchData)
	assert.Len(t, values, 5)
	assert.Len(t, params, 5)
	assert.Contains(t, values, float64(858))
	assert.Contains(t, params, expectedGoodParams)
}

func TestGetParamsAndValuesFromFormat_Success(t *testing.T) {
	unittest.SmallTest(t)
	// Load the sample data file as BenchData.
	r, err := os.Open(filepath.Join("testdata", "version_1", "success.json"))
	require.NoError(t, err)

	f, err := format.Parse(r)
	require.NoError(t, err)

	params, values := getParamsAndValuesFromVersion1Format(f)
	assert.Len(t, values, 5)
	assert.Len(t, params, 5)
	assert.Contains(t, values, float64(858))
	assert.Contains(t, params, expectedGoodParams)
}

func TestParser(t *testing.T) {
	// Loop over all the ingestion formats we support. Parallel test files
	// with the same names are help in subdirectories of 'testdata'.
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
	r, err := os.Open(filepath.Join("testdata", subdir, filename))
	require.NoError(t, err)

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
	"parse_Success":                            {parse_Success, "success.json"},
	"parse_MalformedJSONError":                 {parse_MalformedJSONError, "invalid.json"},
	"parse_SkipIfNotListedInBranches":          {parse_SkipIfNotListedInBranches, "unknown_branch.json"},
	"parse_SkipIfListedInBranchesButHasNoData": {parse_SkipIfListedInBranchesButHasNoData, "no_results.json"},
}

func parse_Success(t *testing.T, p *Parser, f file.File) {
	params, values, gitHash, err := p.Parse(f)
	require.NoError(t, err)
	assert.Equal(t, "fe4a4029a080bc955e9588d05a6cd9eb490845d4", gitHash)
	assert.Len(t, values, 5)
	assert.Len(t, params, 5)
	assert.Contains(t, values, float64(858))
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
