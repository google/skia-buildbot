// Package parser has funcs for parsing ingestion files.
package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/ingest/format"
)

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
