package tiling

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

func TestTraceIDFromParams_ValidKeysAndValues_Success(t *testing.T) {
	unittest.SmallTest(t)

	input := paramtools.Params{
		"cpu":                 "x86",
		"gpu":                 "nVidia",
		types.PrimaryKeyField: "test_alpha",
		types.CorpusField:     "dm",
	}

	expected := TraceID(",cpu=x86,gpu=nVidia,name=test_alpha,source_type=dm,")

	require.Equal(t, expected, TraceIDFromParams(input))
}

// TestTraceIDFromParamsMalicious adds some values with invalid chars.
func TestTraceIDFromParams_MaliciousKeysAndValues_Success(t *testing.T) {
	unittest.SmallTest(t)

	input := paramtools.Params{
		"c=p,u":               `"x86"`,
		"gpu":                 "nVi,,=dia",
		types.PrimaryKeyField: "test=alpha",
		types.CorpusField:     "dm!",
	}

	expected := TraceID(`,c_p_u="x86",gpu=nVi___dia,name=test_alpha,source_type=dm!,`)

	require.Equal(t, expected, TraceIDFromParams(input))
}
