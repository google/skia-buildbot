package tiling

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

func TestNewEmptyTrace_Success(t *testing.T) {
	unittest.SmallTest(t)
	const N = 5
	// Test NewTrace.
	g := NewEmptyTrace(N, nil)
	assert.Equal(t, N, g.Len(), "wrong digests size")
	assert.Equal(t, 0, len(g.Keys), "wrong keys initial size")
	g.Digests[0] = "a digest"

	assert.True(t, g.IsMissing(1), "values start missing")
	assert.False(t, g.IsMissing(0), "set values shouldn't be missing")
}

func TestTrace_Merge_Success(t *testing.T) {
	unittest.SmallTest(t)
	const N = 5
	const M = 7
	g := NewEmptyTrace(N, nil)
	g.Digests[0] = "a digest"
	gm := NewEmptyTrace(M, nil)
	gm.Digests[1] = "another digest"
	g2 := g.Merge(gm)
	assert.Equal(t, N+M, g2.Len(), "merge length wrong")
	assert.Equal(t, types.Digest("a digest"), g2.Digests[0])
	assert.Equal(t, types.Digest("another digest"), g2.Digests[6])
}

func TestTrace_Grow_FillBefore_Success(t *testing.T) {
	unittest.SmallTest(t)
	const N = 5
	g := NewEmptyTrace(N, nil)
	g.Digests[0] = "foo"
	g.Grow(2*N, FillBefore)
	assert.Equal(t, types.Digest("foo"), g.Digests[N], "Grow didn't FillBefore correctly")
}

func TestTrace_Grow_FillAfter_Success(t *testing.T) {
	unittest.SmallTest(t)
	const N = 5
	g := NewEmptyTrace(N, nil)
	g.Digests[0] = "foo"
	g.Grow(2*N, FillAfter)
	assert.Equal(t, types.Digest("foo"), g.Digests[0], "Grow didn't FillAfter correctly")
}

func TestTrace_Trim(t *testing.T) {
	unittest.SmallTest(t)
	const N = 5
	g := NewEmptyTrace(N, nil)
	g.Digests[1] = "foo"
	require.NoError(t, g.Trim(1, 3))
	assert.Equal(t, types.Digest("foo"), g.Digests[0], "Trim didn't copy correctly")
	assert.Equal(t, 2, g.Len(), "Trim wrong length")

	assert.Error(t, g.Trim(-1, 1))
	assert.Error(t, g.Trim(1, 3))
	assert.Error(t, g.Trim(2, 1))

	require.NoError(t, g.Trim(1, 1))
	assert.Equal(t, 0, g.Len(), "final size wrong")
}

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
