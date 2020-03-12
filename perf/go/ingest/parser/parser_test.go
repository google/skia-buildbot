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

func parserForTest(t *testing.T) *Parser {
	instanceConfig := &config.InstanceConfig{
		IngestionConfig: config.IngestionConfig{
			Branches: []string{goodBranchName},
		},
	}
	return New(instanceConfig)
}
func TestGetParamsAndValues_Success(t *testing.T) {
	unittest.SmallTest(t)
	// Load the sample data file as BenchData.
	r, err := os.Open(filepath.Join("testdata", "nano.json"))
	require.NoError(t, err)

	benchData, err := format.ParseBenchDataFromReader(r)
	require.NoError(t, err)

	params, values := getParamsAndValues(benchData)
	assert.Len(t, values, 13)
	assert.Len(t, params, 13)
	assert.Contains(t, values, float64(858))
	assert.Contains(t, params, paramtools.Params{
		"arch":       "x86",
		"config":     "meta",
		"gpu":        "GTX660",
		"model":      "ShuttleA",
		"os":         "Ubuntu12",
		"sub_result": "max_rss_mb",
		"system":     "UNIX",
		"test":       "memory_usage_0_0",
	})
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

	params, values, hash, err := p.Parse(f)
	require.NoError(t, err)
	assert.Equal(t, "fe4a4029a080bc955e9588d05a6cd9eb490845d4", hash)

	assert.Len(t, values, 13)
	assert.Len(t, params, 13)
	assert.Contains(t, values, float64(858))
	assert.Contains(t, params, paramtools.Params{
		"arch":       "x86",
		"config":     "meta",
		"gpu":        "GTX660",
		"model":      "ShuttleA",
		"os":         "Ubuntu12",
		"sub_result": "max_rss_mb",
		"system":     "UNIX",
		"test":       "memory_usage_0_0",
	})
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
}

func TestParse_SkipIfNotListedInBranches(t *testing.T) {
	unittest.SmallTest(t)

	p := parserForTest(t)
	contents := `{
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
}
