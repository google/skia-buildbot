package paramtools

import (
	"testing"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"

	"github.com/stretchr/testify/assert"
)

func TestParamsNew(t *testing.T) {
	testutils.SmallTest(t)
	p := NewParams(",arch=x86,")
	assert.Equal(t, Params{"arch": "x86"}, p)
	p = NewParams(",arch=x86,config=565,")
	assert.Equal(t, Params{"arch": "x86", "config": "565"}, p)
}

func TestAddParamsFromKey(t *testing.T) {
	testutils.SmallTest(t)
	p := ParamSet{}
	p.AddParamsFromKey(",arch=x86,")
	assert.Equal(t, ParamSet{"arch": []string{"x86"}}, p)
	p.AddParamsFromKey(",arch=x86,config=565,")
	assert.Equal(t, ParamSet{"arch": []string{"x86"}, "config": []string{"565"}}, p)
}

func TestParams(t *testing.T) {
	testutils.SmallTest(t)
	p := Params{"foo": "1", "bar": "2"}
	p2 := p.Dup()
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

	assert.True(t, util.SSliceEqual([]string{"bar", "baz", "foo"}, p.Keys()))

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
	testutils.SmallTest(t)
	p := ParamSet{"foo": []string{"bar", "baz"}}
	assert.Equal(t, []string{"foo"}, p.Keys())

	p = ParamSet{}
	assert.Equal(t, []string{}, p.Keys())
}

func TestAddParamsToParamSet(t *testing.T) {
	testutils.SmallTest(t)
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
		if got, want := tc.a["foo"], tc.wantFoo; !util.SSliceEqual(got, want) {
			t.Errorf("Merge failed: Got %v Want %v", got, want)
		}
	}
}

func TestAddParamSetToParamSet(t *testing.T) {
	testutils.SmallTest(t)
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
		if got, want := tc.a["foo"], tc.wantFoo; !util.SSliceEqual(got, want) {
			t.Errorf("Merge failed: Got %v Want %v", got, want)
		}
	}
}

func TestParamSetCopy(t *testing.T) {
	testutils.SmallTest(t)
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

func TestMatching(t *testing.T) {
	testutils.SmallTest(t)
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

	// Test with some realistice data.
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

func TestOrderedParamSet(t *testing.T) {
	p := &OrderedParamSet{
		KeyOrder: []string{"config", "name", "arch"},
		ParamSet: ParamSet{
			"config": []string{"8888", "565", "gpu"},
			"arch":   []string{"x86", "arm"},
			"name": []string{
				"test00", "test01", "test02", "test03", "test04", "test05", "test06", "test07", "test08", "test09",
				"test10", "test11", "test12", "test13", "test14", "test15", "test16", "test17", "test18", "test19",
				"test20", "test21", "test22", "test23", "test24", "test25", "test26", "test27", "test28", "test29",
				"test30", "test31", "test32", "test33", "test34", "test35", "test36", "test37", "test38", "test39",
				"test40", "test41", "test42", "test43", "test44", "test45", "test46", "test47", "test48", "test49",
				"test50", "test51", "test52", "test53", "test54", "test55", "test56", "test57", "test58", "test59",
				"test60", "test61", "test62", "test63", "test64", "test65", "test66", "test67", "test68", "test69",
			},
		},
	}
	b, err := p.Encode()
	assert.NoError(t, err)
	assert.Len(t, b, 249) // Raw text is >700 chars.
	back, err := NewOrderedParamSetFromBytes(b)
	assert.NoError(t, err)
	assert.Equal(t, p, back)

	p2 := ParamSet{
		"config": []string{"8888", "gles"},
		"arch":   []string{"riscv", "arm"},
		"srgb":   []string{"true", "false"},
	}
	toAdd := p.Check(p2)
	expected := ParamSet{
		"config": []string{"gles"},
		"arch":   []string{"riscv"},
		"srgb":   []string{"true", "false"},
	}
	assert.Equal(t, expected, toAdd)
	p.Update(toAdd)
	assert.Equal(t, []string{"config", "name", "arch", "srgb"}, p.KeyOrder)
	assert.Equal(t, []string{"true", "false"}, p.ParamSet["srgb"])
	assert.Equal(t, []string{"8888", "565", "gpu", "gles"}, p.ParamSet["config"])

	params := Params{
		"config": "8888",
		"arch":   "x86",
		"name":   "test01",
	}
	b, err = p.EncodeParams(params)
	assert.NoError(t, err)
	assert.Len(t, b, 6)
	paramsDecoded, err := p.DecodeParams(b)
	assert.NoError(t, err)
	assert.Equal(t, params, paramsDecoded)

	params = Params{
		"config": "gles",
		"arch":   "arm",
		"name":   "test68",
	}
	b, err = p.EncodeParams(params)
	assert.NoError(t, err)
	assert.Len(t, b, 7)
	paramsDecoded, err = p.DecodeParams(b)
	assert.NoError(t, err)
	assert.Equal(t, params, paramsDecoded)
}
