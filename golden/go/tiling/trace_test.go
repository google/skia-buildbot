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
	g := NewEmptyTrace(N, nil, nil)
	assert.Equal(t, N, g.Len(), "wrong digests size")
	assert.Equal(t, 0, len(g.keys), "wrong keys initial size")
	g.Digests[0] = "a digest"

	assert.True(t, g.IsMissing(1), "values start missing")
	assert.False(t, g.IsMissing(0), "set values shouldn't be missing")
}

func TestTrace_Merge_Success(t *testing.T) {
	unittest.SmallTest(t)
	const N = 5
	const M = 7
	firstTrace := NewEmptyTrace(N, map[string]string{
		"first":  "alpha",
		"second": "beta",
	}, map[string]string{
		"third":  "gamma",
		"fourth": "delta",
	})
	firstTrace.Digests[0] = "a digest"
	secondTrace := NewEmptyTrace(M, map[string]string{
		"zeroth": "thing before alpha",
		"second": "new second",
	}, map[string]string{
		"third": "gamma",
		"fifth": "epsilon",
	})
	secondTrace.Digests[1] = "another digest"
	mergedTrace := firstTrace.Merge(secondTrace)
	assert.Equal(t, N+M, mergedTrace.Len(), "merge length wrong")
	assert.Equal(t, types.Digest("a digest"), mergedTrace.Digests[0])
	assert.Equal(t, types.Digest("another digest"), mergedTrace.Digests[6])

	// Original traces should not have keys modified.
	assert.Equal(t, map[string]string{
		"first":  "alpha",
		"second": "beta",
	}, firstTrace.Keys())
	assert.Equal(t, map[string]string{
		"third":  "gamma",
		"fourth": "delta",
	}, firstTrace.Options())

	assert.Equal(t, map[string]string{
		"zeroth": "thing before alpha",
		"second": "new second",
	}, secondTrace.Keys())
	assert.Equal(t, map[string]string{
		"third": "gamma",
		"fifth": "epsilon",
	}, secondTrace.Options())

	// Combined trace should have the secondTrace values win, if there was a conflict
	assert.Equal(t, map[string]string{
		"zeroth": "thing before alpha",
		"first":  "alpha",
		"second": "new second",
	}, mergedTrace.Keys())
	assert.Equal(t, map[string]string{
		"third":  "gamma",
		"fourth": "delta",
		"fifth":  "epsilon",
	}, mergedTrace.Options())
}

func TestTrace_Grow_FillBefore_Success(t *testing.T) {
	unittest.SmallTest(t)
	const N = 5
	g := NewEmptyTrace(N, nil, nil)
	g.Digests[0] = "foo"
	g.Grow(2*N, FillBefore)
	assert.Equal(t, types.Digest("foo"), g.Digests[N], "Grow didn't FillBefore correctly")
}

func TestTrace_Grow_FillAfter_Success(t *testing.T) {
	unittest.SmallTest(t)
	const N = 5
	g := NewEmptyTrace(N, nil, nil)
	g.Digests[0] = "foo"
	g.Grow(2*N, FillAfter)
	assert.Equal(t, types.Digest("foo"), g.Digests[0], "Grow didn't FillAfter correctly")
}

func TestNewTrace_GettersReturnCorrectValues(t *testing.T) {
	unittest.SmallTest(t)
	trace := NewTrace([]types.Digest{digestOne, digestTwo, digestThree, digestTwo},
		map[string]string{
			types.PrimaryKeyField: "beta",
			types.CorpusField:     "dog",
		}, map[string]string{
			"optional": "value",
			"ext":      "png",
		})

	assert.Equal(t, map[string]string{
		types.PrimaryKeyField: "beta",
		types.CorpusField:     "dog",
	}, trace.Keys())
	assert.Equal(t, map[string]string{
		"optional": "value",
		"ext":      "png",
	}, trace.Options())
	assert.Equal(t, map[string]string{
		types.PrimaryKeyField: "beta",
		types.CorpusField:     "dog",
		"optional":            "value",
		"ext":                 "png",
	}, trace.KeysAndOptions())
	assert.Equal(t, 4, trace.Len())
	assert.Equal(t, "dog", trace.Corpus())
	assert.Equal(t, types.TestName("beta"), trace.TestName())
}

func TestTrace_MatchesIncludesKeysAndOptions(t *testing.T) {
	unittest.SmallTest(t)
	trace := NewTrace([]types.Digest{digestOne, digestTwo, digestThree, digestTwo},
		map[string]string{
			"alpha": "beta",
			"cat":   "dog",
		}, map[string]string{
			"optional": "value",
			"ext":      "png",
		})

	assert.True(t, trace.Matches(nil))
	assert.True(t, trace.Matches(paramtools.ParamSet{
		"alpha": []string{"delta", "gamma", "beta"},
	}))
	assert.True(t, trace.Matches(paramtools.ParamSet{
		"alpha": []string{"delta", "gamma", "beta"},
		"cat":   []string{"lion", "dog", "tiger"},
	}))
	assert.True(t, trace.Matches(paramtools.ParamSet{
		"ext": []string{"png", "jpeg"},
	}))
	assert.True(t, trace.Matches(paramtools.ParamSet{
		"ext":   []string{"png", "jpeg"},
		"alpha": []string{"beta"},
	}))

	assert.False(t, trace.Matches(paramtools.ParamSet{
		"not": []string{"in", "there"},
	}))
	assert.False(t, trace.Matches(paramtools.ParamSet{
		"alpha": []string{"no", "match"},
	}))
	assert.False(t, trace.Matches(paramtools.ParamSet{
		"alpha": []string{"beta", "matches"},
		"cat":   []string{"no", "match", "here"},
	}))
	assert.False(t, trace.Matches(paramtools.ParamSet{
		"ext": []string{"nope"},
	}))
	assert.False(t, trace.Matches(paramtools.ParamSet{
		"ext":   []string{"nope"},
		"alpha": []string{"beta"},
	}))
	assert.False(t, trace.Matches(paramtools.ParamSet{
		"ext":   []string{"png"},
		"alpha": []string{"nope"},
	}))
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

const (
	digestOne   = types.Digest("11111111111111111111111111111111")
	digestTwo   = types.Digest("22222222222222222222222222222222")
	digestThree = types.Digest("33333333333333333333333333333333")
)
