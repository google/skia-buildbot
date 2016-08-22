package params

import (
	"testing"

	"go.skia.org/infra/go/util"

	"github.com/stretchr/testify/assert"
)

func TestParamsNew(t *testing.T) {
	p := NewParams(",arch=x86,")
	assert.Equal(t, Params{"arch": "x86"}, p)
	p = NewParams(",arch=x86,config=565,")
	assert.Equal(t, Params{"arch": "x86", "config": "565"}, p)
}

func TestParams(t *testing.T) {
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
	p := ParamSet{"foo": []string{"bar", "baz"}}
	assert.Equal(t, []string{"foo"}, p.Keys())

	p = ParamSet{}
	assert.Equal(t, []string{}, p.Keys())
}

func TestAddParamsToParamSet(t *testing.T) {
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
