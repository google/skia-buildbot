package paramtools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestParamsNew(t *testing.T) {
	unittest.SmallTest(t)
	p := NewParams(",arch=x86,")
	assert.Equal(t, Params{"arch": "x86"}, p)
	p = NewParams(",arch=x86,config=565,")
	assert.Equal(t, Params{"arch": "x86", "config": "565"}, p)
}

func TestAddParamsFromKey(t *testing.T) {
	unittest.SmallTest(t)
	p := ParamSet{}
	p.AddParamsFromKey(",arch=x86,")
	assert.Equal(t, ParamSet{"arch": []string{"x86"}}, p)
	p.AddParamsFromKey(",arch=x86,config=565,")
	assert.Equal(t, ParamSet{"arch": []string{"x86"}, "config": []string{"565"}}, p)
}

func TestParams(t *testing.T) {
	unittest.SmallTest(t)
	p := Params{"foo": "1", "bar": "2"}
	p2 := p.Copy()
	p["baz"] = "3"
	assert.NotEqual(t, p["baz"], p2["baz"])

	p.Add(p2)
	assert.Equal(t, p, Params{"foo": "1", "bar": "2", "baz": "3"})

	p.Add(nil)
	assert.Equal(t, p, Params{"foo": "1", "bar": "2", "baz": "3"})

	assert.True(t, p.Equal(p))
	assert.False(t, p2.Equal(p))
	assert.False(t, p.Equal(p2))
	assert.False(t, p.Equal(nil))

	assert.ElementsMatch(t, []string{"bar", "baz", "foo"}, p.Keys())

	var pnil Params
	assert.True(t, pnil.Equal(Params{}))
	assert.Equal(t, []string{}, pnil.Keys())

	p4 := Params{}
	p5 := Params{"foo": "bar", "fred": "barney"}
	p6 := Params{"foo": "baz", "qux": "corge"}
	p4.Add(p5, p6)
	assert.Equal(t, p4, Params{"foo": "baz", "qux": "corge", "fred": "barney"})
}

func TestParamSet(t *testing.T) {
	unittest.SmallTest(t)
	p := ParamSet{"foo": []string{"bar", "baz"}}
	assert.Equal(t, []string{"foo"}, p.Keys())

	p = ParamSet{}
	assert.Equal(t, []string{}, p.Keys())
}

func TestAddParamsToParamSet(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		a       ParamSet
		b       Params
		wantFoo []string
	}{
		{
			a: ParamSet{
				"foo": []string{"a", "b"},
			},
			b: Params{
				"foo": "c",
			},
			wantFoo: []string{"a", "b", "c"},
		},
		{
			a: ParamSet{
				"foo": []string{},
			},
			b: Params{
				"foo": "c",
			},
			wantFoo: []string{"c"},
		},
		{
			a: ParamSet{
				"foo": []string{"c"},
			},
			b: Params{
				"foo": "c",
			},
			wantFoo: []string{"c"},
		},
		{
			a: ParamSet{},
			b: Params{
				"foo": "c",
			},
			wantFoo: []string{"c"},
		},
		{
			a: ParamSet{
				"foo": []string{"c"},
			},
			b:       Params{},
			wantFoo: []string{"c"},
		},
	}
	for _, tc := range testCases {
		tc.a.AddParams(tc.b)
		assert.ElementsMatch(t, tc.wantFoo, tc.a["foo"])
	}
}

func TestAddParamSetToParamSet(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		a       ParamSet
		b       ParamSet
		wantFoo []string
	}{
		{
			a: ParamSet{
				"foo": []string{"a", "b"},
			},
			b: ParamSet{
				"foo": []string{"c"},
			},
			wantFoo: []string{"a", "b", "c"},
		},
		{
			a: ParamSet{
				"foo": []string{},
			},
			b: ParamSet{
				"foo": []string{"c"},
			},
			wantFoo: []string{"c"},
		},
		{
			a: ParamSet{
				"foo": []string{"c"},
			},
			b: ParamSet{
				"foo": []string{},
			},
			wantFoo: []string{"c"},
		},
		{
			a: ParamSet{
				"foo": []string{"c"},
			},
			b: ParamSet{
				"bar": []string{"b"},
			},
			wantFoo: []string{"c"},
		},
		{
			a:       ParamSet{},
			b:       ParamSet{},
			wantFoo: nil,
		},
	}
	for _, tc := range testCases {
		tc.a.AddParamSet(tc.b)
		assert.ElementsMatch(t, tc.wantFoo, tc.a["foo"])
	}
}

func TestParamSetCopy(t *testing.T) {
	unittest.SmallTest(t)
	p := ParamSet{
		"foo": []string{"bar", "baz"},
		"qux": []string{"quux"},
	}
	cp := p.Copy()
	assert.Equal(t, p, cp)
	p["foo"] = []string{"fred"}
	assert.NotEqual(t, p, cp)

	assert.Equal(t, ParamSet{}, ParamSet{}.Copy())
}

func TestMatchAny(t *testing.T) {
	unittest.SmallTest(t)
	recParams := ParamSet{
		"foo": {"1", "2"},
		"bar": {"a", "b", "c"},
		"baz": {"u", "v", "w"},
	}

	rule1 := ParamSet{"foo": {"1"}}
	rule2 := ParamSet{"bar": {"e"}}
	rule3 := ParamSet{"baz": {"v", "w"}}
	rule4 := ParamSet{"x": {"something"}}
	empty := ParamSet{}

	assert.True(t, ParamMatcher{rule1}.MatchAny(recParams))
	assert.False(t, ParamMatcher{rule2}.MatchAny(recParams))
	assert.True(t, ParamMatcher{rule3}.MatchAny(recParams))
	assert.False(t, ParamMatcher{rule4}.MatchAny(recParams))
	assert.True(t, ParamMatcher{empty}.MatchAny(recParams))

	assert.True(t, ParamMatcher{rule1, rule2}.MatchAny(recParams))
	assert.True(t, ParamMatcher{rule1, rule3}.MatchAny(recParams))
	assert.True(t, ParamMatcher{rule2, rule3}.MatchAny(recParams))
	assert.False(t, ParamMatcher{rule2, rule4}.MatchAny(recParams))
	assert.True(t, ParamMatcher{rule2, rule4, empty}.MatchAny(recParams))

	assert.False(t, ParamMatcher{rule1}.MatchAny(ParamSet{}))
	assert.True(t, ParamMatcher{empty}.MatchAny(ParamSet{}))
	assert.False(t, ParamMatcher{}.MatchAny(ParamSet{}))

	// Test with some realistic data.
	testVal := ParamSet{
		"cpu_or_gpu":       {"GPU"},
		"config":           {"gles", "glesdft"},
		"ext":              {"png"},
		"name":             {"drrect_small_inner"},
		"source_type":      {"gm"},
		"cpu_or_gpu_value": {"GT7800"},
		"os":               {"iOS"},
		"gamma_correct":    {"no"},
		"configuration":    {"Release"},
		"model":            {"iPadPro"},
		"compiler":         {"Clang"},
		"arch":             {"arm64"},
	}

	testRule := ParamSet{
		"config":      {"gldft", "glesdft"},
		"model":       {"AlphaR2", "AndroidOne", "ShuttleC", "ZBOX", "iPadPro", "iPhone7"},
		"name":        {"drrect_small_inner"},
		"source_type": {"gm"},
	}
	assert.True(t, ParamMatcher{testRule}.MatchAny(testVal))
}

func TestMatchAnyParams(t *testing.T) {
	unittest.SmallTest(t)
	recParams := Params{
		"foo": "1",
		"bar": "a",
		"baz": "v",
	}

	rule1 := ParamSet{"foo": {"1"}}
	rule2 := ParamSet{"bar": {"e"}}
	rule3 := ParamSet{"baz": {"v", "w"}}
	rule4 := ParamSet{"x": {"something"}}
	empty := ParamSet{}

	assert.True(t, ParamMatcher{rule1}.MatchAnyParams(recParams))
	assert.False(t, ParamMatcher{rule2}.MatchAnyParams(recParams))
	assert.True(t, ParamMatcher{rule3}.MatchAnyParams(recParams))
	assert.False(t, ParamMatcher{rule4}.MatchAnyParams(recParams))
	assert.True(t, ParamMatcher{empty}.MatchAnyParams(recParams))

	assert.True(t, ParamMatcher{rule1, rule2}.MatchAnyParams(recParams))
	assert.True(t, ParamMatcher{rule1, rule3}.MatchAnyParams(recParams))
	assert.True(t, ParamMatcher{rule2, rule3}.MatchAnyParams(recParams))
	assert.False(t, ParamMatcher{rule2, rule4}.MatchAnyParams(recParams))
	assert.True(t, ParamMatcher{rule2, rule4, empty}.MatchAnyParams(recParams))

	assert.False(t, ParamMatcher{rule1}.MatchAnyParams(Params{}))
	assert.True(t, ParamMatcher{empty}.MatchAnyParams(Params{}))
	assert.False(t, ParamMatcher{}.MatchAnyParams(Params{}))
}

// roundTripsEncode tests that an Ord

func TestReadOnlyParamSet_NewNonEmptyParamSet_Success(t *testing.T) {
	unittest.SmallTest(t)
	ps := NewReadOnlyParamSet(Params{"a": "b"}, Params{"a": "c"}, Params{"b": "e"})
	require.Equal(t, ReadOnlyParamSet{"a": []string{"b", "c"}, "b": []string{"e"}}, ps)
}

func TestReadOnlyParamSet_NewEmptyParamSet_Success(t *testing.T) {
	unittest.SmallTest(t)
	require.Equal(t, ReadOnlyParamSet{}, NewReadOnlyParamSet())
}

func TestParamSet_Freeze_ReturnsReadOnlyParamSet(t *testing.T) {
	unittest.SmallTest(t)
	ps := NewParamSet(Params{"a": "b"}, Params{"a": "c"}, Params{"b": "e"})

	require.Equal(t, ReadOnlyParamSet{"a": []string{"b", "c"}, "b": []string{"e"}}, ps.Freeze())
}

func TestParamSetFrozenCopy_NonEmptyParamSet_Success(t *testing.T) {
	unittest.SmallTest(t)
	p := ParamSet{
		"foo": []string{"bar", "baz"},
		"qux": []string{"quux"},
	}
	cp := p.FrozenCopy()
	assert.Equal(t, ReadOnlyParamSet(p), cp)

	// Confirm we made a deep copy by modifying the original.
	p["foo"] = []string{"fred"}
	assert.NotEqual(t, ReadOnlyParamSet(p), cp)
}

func TestParamSetFrozenCopy_EmptyParamSet_Success(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, ReadOnlyParamSet{}, ParamSet{}.FrozenCopy())
}

func TestParamSetAddParamSet_NonEmptyReadOnlyParamSet_Success(t *testing.T) {
	unittest.SmallTest(t)
	ps := NewParamSet()
	rops := ReadOnlyParamSet{
		"foo": {"bar", "baz"},
		"qux": {"quux"},
	}
	ps.AddParamSet(rops)
	assert.Equal(t, ReadOnlyParamSet(ps), rops)
}

func TestParamSetEqual_KeysAndValuesMatch_ReturnsTrue(t *testing.T) {
	unittest.SmallTest(t)

	assert.True(t, ParamSet{}.Equal(nil))
	assert.True(t, ParamSet{
		"alpha": {"beta", "gamma", "delta"},
	}.Equal(ReadOnlyParamSet{
		"alpha": {"gamma", "delta", "beta"},
	}))
	assert.True(t, ParamSet{
		"alpha":   {"beta", "gamma", "delta"},
		"epsilon": {},
		"lambda":  {"mu", "kappa"},
	}.Equal(map[string][]string{
		"alpha":   {"gamma", "delta", "beta"},
		"epsilon": {},
		"lambda":  {"kappa", "mu"},
	}))
}

func TestParamSetEqual_KeysAndValuesDoNotMatch_ReturnsFalse(t *testing.T) {
	unittest.SmallTest(t)

	assert.False(t, ParamSet{}.Equal(ParamSet{
		"something": {"something else"},
	}))
	assert.False(t, ParamSet{
		"something": {"something else"},
	}.Equal(ParamSet{}))
	assert.False(t, ParamSet{
		"something": {"something else"},
	}.Equal(nil))
	assert.False(t, ParamSet{
		"alpha": {"beta", "gamma", "delta"},
	}.Equal(ParamSet{
		"alpha": {"gamma", "delta"},
	}))
	assert.False(t, ParamSet{
		"alpha": {"beta", "delta"},
	}.Equal(ParamSet{
		"alpha": {"gamma", "delta", "beta"},
	}))
	assert.False(t, ParamSet{
		"alpha": {"beta", "delta", "gamma", "gamma"},
	}.Equal(ParamSet{
		"alpha": {"gamma", "delta", "beta"},
	}))
	assert.False(t, ParamSet{
		"alpha": {"beta", "delta", "gamma"},
	}.Equal(ParamSet{
		"alpha": {"gamma", "delta", "beta", "gamma"},
	}))
	assert.False(t, ParamSet{
		"alpha":   {"beta", "gamma", "delta"},
		"epsilon": {},
		"lambda":  {"mu", "kappa"},
	}.Equal(ParamSet{
		"alpha":  {"gamma", "delta", "beta"},
		"lambda": {"kappa", "mu"},
	}))
	assert.False(t, ParamSet{
		"alpha":  {"beta", "gamma", "delta"},
		"lambda": {"mu", "kappa"},
	}.Equal(map[string][]string{
		"alpha":   {"gamma", "delta", "beta"},
		"epsilon": {},
		"lambda":  {"kappa", "mu"},
	}))
	assert.False(t, ParamSet{
		"alpha":  {"beta", "gamma", "delta"},
		"delta":  {},
		"lambda": {"mu", "kappa"},
	}.Equal(ParamSet{
		"alpha":   {"gamma", "delta", "beta"},
		"epsilon": {},
		"lambda":  {"kappa", "mu"},
	}))
	assert.False(t, ParamSet{
		"alpha": {"beta", "gamma", "delta"},
	}.Equal(ParamSet{
		"alpha": {"gamma", "delta", "bettttttttttta"},
	}))
}
