package tracestore

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

func TestTraceIDFromParams(t *testing.T) {
	unittest.SmallTest(t)

	input := paramtools.Params{
		"cpu":                   "x86",
		"gpu":                   "nVidia",
		types.PRIMARY_KEY_FIELD: "test_alpha",
		types.CORPUS_FIELD:      "dm",
	}

	expected := tiling.TraceId(",cpu=x86,gpu=nVidia,name=test_alpha,source_type=dm,")

	assert.Equal(t, expected, TraceIDFromParams(input))
}
