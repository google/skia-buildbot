package util

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestAtMost(t *testing.T) {
	unittest.SmallTest(t)
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
	unittest.SmallTest(t)
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

func TestInsertString(t *testing.T) {
	unittest.SmallTest(t)
	assertdeep.Equal(t, []string{"a"}, InsertString([]string{}, 0, "a"))
	assertdeep.Equal(t, []string{"b", "a"}, InsertString([]string{"a"}, 0, "b"))
	assertdeep.Equal(t, []string{"b", "c", "a"}, InsertString([]string{"b", "a"}, 1, "c"))
	assertdeep.Equal(t, []string{"b", "c", "a", "d"}, InsertString([]string{"b", "c", "a"}, 3, "d"))
}

func TestInsertStringSorted(t *testing.T) {
	unittest.SmallTest(t)
	assertdeep.Equal(t, []string{"a"}, InsertStringSorted([]string{}, "a"))
	assertdeep.Equal(t, []string{"a"}, InsertStringSorted([]string{"a"}, "a"))
	assertdeep.Equal(t, []string{"a", "b"}, InsertStringSorted([]string{"a"}, "b"))
	assertdeep.Equal(t, []string{"0", "a", "b"}, InsertStringSorted([]string{"a", "b"}, "0"))
	assertdeep.Equal(t, []string{"0", "a", "b"}, InsertStringSorted([]string{"0", "a", "b"}, "b"))
}

func TestIntersectIntSets(t *testing.T) {
	unittest.SmallTest(t)
	sets := []map[int]bool{
		{1: true, 2: true, 3: true, 4: true},
		{2: true, 4: true, 5: true, 7: true},
	}
	minIdx := 1
	intersect := IntersectIntSets(sets, minIdx)
	require.Equal(t, map[int]bool{2: true, 4: true}, intersect)
}

func TestAddParamsToParamSet(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		a       map[string][]string
		b       map[string]string
		wantFoo []string
	}{
		{
			a: map[string][]string{
				"foo": {"a", "b"},
			},
			b: map[string]string{
				"foo": "c",
			},
			wantFoo: []string{"a", "b", "c"},
		},
		{
			a: map[string][]string{
				"foo": {},
			},
			b: map[string]string{
				"foo": "c",
			},
			wantFoo: []string{"c"},
		},
		{
			a: map[string][]string{
				"foo": {"c"},
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
				"foo": {"c"},
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
	unittest.SmallTest(t)
	testCases := []struct {
		a       map[string][]string
		b       map[string][]string
		wantFoo []string
	}{
		{
			a: map[string][]string{
				"foo": {"a", "b"},
			},
			b: map[string][]string{
				"foo": {"c"},
			},
			wantFoo: []string{"a", "b", "c"},
		},
		{
			a: map[string][]string{
				"foo": {},
			},
			b: map[string][]string{
				"foo": {"c"},
			},
			wantFoo: []string{"c"},
		},
		{
			a: map[string][]string{
				"foo": {"c"},
			},
			b: map[string][]string{
				"foo": {},
			},
			wantFoo: []string{"c"},
		},
		{
			a: map[string][]string{
				"foo": {"c"},
			},
			b: map[string][]string{
				"bar": {"b"},
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
	unittest.SmallTest(t)
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
		require.Equal(t, e, AnyMatch(slice, s))
	}
}

func TestIsNil(t *testing.T) {
	unittest.SmallTest(t)
	require.True(t, IsNil(nil))
	require.False(t, IsNil(false))
	require.False(t, IsNil(0))
	require.False(t, IsNil(""))
	require.False(t, IsNil([0]int{}))
	type Empty struct{}
	require.False(t, IsNil(Empty{}))
	require.True(t, IsNil(chan interface{}(nil)))
	require.False(t, IsNil(make(chan interface{})))
	var f func()
	require.True(t, IsNil(f))
	require.False(t, IsNil(func() {}))
	require.True(t, IsNil(map[bool]bool(nil)))
	require.False(t, IsNil(make(map[bool]bool)))
	require.True(t, IsNil([]int(nil)))
	require.False(t, IsNil([][]int{nil}))
	require.True(t, IsNil((*int)(nil)))
	var i int
	require.False(t, IsNil(&i))
	var pi *int
	require.True(t, IsNil(pi))
	require.True(t, IsNil(&pi))
	var ppi **int
	require.True(t, IsNil(&ppi))
	var c chan interface{}
	require.True(t, IsNil(&c))
	var w io.Writer
	require.True(t, IsNil(w))
	w = (*bytes.Buffer)(nil)
	require.True(t, IsNil(w))
	w = &bytes.Buffer{}
	require.False(t, IsNil(w))
	require.False(t, IsNil(&w))
	var ii interface{}
	ii = &pi
	require.True(t, IsNil(ii))
}

func TestUnixFloatToTime(t *testing.T) {
	unittest.SmallTest(t)
	cases := []struct {
		in  float64
		out time.Time
	}{
		{
			in:  1414703190.292151927,
			out: time.Unix(1414703190, 292000000),
		},
	}
	for _, tc := range cases {
		require.Equal(t, tc.out, UnixFloatToTime(tc.in))
	}
}

func TestTimeToUnixFloat(t *testing.T) {
	unittest.SmallTest(t)
	cases := []struct {
		in  time.Time
		out float64
	}{
		{
			in:  time.Unix(1414703190, 292000000),
			out: 1414703190.292000,
		},
	}
	for _, tc := range cases {
		require.Equal(t, tc.out, TimeToUnixFloat(tc.in))
	}
}

func TestTimeConversion(t *testing.T) {
	unittest.SmallTest(t)
	cases := []float64{
		0.0,
		1.0,
		1414703190.0,
		1414703190.292000,
	}
	for _, tc := range cases {
		require.Equal(t, tc, TimeToUnixFloat(UnixFloatToTime(tc)))
	}
}

func TestMD5Hash(t *testing.T) {
	unittest.SmallTest(t)
	m_1 := map[string]string{"key1": "val1"}
	m_2 := map[string]string{}
	var m_3 map[string]string = nil
	m_4 := map[string]string{
		"k3": "v1",
		"k2": "v2",
		"k1": "v3",
		"k4": "v4",
	}

	h_1, err := MD5Sum(m_1)
	require.NoError(t, err)

	h_2, err := MD5Sum(m_2)
	require.NoError(t, err)

	h_3, err := MD5Sum(m_3)
	require.NoError(t, err)
	require.Equal(t, 32, len(h_1))
	require.Equal(t, 32, len(h_2))
	require.Equal(t, 32, len(h_3))
	require.NotEqual(t, h_1, h_2)
	require.NotEqual(t, h_1, h_3)
	require.Equal(t, h_2, h_3)

	// Ensure that we get the same hash every time.
	h_4, err := MD5Sum(m_4)
	require.NoError(t, err)
	for i := 0; i < 100; i++ {
		h, err := MD5Sum(m_4)
		require.NoError(t, err)
		require.Equal(t, h_4, h)
	}
	h, err := MD5Sum(map[string]string{
		"k4": "v4",
		"k2": "v2",
		"k3": "v1",
		"k1": "v3",
	})
	require.NoError(t, err)
	require.Equal(t, h_4, h)
}

func TestBugsFromCommitMsg(t *testing.T) {
	unittest.SmallTest(t)
	cases := []struct {
		in  string
		out map[string][]string
	}{
		{
			in: "BUG=skia:1234",
			out: map[string][]string{
				"skia": {"1234"},
			},
		},
		{
			in: "BUG=skia:1234,skia:4567",
			out: map[string][]string{
				"skia": {"1234", "4567"},
			},
		},
		{
			in: "BUG=skia:1234,skia:4567,skia:8901",
			out: map[string][]string{
				"skia": {"1234", "4567", "8901"},
			},
		},
		{
			in: "BUG=1234",
			out: map[string][]string{
				"chromium": {"1234"},
			},
		},
		{
			in: "BUG=skia:1234, 456",
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"1234"},
			},
		},
		{
			in: "BUG=skia:1234,456",
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"1234"},
			},
		},
		{
			in: `Lorem ipsum dolor sit amet, consectetur adipiscing elit.

Quisque feugiat, mi et tristique dignissim, sapien risus tristique mi, non dignissim nibh erat ut ex.

BUG=1234, skia:5678
`,
			out: map[string][]string{
				"chromium": {"1234"},
				"skia":     {"5678"},
			},
		},
		{
			in: "Bug: skia:1234",
			out: map[string][]string{
				"skia": {"1234"},
			},
		},
		{
			in: "Bug: skia:1234,skia:4567",
			out: map[string][]string{
				"skia": {"1234", "4567"},
			},
		},
		{
			in: "Bug: skia:1234,skia:4567,skia:8901",
			out: map[string][]string{
				"skia": {"1234", "4567", "8901"},
			},
		},
		{
			in: "Bug: 1234",
			out: map[string][]string{
				"chromium": {"1234"},
			},
		},
		{
			in: "Bug: skia:1234, 456",
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"1234"},
			},
		},
		{
			in: "Bug: skia:1234,456",
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"1234"},
			},
		},
		{
			in: "Bug: 1234,456",
			out: map[string][]string{
				"chromium": {"1234", "456"},
			},
		},
		{
			in: "Bug: skia:1234,chromium:456",
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"1234"},
			},
		},
		{
			in: `asdf
Bug: skia:1234,456
BUG=skia:888
`,
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"1234", "888"},
			},
		},
		{
			in: "Bug: skia:123 chromium:456",
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"123"},
			},
		},
		{
			in: "Bug: skia:123, chromium:456",
			out: map[string][]string{
				"chromium": {"456"},
				"skia":     {"123"},
			},
		},
		{
			in: "Bug: skia:123,chromium:",
			out: map[string][]string{
				"skia": {"123"},
			},
		},
		{
			in: "Bug: b/123",
			out: map[string][]string{
				BUG_PROJECT_BUGANIZER: {"123"},
			},
		},
		{
			in: "Bug: skia:123,b/456",
			out: map[string][]string{
				"skia":                {"123"},
				BUG_PROJECT_BUGANIZER: {"456"},
			},
		},
		{
			in: `testing
Test: tested
BUG=skia:123
Bug: skia:456
BUG=b/123
Bug: b/234`,
			out: map[string][]string{
				"skia":                {"123", "456"},
				BUG_PROJECT_BUGANIZER: {"123", "234"},
			},
		},
		{
			in: `testing
Test: tested
BUG=skia:123
Bug: skia:456
BUG=ba/123
Bug: bb/234`,
			out: map[string][]string{
				"skia": {"123", "456"},
			},
		},
	}
	for _, tc := range cases {
		result := BugsFromCommitMsg(tc.in)
		require.Equal(t, tc.out, result)
	}
}

func TestIsDirEmpty(t *testing.T) {
	unittest.SmallTest(t)
	d, err := ioutil.TempDir(os.TempDir(), "test_empty")
	require.NoError(t, err)
	defer RemoveAll(d)

	// Directory is initially empty.
	empty, err := IsDirEmpty(d)
	require.NoError(t, err)
	require.True(t, empty)

	// Add a file in the directory.
	f, err := ioutil.TempFile(d, "test_file")
	require.NoError(t, err)
	_, err = f.WriteString("testing")
	Close(f)
	require.NoError(t, err)
	empty, err = IsDirEmpty(d)
	require.NoError(t, err)
	require.False(t, empty)

	// Test non existent directory.
	empty, err = IsDirEmpty(path.Join(d, "nonexistent_dir"))
	require.NotNil(t, err)
}

type DomainTestCase struct {
	DomainA string
	DomainB string
	Match   bool
}

func TestCookieDomainMatch(t *testing.T) {
	unittest.SmallTest(t)
	// Test cases borrowed from test_domain_match in
	// https://svn.python.org/projects/python/trunk/Lib/test/test_cookielib.py
	testCases := []DomainTestCase{
		{DomainA: "x.y.com", DomainB: "x.Y.com", Match: true},
		{DomainA: "x.y.com", DomainB: ".Y.com", Match: true},
		{DomainA: "x.y.com", DomainB: "Y.com", Match: false},
		{DomainA: "a.b.c.com", DomainB: ".c.com", Match: true},
		{DomainA: ".c.com", DomainB: "a.b.c.com", Match: false},
		{DomainA: "example.local", DomainB: ".local", Match: true},
		{DomainA: "blah.blah", DomainB: "", Match: false},
		{DomainA: "", DomainB: ".rhubarb.rhubarb", Match: false},
		{DomainA: "", DomainB: "", Match: true},

		{DomainA: "acme.com", DomainB: "acme.com", Match: true},
		{DomainA: "acme.com", DomainB: ".acme.com", Match: false},
		{DomainA: "rhubarb.acme.com", DomainB: ".acme.com", Match: true},
		{DomainA: "www.rhubarb.acme.com", DomainB: ".acme.com", Match: true},
		{DomainA: "y.com", DomainB: "Y.com", Match: true},
		{DomainA: ".y.com", DomainB: "Y.com", Match: false},
		{DomainA: ".y.com", DomainB: ".Y.com", Match: true},
		{DomainA: "x.y.com", DomainB: ".com", Match: true},
		{DomainA: "x.y.com", DomainB: "com", Match: false},
		{DomainA: "x.y.com", DomainB: "m", Match: false},
		{DomainA: "x.y.com", DomainB: ".m", Match: false},
		{DomainA: "x.y.com", DomainB: "", Match: false},
		{DomainA: "x.y.com", DomainB: ".", Match: false},
	}
	for _, tc := range testCases {
		require.Equal(t, tc.Match, CookieDomainMatch(tc.DomainA, tc.DomainB))
	}
}

func TestValidateCommit(t *testing.T) {
	unittest.SmallTest(t)
	tc := map[string]bool{
		"":       false,
		"abc123": false,
		"abcde12345abcde12345abcde12345abcde12345":  true,
		"abcde12345abcde12345abcde12345abcde1234":   false,
		"abcde12345abcde12345abcde12345abcde123456": false,
		"abcde12345abcde12345abcde12345abcde1234g":  false,
		"abcde12345abcde12345abcde12345abcde1234 ":  false,
	}
	for input, expect := range tc {
		require.Equal(t, ValidateCommit(input), expect)
	}
}

func TestPermute(t *testing.T) {
	unittest.SmallTest(t)

	require.Equal(t, [][]int{}, Permute([]int{}))
	require.Equal(t, [][]int{{0}}, Permute([]int{0}))
	require.Equal(t, [][]int{{0, 1}, {1, 0}}, Permute([]int{0, 1}))
	require.Equal(t, [][]int{
		{0, 1, 2},
		{0, 2, 1},
		{1, 0, 2},
		{1, 2, 0},
		{2, 0, 1},
		{2, 1, 0},
	}, Permute([]int{0, 1, 2}))
	require.Equal(t, [][]int{
		{0, 1, 2, 3},
		{0, 1, 3, 2},
		{0, 2, 1, 3},
		{0, 2, 3, 1},
		{0, 3, 1, 2},
		{0, 3, 2, 1},
		{1, 0, 2, 3},
		{1, 0, 3, 2},
		{1, 2, 0, 3},
		{1, 2, 3, 0},
		{1, 3, 0, 2},
		{1, 3, 2, 0},
		{2, 0, 1, 3},
		{2, 0, 3, 1},
		{2, 1, 0, 3},
		{2, 1, 3, 0},
		{2, 3, 0, 1},
		{2, 3, 1, 0},
		{3, 0, 1, 2},
		{3, 0, 2, 1},
		{3, 1, 0, 2},
		{3, 1, 2, 0},
		{3, 2, 0, 1},
		{3, 2, 1, 0},
	}, Permute([]int{0, 1, 2, 3}))
}

func TestPermuteStrings(t *testing.T) {
	unittest.SmallTest(t)

	require.Equal(t, [][]string{}, PermuteStrings([]string{}))
	require.Equal(t, [][]string{{"a"}}, PermuteStrings([]string{"a"}))
	require.Equal(t, [][]string{{"a", "b"}, {"b", "a"}}, PermuteStrings([]string{"a", "b"}))
	require.Equal(t, [][]string{
		{"a", "b", "c"},
		{"a", "c", "b"},
		{"b", "a", "c"},
		{"b", "c", "a"},
		{"c", "a", "b"},
		{"c", "b", "a"},
	}, PermuteStrings([]string{"a", "b", "c"}))
	require.Equal(t, [][]string{
		{"a", "b", "c", "d"},
		{"a", "b", "d", "c"},
		{"a", "c", "b", "d"},
		{"a", "c", "d", "b"},
		{"a", "d", "b", "c"},
		{"a", "d", "c", "b"},
		{"b", "a", "c", "d"},
		{"b", "a", "d", "c"},
		{"b", "c", "a", "d"},
		{"b", "c", "d", "a"},
		{"b", "d", "a", "c"},
		{"b", "d", "c", "a"},
		{"c", "a", "b", "d"},
		{"c", "a", "d", "b"},
		{"c", "b", "a", "d"},
		{"c", "b", "d", "a"},
		{"c", "d", "a", "b"},
		{"c", "d", "b", "a"},
		{"d", "a", "b", "c"},
		{"d", "a", "c", "b"},
		{"d", "b", "a", "c"},
		{"d", "b", "c", "a"},
		{"d", "c", "a", "b"},
		{"d", "c", "b", "a"},
	}, PermuteStrings([]string{"a", "b", "c", "d"}))
}

func TestParseIntSet(t *testing.T) {
	unittest.SmallTest(t)

	test := func(input string, expect []int, expectErr string) {
		res, err := ParseIntSet(input)
		if expectErr != "" {
			require.Contains(t, err.Error(), expectErr)
		} else {
			require.NoError(t, err)
			require.Equal(t, expect, res)
		}
	}
	test("", []int{}, "")
	test("19", []int{19}, "")
	test("1,2,3", []int{1, 2, 3}, "")
	test("1-3", []int{1, 2, 3}, "")
	test("1,2,4-6", []int{1, 2, 4, 5, 6}, "")
	test("a", nil, "parsing \"a\": invalid syntax")
	test(" 4, 6, 9 - 11", nil, "parsing \" 4\": invalid syntax")
	test("4-9-10", nil, "Invalid expression \"4-9-10\"")
	test("9-3", nil, "Cannot have a range whose beginning is greater than its end (9 vs 3)")
	test("1-3,11-13,21-23", []int{1, 2, 3, 11, 12, 13, 21, 22, 23}, "")
	test("-2", nil, "Invalid expression \"-2\"")
	test("2-", nil, "Invalid expression \"2-\"")
}

func TestContainsMap(t *testing.T) {
	unittest.SmallTest(t)
	child := map[string]string{
		"a": "1",
		"b": "2",
	}
	parent := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}
	// Test success
	require.True(t, ContainsMap(parent, child))
	// Test map with itself.
	require.True(t, ContainsMap(parent, parent))
	// Test failure.
	delete(parent, "b")
	require.False(t, ContainsMap(parent, child))
	// Test edge cases.
	require.True(t, ContainsMap(parent, map[string]string{}))
	require.True(t, ContainsMap(map[string]string{}, map[string]string{}))
	require.False(t, ContainsMap(map[string]string{}, map[string]string{"a": "1"}))
}

func TestContainsAnyMap(t *testing.T) {
	unittest.SmallTest(t)
	child1 := map[string]string{
		"a": "1",
		"b": "2",
	}
	child2 := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}
	parent := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}
	// Test success
	require.True(t, ContainsAnyMap(parent, child1, child2))
	// Test map with itself
	require.True(t, ContainsAnyMap(parent, parent))
	// Test failure
	delete(parent, "b")
	require.False(t, ContainsAnyMap(parent, child1, child2))
	require.False(t, ContainsAnyMap(parent, map[string]string{"a": "1", "c": "4"}))
	// Test success with new parent
	require.True(t, ContainsAnyMap(parent, map[string]string{"a": "1", "c": "3"}))
	require.True(t, ContainsAnyMap(parent, child1, parent))
	// Test edge cases.
	require.True(t, ContainsAnyMap(parent, map[string]string{}, child1))
	require.True(t, ContainsAnyMap(parent, map[string]string{}))
	require.True(t, ContainsAnyMap(map[string]string{}, map[string]string{}))
	require.False(t, ContainsAnyMap(map[string]string{}, child1, child2))
}

func TestContainsMapInSliceValues(t *testing.T) {
	unittest.SmallTest(t)
	child := map[string]string{
		"a": "1",
		"b": "2",
	}
	parent := map[string][]string{
		"a": {"1", "2"},
		"b": {"2", "4"},
		"c": {"3"},
	}
	// Test success
	require.True(t, ContainsMapInSliceValues(parent, child))
	child["b"] = "4"
	require.True(t, ContainsMapInSliceValues(parent, child))
	// Test failure.
	child["b"] = "3"
	require.False(t, ContainsMapInSliceValues(parent, child))
	delete(parent, "b")
	require.False(t, ContainsMapInSliceValues(parent, child))
	// Test edge cases.
	require.True(t, ContainsMapInSliceValues(parent, map[string]string{}))
	require.True(t, ContainsMapInSliceValues(map[string][]string{}, map[string]string{}))
	require.False(t, ContainsMapInSliceValues(map[string][]string{}, map[string]string{"a": "1"}))
}

func TestContainsAnyMapInSliceValues(t *testing.T) {
	unittest.SmallTest(t)
	child1 := map[string]string{
		"a": "1",
		"b": "2",
	}
	child2 := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}
	parent := map[string][]string{
		"a": {"1", "4"},
		"b": {"2", "5"},
		"c": {"3"},
	}
	// Test success
	require.True(t, ContainsAnyMapInSliceValues(parent, child1, child2))
	child2["b"] = "5"
	require.True(t, ContainsAnyMapInSliceValues(parent, child1, child2))
	// Test failure
	child1["a"] = "2"
	child2["b"] = "6"
	require.False(t, ContainsAnyMapInSliceValues(parent, child1, child2))
	delete(parent, "b")
	require.False(t, ContainsAnyMapInSliceValues(parent, child1, child2))
	require.False(t, ContainsAnyMapInSliceValues(parent, map[string]string{"a": "1", "c": "4"}))
	require.False(t, ContainsAnyMapInSliceValues(parent, map[string]string{"a": "2"}))
	// Test success with new parent
	require.True(t, ContainsAnyMapInSliceValues(parent, map[string]string{"a": "1", "c": "3"}))
	require.True(t, ContainsAnyMapInSliceValues(parent, map[string]string{"a": "4", "c": "3"}))
	// Test edge cases.
	require.True(t, ContainsAnyMapInSliceValues(parent, map[string]string{}, child1))
	require.True(t, ContainsAnyMapInSliceValues(parent, map[string]string{}))
	require.True(t, ContainsAnyMapInSliceValues(map[string][]string{}, map[string]string{}))
	require.False(t, ContainsAnyMapInSliceValues(map[string][]string{}, child1, child2))
}

func TestTruncate(t *testing.T) {
	unittest.SmallTest(t)
	s := "abcdefghijkl"
	require.Equal(t, "", Truncate(s, 0))
	require.Equal(t, "a", Truncate(s, 1))
	require.Equal(t, "ab", Truncate(s, 2))
	require.Equal(t, "abc", Truncate(s, 3))
	require.Equal(t, "a...", Truncate(s, 4))
	require.Equal(t, "ab...", Truncate(s, 5))
	require.Equal(t, s, Truncate(s, len(s)))
	require.Equal(t, s, Truncate(s, len(s)+1))
}

type fakeWriter struct {
	writeFn func(p []byte) (int, error)
}

func (w *fakeWriter) Write(p []byte) (int, error) {
	return w.writeFn(p)
}

func TestWithGzipWriter(t *testing.T) {
	unittest.SmallTest(t)

	write := func(w io.Writer, msg string) error {
		_, err := w.Write([]byte(msg))
		return err
	}

	// No error.
	require.NoError(t, WithGzipWriter(ioutil.Discard, func(w io.Writer) error {
		return write(w, "hi")
	}))

	// Contained function returns an error.
	expectErr := errors.New("nope")
	require.EqualError(t, WithGzipWriter(ioutil.Discard, func(w io.Writer) error {
		return expectErr
	}), expectErr.Error())

	// Underlying io.Writer returns an error.
	fw := &fakeWriter{
		writeFn: func(p []byte) (int, error) {
			return -1, expectErr
		},
	}
	require.EqualError(t, WithGzipWriter(fw, func(w io.Writer) error {
		return write(w, "hi")
	}), expectErr.Error())

	// Close() returns an error.
	fw.writeFn = func(p []byte) (int, error) {
		// Look for the gzip footer and return an error when we see it.
		// WARNING: this contains a checksum.
		if string(p) == "\xac*\x93\xd8\x02\x00\x00\x00" {
			return -1, expectErr
		}
		return len(p), nil
	}
	require.EqualError(t, WithGzipWriter(fw, func(w io.Writer) error {
		return write(w, "hi")
	}), "Failed to close gzip.Writer: nope")
}

func TestChunkIter(t *testing.T) {
	unittest.SmallTest(t)

	require.Error(t, ChunkIter(10, -1, func(int, int) error {
		require.Fail(t, "shouldn't be called")
		return nil
	}))
	require.Error(t, ChunkIter(10, 0, func(int, int) error {
		require.Fail(t, "shouldn't be called")
		return nil
	}))

	check := func(length, chunkSize int, expect [][]int) {
		var actual [][]int
		require.NoError(t, ChunkIter(length, chunkSize, func(start, end int) error {
			actual = append(actual, []int{start, end})
			return nil
		}))
		assert.Equal(t, expect, actual)
	}

	check(10, 5, [][]int{{0, 5}, {5, 10}})
	check(4, 1, [][]int{{0, 1}, {1, 2}, {2, 3}, {3, 4}})
	check(7, 5, [][]int{{0, 5}, {5, 7}})
}

func TestChunkIterErr(t *testing.T) {
	unittest.SmallTest(t)
	called := 0
	err := ChunkIter(10, 3, func(int, int) error {
		called++
		return fmt.Errorf("oops, robots took over")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oops")
	assert.Equal(t, 1, called, "stop working after error")
}

func TestChunkIterParallel(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	require.Error(t, ChunkIterParallel(ctx, 10, -1, func(context.Context, int, int) error {
		require.Fail(t, "shouldn't be called")
		return nil
	}))
	require.Error(t, ChunkIterParallel(ctx, 10, 0, func(context.Context, int, int) error {
		require.Fail(t, "shouldn't be called")
		return nil
	}))

	check := func(length, chunkSize int, expect []int) {
		actual := make([]int, length)
		require.NoError(t, ChunkIterParallel(ctx, length, chunkSize, func(eCtx context.Context, start, end int) error {
			assert.NoError(t, eCtx.Err())
			for i := start; i < end; i++ {
				actual[i] = start
			}
			return nil
		}))
		assert.Equal(t, expect, actual)
	}

	check(10, 5, []int{0, 0, 0, 0, 0, 5, 5, 5, 5, 5})
	check(4, 1, []int{0, 1, 2, 3})
	check(7, 4, []int{0, 0, 0, 0, 4, 4, 4})
}

func TestChunkIterParallelErr(t *testing.T) {
	unittest.SmallTest(t)
	err := ChunkIterParallel(context.Background(), 10, 3, func(context.Context, int, int) error {
		return fmt.Errorf("oops, robots took over")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oops")

	// If the context is already in an error state, don't call the passed in function, just error.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = ChunkIterParallel(ctx, 10, 3, func(context.Context, int, int) error {
		require.Fail(t, "shouldn't be called because the original context was no good.")
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canceled")
}

func TestRoundUpToPowerOf2(t *testing.T) {
	unittest.SmallTest(t)

	test := func(input, output int32) {
		require.Equal(t, output, RoundUpToPowerOf2(input))
	}
	test(0, 1)
	test(1, 1)
	test(2, 2)
	test(3, 4)
	test(4, 4)
	test(5, 8)
	test(7, 8)
	test(8, 8)
	test(9, 16)
	test(16, 16)
	test(17, 32)
	test(25, 32)
	test(32, 32)
	test(33, 64)
	test(50, 64)
	test(64, 64)
	for i := 64; i < (1 << 31); i = i << 1 {
		test(int32(i-1), int32(i))
		test(int32(i), int32(i))
	}
}

func TestSSliceCmp(t *testing.T) {
	unittest.SmallTest(t)

	// "Equal" slices.
	testEq := func(a, b []string) {
		require.Equal(t, 0, SSliceCmp(a, b))
		require.Equal(t, 0, SSliceCmp(b, a))
	}
	testEq(nil, nil)
	testEq(nil, []string{})
	testEq([]string{}, []string{})
	testEq([]string{"item"}, []string{"item"})
	testEq([]string{"a", "b", "c"}, []string{"a", "b", "c"})

	// a > b
	testGt := func(a, b []string) {
		require.Equal(t, 1, SSliceCmp(a, b))
		require.Equal(t, -1, SSliceCmp(b, a))
	}
	testGt([]string{"a"}, nil)
	testGt([]string{"a"}, []string{})
	testGt([]string{"b"}, []string{"a"})
	testGt([]string{"a", "b", "d"}, []string{"a", "b", "c"})
	testGt([]string{"a", "b", "c", "d"}, []string{"a", "b", "c"})
	testGt([]string{"a", "b", "d"}, []string{"a", "b", "c", "d"})
	testGt([]string{"a", "c", "b"}, []string{"a", "b", "c"})
}

func TestPowerSet(t *testing.T) {
	unittest.SmallTest(t)
	test := func(inp int, expect [][]int) {
		assertdeep.Equal(t, expect, PowerSet(inp))
	}
	test(0, [][]int{{}})
	test(1, [][]int{{}, {0}})
	test(2, [][]int{{}, {0}, {1}, {0, 1}})
	test(3, [][]int{{}, {0}, {1}, {0, 1}, {2}, {0, 2}, {1, 2}, {0, 1, 2}})
}

func TestSSliceDedup(t *testing.T) {
	unittest.SmallTest(t)

	require.Equal(t, []string{}, SSliceDedup([]string{}))
	require.Equal(t, []string{"foo"}, SSliceDedup([]string{"foo"}))
	require.Equal(t, []string{"foo"}, SSliceDedup([]string{"foo", "foo"}))
	require.Equal(t, []string{"foo", "bar"}, SSliceDedup([]string{"foo", "bar"}))
	require.Equal(t, []string{"foo", "bar"}, SSliceDedup([]string{"foo", "foo", "bar"}))
	require.Equal(t, []string{"foo", "bar"}, SSliceDedup([]string{"foo", "bar", "bar"}))
	require.Equal(t, []string{"foo", "bar"}, SSliceDedup([]string{"foo", "foo", "bar", "bar"}))
	require.Equal(t, []string{"foo", "baz", "bar"}, SSliceDedup([]string{"foo", "foo", "baz", "bar", "bar"}))
	require.Equal(t, []string{"foo", "baz", "bar"}, SSliceDedup([]string{"foo", "foo", "baz", "bar", "bar", "baz"}))
	require.Equal(t, []string{"foo", "bar", "baz"}, SSliceDedup([]string{"foo", "foo", "bar", "baz", "bar", "baz"}))
}
