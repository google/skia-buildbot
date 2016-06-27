package util

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"testing"
	"time"

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

func TestIsNil(t *testing.T) {
	assert.True(t, IsNil(nil))
	assert.False(t, IsNil(false))
	assert.False(t, IsNil(0))
	assert.False(t, IsNil(""))
	assert.False(t, IsNil([0]int{}))
	type Empty struct{}
	assert.False(t, IsNil(Empty{}))
	assert.True(t, IsNil(chan interface{}(nil)))
	assert.False(t, IsNil(make(chan interface{})))
	var f func()
	assert.True(t, IsNil(f))
	assert.False(t, IsNil(func() {}))
	assert.True(t, IsNil(map[bool]bool(nil)))
	assert.False(t, IsNil(make(map[bool]bool)))
	assert.True(t, IsNil([]int(nil)))
	assert.False(t, IsNil([][]int{nil}))
	assert.True(t, IsNil((*int)(nil)))
	var i int
	assert.False(t, IsNil(&i))
	var pi *int
	assert.True(t, IsNil(pi))
	assert.True(t, IsNil(&pi))
	var ppi **int
	assert.True(t, IsNil(&ppi))
	var c chan interface{}
	assert.True(t, IsNil(&c))
	var w io.Writer
	assert.True(t, IsNil(w))
	w = (*bytes.Buffer)(nil)
	assert.True(t, IsNil(w))
	w = &bytes.Buffer{}
	assert.False(t, IsNil(w))
	assert.False(t, IsNil(&w))
	var ii interface{}
	ii = &pi
	assert.True(t, IsNil(ii))
}

func TestUnixFloatToTime(t *testing.T) {
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
		assert.Equal(t, tc.out, UnixFloatToTime(tc.in))
	}
}

func TestTimeToUnixFloat(t *testing.T) {
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
		assert.Equal(t, tc.out, TimeToUnixFloat(tc.in))
	}
}

func TestTimeConversion(t *testing.T) {
	cases := []float64{
		0.0,
		1.0,
		1414703190.0,
		1414703190.292000,
	}
	for _, tc := range cases {
		assert.Equal(t, tc, TimeToUnixFloat(UnixFloatToTime(tc)))
	}
}

func TestMD5Hash(t *testing.T) {
	m_1 := map[string]string{"key1": "val1"}
	m_2 := map[string]string{}
	var m_3 map[string]string = nil
	m_4 := map[string]string{
		"k3": "v1",
		"k2": "v2",
		"k1": "v3",
		"k4": "v4",
	}

	h_1, err := MD5Params(m_1)
	assert.NoError(t, err)

	h_2, err := MD5Params(m_2)
	assert.NoError(t, err)

	h_3, err := MD5Params(m_3)
	assert.NoError(t, err)
	assert.Equal(t, 32, len(h_1))
	assert.Equal(t, 32, len(h_2))
	assert.Equal(t, 32, len(h_3))
	assert.NotEqual(t, h_1, h_2)
	assert.NotEqual(t, h_1, h_3)
	assert.Equal(t, h_2, h_3)

	// Ensure that we get the same hash every time.
	h_4, err := MD5Params(m_4)
	assert.NoError(t, err)
	for i := 0; i < 100; i++ {
		h, err := MD5Params(m_4)
		assert.NoError(t, err)
		assert.Equal(t, h_4, h)
	}
	h, err := MD5Params(map[string]string{
		"k4": "v4",
		"k2": "v2",
		"k3": "v1",
		"k1": "v3",
	})
	assert.NoError(t, err)
	assert.Equal(t, h_4, h)
}

func TestBugsFromCommitMsg(t *testing.T) {
	cases := []struct {
		in  string
		out map[string][]string
	}{
		{
			in: "BUG=skia:1234",
			out: map[string][]string{
				"skia": []string{"1234"},
			},
		},
		{
			in: "BUG=skia:1234,skia:4567",
			out: map[string][]string{
				"skia": []string{"1234", "4567"},
			},
		},
		{
			in: "BUG=skia:1234,skia:4567,skia:8901",
			out: map[string][]string{
				"skia": []string{"1234", "4567", "8901"},
			},
		},
		{
			in: "BUG=1234",
			out: map[string][]string{
				"chromium": []string{"1234"},
			},
		},
		{
			in: "BUG=skia:1234, 456",
			out: map[string][]string{
				"chromium": []string{"456"},
				"skia":     []string{"1234"},
			},
		},
		{
			in: `Lorem ipsum dolor sit amet, consectetur adipiscing elit.

Quisque feugiat, mi et tristique dignissim, sapien risus tristique mi, non dignissim nibh erat ut ex.

BUG=1234, skia:5678
`,
			out: map[string][]string{
				"chromium": []string{"1234"},
				"skia":     []string{"5678"},
			},
		},
	}
	for _, tc := range cases {
		result := BugsFromCommitMsg(tc.in)
		assert.Equal(t, tc.out, result)
	}
}

func TestIsDirEmpty(t *testing.T) {
	d, err := ioutil.TempDir(os.TempDir(), "test_empty")
	assert.NoError(t, err)
	defer RemoveAll(d)

	// Directory is initially empty.
	empty, err := IsDirEmpty(d)
	assert.NoError(t, err)
	assert.True(t, empty)

	// Add a file in the directory.
	f, err := ioutil.TempFile(d, "test_file")
	assert.NoError(t, err)
	_, err = f.WriteString("testing")
	Close(f)
	assert.NoError(t, err)
	empty, err = IsDirEmpty(d)
	assert.NoError(t, err)
	assert.False(t, empty)

	// Test non existent directory.
	empty, err = IsDirEmpty(path.Join(d, "nonexistent_dir"))
	assert.NotNil(t, err)
}
