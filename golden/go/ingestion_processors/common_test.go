package ingestion_processors

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/types"
)

const (
	// name of the input file containing test data.
	TEST_INGESTION_FILE = "testdata/dm.json"
)

// Tests parsing and processing of a single file.
// There don't need to be more of these here because we should
// depend on jsonio.ParseGoldResults which has its own test suite.
func TestDMResults(t *testing.T) {
	unittest.SmallTest(t)
	f, err := os.Open(TEST_INGESTION_FILE)
	assert.NoError(t, err)

	gr, err := parseGoldResultsFromReader(f)
	assert.NoError(t, err)

	assert.Equal(t, &jsonio.GoldResults{
		GitHash: "02cb37309c01506e2552e931efa9c04a569ed266",
		Key: map[string]string{
			"arch":             "x86_64",
			"compiler":         "MSVC",
			"configuration":    "Debug",
			"cpu_or_gpu":       "CPU",
			"cpu_or_gpu_value": "AVX2",
			"model":            "ShuttleB",
			"os":               "Win8",
		},
		Results: []*jsonio.Result{
			{
				Key: map[string]string{
					"config":                "pipe-8888",
					types.PRIMARY_KEY_FIELD: "aaclip",
					types.CORPUS_FIELD:      "gm",
				},
				Digest: "fa3c371d201d6f88f7a47b41862e2e85",
				Options: map[string]string{
					"ext": "png",
				},
			},
			{
				Key: map[string]string{
					"config":                "pipe-8888",
					types.PRIMARY_KEY_FIELD: "clipcubic",
					types.CORPUS_FIELD:      "gm",
				},
				Digest: "64e446d96bebba035887dd7dda6db6c4",
				Options: map[string]string{
					"ext": "png",
				},
			},
			{
				Key: map[string]string{
					"config":                "pipe-8888",
					types.PRIMARY_KEY_FIELD: "manyarcs",
					types.CORPUS_FIELD:      "gm",
				},
				Digest: "4d289d13da841e4a2f153bcb61024f42",
				Options: map[string]string{
					"ext": "pdf",
				},
			},
		},
	}, gr)
}

const (
	alphaCommitHash = "aaa96d8aff4cd689c2e49336d12928a8bd23cdec"
	betaCommitHash  = "bbbcf37f5bd91f1a7b3f080bf038af8e8fa4cab2"
)

var (
	commitNotFound = errors.New("commit not found")
)
