package util

import (
	"regexp"
	"sort"
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestAtMost(t *testing.T) {
	a := AtMost([]string{"a", "b"}, 3)
	if got, want := len(a), 2; got != want {
		t.Errorf("Wrong length: Got %v Want %v", got, want)
	}

	a = AtMost([]string{"a", "b"}, 1)
	if got, want := len(a), 1; got != want {
		t.Errorf("Wrong length: Got %v Want %v", got, want)
	}

	a = AtMost([]string{"a", "b"}, 0)
	if got, want := len(a), 0; got != want {
		t.Errorf("Wrong length: Got %v Want %v", got, want)
	}
}

func TestSSliceEqual(t *testing.T) {
	testcases := []struct {
		a    []string
		b    []string
		want bool
	}{
		{
			a:    []string{},
			b:    []string{},
			want: true,
		},
		{
			a:    nil,
			b:    []string{},
			want: false,
		},
		{
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			a:    []string{"foo"},
			b:    []string{},
			want: false,
		},
		{
			a:    []string{"foo", "bar"},
			b:    []string{"bar", "foo"},
			want: true,
		},
	}

	for _, tc := range testcases {
		if got, want := SSliceEqual(tc.a, tc.b), tc.want; got != want {
			t.Errorf("SSliceEqual(%#v, %#v): Got %v Want %v", tc.a, tc.b, got, want)
		}
	}
}

func TestIntersectIntSets(t *testing.T) {
	sets := []map[int]bool{
		map[int]bool{1: true, 2: true, 3: true, 4: true},
		map[int]bool{2: true, 4: true, 5: true, 7: true},
	}
	minIdx := 1
	intersect := IntersectIntSets(sets, minIdx)
	assert.Equal(t, map[int]bool{2: true, 4: true}, intersect)
}

func TestUnionStrings(t *testing.T) {
	ret := UnionStrings([]string{"abc", "abc"}, []string{"efg", "abc"})
	sort.Strings(ret)
	assert.Equal(t, []string{"abc", "efg"}, ret)

	assert.Equal(t, []string{}, UnionStrings())
	assert.Equal(t, []string{"abc"}, UnionStrings([]string{"abc"}))
	assert.Equal(t, []string{"abc"}, UnionStrings([]string{"abc", "abc", "abc"}))
}

func TestAddParamsToParamSet(t *testing.T) {
	testCases := []struct {
		a       map[string][]string
		b       map[string]string
		wantFoo []string
	}{
		{
			a: map[string][]string{
				"foo": []string{"a", "b"},
			},
			b: map[string]string{
				"foo": "c",
			},
			wantFoo: []string{"a", "b", "c"},
		},
		{
			a: map[string][]string{
				"foo": []string{},
			},
			b: map[string]string{
				"foo": "c",
			},
			wantFoo: []string{"c"},
		},
		{
			a: map[string][]string{
				"foo": []string{"c"},
			},
			b: map[string]string{
				"foo": "c",
			},
			wantFoo: []string{"c"},
		},
		{
			a: map[string][]string{},
			b: map[string]string{
				"foo": "c",
			},
			wantFoo: []string{"c"},
		},
		{
			a: map[string][]string{
				"foo": []string{"c"},
			},
			b:       map[string]string{},
			wantFoo: []string{"c"},
		},
	}
	for _, tc := range testCases {
		if got, want := AddParamsToParamSet(tc.a, tc.b)["foo"], tc.wantFoo; !SSliceEqual(got, want) {
			t.Errorf("Merge failed: Got %v Want %v", got, want)
		}
	}
}

func TestAddParamSetToParamSet(t *testing.T) {
	testCases := []struct {
		a       map[string][]string
		b       map[string][]string
		wantFoo []string
	}{
		{
			a: map[string][]string{
				"foo": []string{"a", "b"},
			},
			b: map[string][]string{
				"foo": []string{"c"},
			},
			wantFoo: []string{"a", "b", "c"},
		},
		{
			a: map[string][]string{
				"foo": []string{},
			},
			b: map[string][]string{
				"foo": []string{"c"},
			},
			wantFoo: []string{"c"},
		},
		{
			a: map[string][]string{
				"foo": []string{"c"},
			},
			b: map[string][]string{
				"foo": []string{},
			},
			wantFoo: []string{"c"},
		},
		{
			a: map[string][]string{
				"foo": []string{"c"},
			},
			b: map[string][]string{
				"bar": []string{"b"},
			},
			wantFoo: []string{"c"},
		},
	}
	for _, tc := range testCases {
		if got, want := AddParamSetToParamSet(tc.a, tc.b)["foo"], tc.wantFoo; !SSliceEqual(got, want) {
			t.Errorf("Merge failed: Got %v Want %v", got, want)
		}
	}
}

func TestAnyMatch(t *testing.T) {
	slice := []*regexp.Regexp{
		regexp.MustCompile("somestring"),
		regexp.MustCompile("^abcdefg$"),
		regexp.MustCompile("^defg123"),
		regexp.MustCompile("abc\\.xyz"),
	}
	tc := map[string]bool{
		"somestring":      true,
		"somestringother": true,
		"abcdefg":         true,
		"abcdefgh":        false,
		"defg1234":        true,
		"cdefg123":        false,
		"abc.xyz":         true,
		"abcqxyz":         false,
	}
	for s, e := range tc {
		assert.Equal(t, e, AnyMatch(slice, s))
	}
}
