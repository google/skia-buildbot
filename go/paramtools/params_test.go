package paramtools

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

// roundTripsEncode tests that an OrdererParamSet survives a round-trip
// through encoding and decoding.
func roundTripsEncode(t *testing.T, p *OrderedParamSet) {
	b, err := p.Encode()
	assert.NoError(t, err)
	back, err := NewOrderedParamSetFromBytes(b)
	assert.NoError(t, err)
	assert.Equal(t, p, back)
}

func TestOrderedParamSetEmpty(t *testing.T) {
	unittest.SmallTest(t)
	// Confirm that empty OrderedParamSet's round-trip correctly.
	roundTripsEncode(t, NewOrderedParamSet())
}

func TestOrderedParamSetCopy(t *testing.T) {
	unittest.SmallTest(t)

	p := NewOrderedParamSet()
	// Add some data.
	p.Update(
		ParamSet{
			"arch": []string{"riscv", "arm"},
		},
	)
	// Make a copy and modify that copy.
	p2 := p.Copy()
	p2.Update(
		ParamSet{
			"config": []string{"8888", "565", "gpu"},
		},
	)
	// Config they are different.
	assert.Equal(t, []string{"arch"}, p.KeyOrder)
	assert.Equal(t, []string{"arch", "config"}, p2.KeyOrder)
	_, err := p.EncodeParamsAsString(Params{"arch": "arm"})
	assert.NoError(t, err)
	_, err = p.EncodeParamsAsString(Params{"config": "8888"})
	assert.Error(t, err)
	_, err = p2.EncodeParamsAsString(Params{"config": "8888"})
	assert.NoError(t, err)
}

func TestOrderedParamSetStartFromEmpty(t *testing.T) {
	unittest.SmallTest(t)
	// Start from an empty OrderedParamSet.
	p := NewOrderedParamSet()

	// Add some data.
	p.Update(
		ParamSet{
			"arch": []string{"riscv", "arm"},
		},
	)
	// Does it encode params correctly?
	s, err := p.EncodeParamsAsString(Params{"arch": "arm"})
	assert.NoError(t, err)
	assert.Equal(t, ",0=1,", s)
	s, err = p.EncodeParamsAsString(Params{"config": "8888"})
	assert.Error(t, err)

	// Add more data.
	p.Update(
		ParamSet{
			"config": []string{"8888", "565", "gpu"},
		},
	)
	// Does is still roundtrip?
	roundTripsEncode(t, p)

	// Does it encode params correctly?
	s, err = p.EncodeParamsAsString(Params{"arch": "riscv", "config": "gpu"})
	assert.NoError(t, err)
	assert.Equal(t, ",0=0,1=2,", s)
	s, err = p.EncodeParamsAsString(Params{"config": "101010"})
	assert.Error(t, err)
}

func TestOrderedParamSet(t *testing.T) {
	unittest.SmallTest(t)
	// Start with an already populated OrderedParamSet.
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

	// Does it round-trip?
	roundTripsEncode(t, p)

	// Confirm that Delta() works.
	p2 := ParamSet{
		"config": []string{"8888", "gles"},
		"arch":   []string{"riscv", "arm"},
		"srgb":   []string{"true", "false"},
	}
	toAdd := p.Delta(p2)
	expected := ParamSet{
		"config": []string{"gles"},
		"arch":   []string{"riscv"},
		"srgb":   []string{"true", "false"},
	}
	assert.Equal(t, expected, toAdd)

	// Use the given delta to update the OrderedParamSet.
	p.Update(toAdd)
	assert.Equal(t, []string{"config", "name", "arch", "srgb"}, p.KeyOrder)
	assert.Equal(t, []string{"true", "false"}, p.ParamSet["srgb"])
	assert.Equal(t, []string{"8888", "565", "gpu", "gles"}, p.ParamSet["config"])

	// Confirm the updated OPS encodes and decodes Params and ParamSets correctly.
	params := Params{
		"config": "8888",
		"arch":   "x86",
		"name":   "test01",
	}
	s, err := p.EncodeParamsAsString(params)
	assert.NoError(t, err)
	assert.Equal(t, ",0=0,1=1,2=0,", s)
	ep, err := p.EncodeParams(params)
	assert.NoError(t, err)
	assert.Equal(t, Params{"0": "0", "1": "1", "2": "0"}, ep)
	eps, err := p.EncodeParamSet(NewParamSet(params))
	assert.NoError(t, err)
	assert.Equal(t, ParamSet{"0": []string{"0"}, "1": []string{"1"}, "2": []string{"0"}}, eps)

	paramsDecoded, err := p.DecodeParamsFromString(s)
	assert.NoError(t, err)
	assert.Equal(t, nil, err)
	assert.Equal(t, params, paramsDecoded)

	// Test encoding an empty Params.
	s, err = p.EncodeParamsAsString(Params{})
	assert.Error(t, err)
	_, err = p.EncodeParams(Params{})
	assert.Error(t, err)
	_, err = p.EncodeParamSet(ParamSet{})
	assert.Error(t, err)

	// Test values at the end of the ParamSet.
	params = Params{
		"config": "gles",
		"arch":   "arm",
		"name":   "test68",
	}
	s, err = p.EncodeParamsAsString(params)
	assert.NoError(t, err)
	assert.Equal(t, ",0=3,1=68,2=1,", s)
	ep, err = p.EncodeParams(params)
	assert.NoError(t, err)
	assert.Equal(t, Params{"0": "3", "1": "68", "2": "1"}, ep)
	eps, err = p.EncodeParamSet(NewParamSet(params))
	assert.NoError(t, err)
	assert.Equal(t, ParamSet{"0": []string{"3"}, "1": []string{"68"}, "2": []string{"1"}}, eps)

	paramsDecoded, err = p.DecodeParamsFromString(s)
	assert.NoError(t, err)
	assert.Equal(t, params, paramsDecoded)

	// Encoding and Decoding error conditions.
	params = Params{
		"config": "some unknown value",
	}
	_, err = p.EncodeParamsAsString(params)
	assert.Error(t, err)
	_, err = p.EncodeParams(params)
	assert.Error(t, err)
	_, err = p.EncodeParamSet(NewParamSet(params))
	assert.Error(t, err)

	_, err = p.DecodeParamsFromString("")
	assert.NoError(t, err, "Shouldn't fail since an empty string decodes to an empty Params.")

	_, err = p.DecodeParamsFromString(",,")
	assert.NoError(t, err, "Shouldn't fail since an empty structured key decodes to an empty Params.")

	_, err = p.DecodeParamsFromString(",1,")
	assert.Error(t, err, "Should fail since is a bad pair.")

	_, err = p.DecodeParamsFromString(",1=foo,")
	assert.Error(t, err, "Should fail since foo isn't found.")

	_, err = p.DecodeParamsFromString(",-1=0,")
	assert.Error(t, err, "Should fail since -1 isn't found.")
}

func TestParamSet_Size(t *testing.T) {
	unittest.SmallTest(t)
	tests := []struct {
		name string
		p    ParamSet
		want int
	}{
		{
			name: "nil",
			p:    nil,
			want: 0,
		},

		{
			name: "empty",
			p:    ParamSet{},
			want: 0,
		},
		{
			name: "simple",
			p:    ParamSet{"foo": []string{"bar", "baz"}},
			want: 2,
		},
		{
			name: "2 values",
			p: ParamSet{
				"foo": []string{"bar", "baz"},
				"bar": []string{"baz"},
			},
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Size(); got != tt.want {
				t.Errorf("ParamSet.Size() = %v, want %v", got, tt.want)
			}
		})
	}
}
