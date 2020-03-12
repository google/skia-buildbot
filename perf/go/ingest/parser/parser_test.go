// Package parser has funcs for parsing ingestion files.
package parser

import (
	"bytes"
	"fmt"
	"io/ioutil"
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

func parserForTest(t *testing.T) *Parser {
	instanceConfig := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			Branches: []string{goodBranchName},
		},
	}
	ret := New(instanceConfig)
	ret.parseCounter.Reset()
	ret.parseFailCounter.Reset()
	return ret
}

func TestGetParamsAndValues_Success(t *testing.T) {
	unittest.SmallTest(t)
	// Load the sample data file as BenchData.
	r, err := os.Open(filepath.Join("testdata", "nano.json"))
	require.NoError(t, err)

	benchData, err := format.ParseBenchDataFromReader(r)
	require.NoError(t, err)

	params, values := getParamsAndValues(benchData)
	assert.Len(t, values, 5)
	assert.Len(t, params, 5)
	assert.Contains(t, values, float64(858))
	assert.Contains(t, params, expectedGoodParams)
}

func TestParse_Success(t *testing.T) {
	unittest.SmallTest(t)

	p := parserForTest(t)
	// Load the sample data file as BenchData.
	r, err := os.Open(filepath.Join("testdata", "nano.json"))
	require.NoError(t, err)

	f := file.File{
		Name:     "nano.json",
		Contents: r,
	}

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

func TestParse_MalformedJSONError(t *testing.T) {
	unittest.SmallTest(t)

	p := parserForTest(t)
	f := file.File{
		Name:     "nano.json",
		Contents: ioutil.NopCloser(bytes.NewReader([]byte("not valid json"))),
	}

	_, _, _, err := p.Parse(f)
	require.Error(t, err)
	assert.NotEqual(t, ErrFileShouldBeSkipped, err)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(1), p.parseFailCounter.Get())
}

func TestParse_SkipIfNotListedInBranches(t *testing.T) {
	unittest.SmallTest(t)

	p := parserForTest(t)
	const contents = `{
		"gitHash" : "fe4a4029a080bc955e9588d05a6cd9eb490845d4",
		"results" : {},
		"key" : {
		   "branch" : "ignoreme"
		}
	 }`

	f := file.File{
		Name:     "nano.json",
		Contents: ioutil.NopCloser(bytes.NewReader([]byte(contents))),
	}

	_, _, _, err := p.Parse(f)
	assert.Equal(t, ErrFileShouldBeSkipped, err)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(0), p.parseFailCounter.Get())
}

func TestParse_SkipIfListedInBranchesButHasNoData(t *testing.T) {
	unittest.SmallTest(t)

	p := parserForTest(t)
	contents := fmt.Sprintf(`{
		"gitHash" : "fe4a4029a080bc955e9588d05a6cd9eb490845d4",
		"results" : {},
		"key" : {
		   "branch" : "%s"
		}
	 }`, goodBranchName)

	f := file.File{
		Name:     "nano.json",
		Contents: ioutil.NopCloser(bytes.NewReader([]byte(contents))),
	}

	_, _, _, err := p.Parse(f)
	assert.Equal(t, ErrFileShouldBeSkipped, err)
	assert.Equal(t, int64(1), p.parseCounter.Get())
	assert.Equal(t, int64(0), p.parseFailCounter.Get())
}
