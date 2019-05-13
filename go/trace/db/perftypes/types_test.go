package perftypes

import (
	"math"
	"net/url"
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
)

func TestMerge(t *testing.T) {
	unittest.SmallTest(t)
	t1 := tiling.NewTile()
	t1.Scale = 1
	t1.TileIndex = 20
	t1.Commits[1].Hash = "hash1"

	t2 := tiling.NewTile()
	t2.Scale = 1
	t2.TileIndex = 21
	t2.Commits[1].Hash = "hash33"
	t2.Commits[2].Hash = "hash34"

	t3 := tiling.NewTile()
	t2.Scale = 1
	t2.TileIndex = 22
	t2.Commits[1].Hash = "hash43"
	t2.Commits[2].Hash = "hash44"

	// Create a Trace that exists in both tile1 and tile2.
	tr := NewPerfTrace()
	tr.Params_["p1"] = "v1"
	tr.Params_["p2"] = "v2"
	tr.Values[0] = 0.1
	tr.Values[1] = 0.2

	t1.Traces["foo"] = tr

	tr = NewPerfTrace()
	tr.Params_["p1"] = "v1"
	tr.Params_["p2"] = "v2"
	tr.Params_["p5"] = "5"
	tr.Values[0] = 0.3
	tr.Values[1] = 0.4

	t2.Traces["foo"] = tr

	// Add a trace that only appears in tile2.
	tr = NewPerfTrace()
	tr.Params_["p1"] = "v1"
	tr.Params_["p3"] = "v3"
	tr.Values[0] = 0.5
	tr.Values[1] = 0.6

	t2.Traces["bar"] = tr

	// Merge the two tiles.
	merged := tiling.Merge(tiling.Merge(t1, t2), t3)
	if got, want := len(merged.Traces["foo"].(*PerfTrace).Values), 3*tiling.TILE_SIZE; got != want {
		t.Errorf("Wrong config.TILE_SIZE: Got %v Want %v", got, want)
	}

	if got, want := merged.Scale, 1; got != want {
		t.Errorf("Wrong scale: Got %v Want %v", got, want)
	}
	if got, want := merged.TileIndex, t1.TileIndex; got != want {
		t.Errorf("TileIndex is wrong: Got %v Want %v", got, want)
	}
	if got, want := len(merged.Traces), 2; got != want {
		t.Errorf("Number of traces: Got %v Want %v", got, want)
	}
	if got, want := len(merged.Traces["foo"].(*PerfTrace).Values), 3*tiling.TILE_SIZE; got != want {
		t.Errorf("Number of values: Got %v Want %v", got, want)
	}
	if got, want := len(merged.ParamSet), 4; got != want {
		t.Errorf("ParamSet length: Got %v Want %v", got, want)
	}
	if _, ok := merged.ParamSet["p5"]; !ok {
		t.Errorf("Merged tile missing 'p5' param.")
	}

	// Test the "foo" trace.
	tr = merged.Traces["foo"].(*PerfTrace)
	testCases := []struct {
		N int
		V float64
	}{
		{127, 1e100},
		{50, 0.3},
		{51, 0.4},
		{130, 1e100},
		{0, 0.1},
		{1, 0.2},
		{2, 1e100},
	}
	for _, tc := range testCases {
		if got, want := tr.Values[tc.N], tc.V; got != want {
			t.Errorf("Error copying trace values: Got %v Want %v at %d", got, want, tc.N)
		}
	}
	if got, want := tr.Params()["p1"], "v1"; got != want {
		t.Errorf("Wrong params for trace: Got %v Want %v", got, want)
	}

	// Test the "bar" trace.
	tr = merged.Traces["bar"].(*PerfTrace)
	testCases = []struct {
		N int
		V float64
	}{
		{127, 1e100},
		{50, 0.5},
		{51, 0.6},
		{130, 1e100},
	}
	for _, tc := range testCases {
		if got, want := tr.Values[tc.N], tc.V; got != want {
			t.Errorf("Error copying trace values: Got %v Want %v at %d", got, want, tc.N)
		}
	}
	if got, want := tr.Params()["p3"], "v3"; got != want {
		t.Errorf("Wrong params for trace: Got %v Want %v", got, want)
	}
}

func TestPerfTrace(t *testing.T) {
	unittest.SmallTest(t)
	N := 5
	// Test NewPerfTrace.
	g := NewPerfTraceN(N)
	if got, want := g.Len(), N; got != want {
		t.Errorf("Wrong Values Size: Got %v Want %v", got, want)
	}
	if got, want := len(g.Params_), 0; got != want {
		t.Errorf("Wrong Params_ initial size: Got %v Want %v", got, want)
	}

	g.Values[0] = 1.1

	if got, want := g.IsMissing(1), true; got != want {
		t.Errorf("All values should start as missing: Got %v Want %v", got, want)
	}
	if got, want := g.IsMissing(0), false; got != want {
		t.Errorf("Set values shouldn't be missing: Got %v Want %v", got, want)
	}

	// Test Merge.
	M := 7
	gm := NewPerfTraceN(M)
	gm.Values[1] = 1.2
	g2 := g.Merge(gm)
	if got, want := g2.Len(), N+M; got != want {
		t.Errorf("Merge length wrong: Got %v Want %v", got, want)
	}
	if got, want := g2.(*PerfTrace).Values[0], 1.1; got != want {
		t.Errorf("Digest not copied correctly: Got %v Want %v", got, want)
	}
	if got, want := g2.(*PerfTrace).Values[6], 1.2; got != want {
		t.Errorf("Digest not copied correctly: Got %v Want %v", got, want)
	}

	// Test Grow.
	g = NewPerfTraceN(N)
	g.Values[0] = 3.1
	g.Grow(2*N, tiling.FILL_BEFORE)
	if got, want := g.Values[N], 3.1; got != want {
		t.Errorf("Grow didn't FILL_BEFORE correctly: Got %v Want %v", got, want)
	}

	g = NewPerfTraceN(N)
	g.Values[0] = 1.3
	g.Grow(2*N, tiling.FILL_AFTER)
	if got, want := g.Values[0], 1.3; got != want {
		t.Errorf("Grow didn't FILL_AFTER correctly: Got %v Want %v", got, want)
	}

	// Test Trim
	g = NewPerfTraceN(N)
	g.Values[1] = 1.3
	if err := g.Trim(1, 3); err != nil {
		t.Fatalf("Trim Failed: %s", err)
	}
	if got, want := g.Values[0], 1.3; got != want {
		t.Errorf("Trim didn't copy correctly: Got %v Want %v", got, want)
	}
	if got, want := g.Len(), 2; got != want {
		t.Errorf("Trim wrong length: Got %v Want %v", got, want)
	}

	if err := g.Trim(-1, 1); err == nil {
		t.Error("Trim failed to error.")
	}
	if err := g.Trim(1, 3); err == nil {
		t.Error("Trim failed to error.")
	}
	if err := g.Trim(2, 1); err == nil {
		t.Error("Trim failed to error.")
	}

	if err := g.Trim(1, 1); err != nil {
		t.Fatalf("Trim Failed: %s", err)
	}
	if got, want := g.Len(), 0; got != want {
		t.Errorf("Trim wrong length: Got %v Want %v", got, want)
	}
}

func TestTileTrim(t *testing.T) {
	unittest.SmallTest(t)
	t1 := tiling.NewTile()
	t1.Scale = 1
	t1.TileIndex = 1
	t1.Commits[len(t1.Commits)-2].Hash = "hash0"
	t1.Commits[len(t1.Commits)-1].Hash = "hash1"

	tr := NewPerfTrace()
	tr.Values[0] = 0.5
	tr.Values[1] = 0.6
	tr.Values[2] = 0.7

	t1.Traces["bar"] = tr

	t2, err := t1.Trim(len(t1.Commits)-2, len(t1.Commits))
	if err != nil {
		t.Errorf("Failed to trim: %s", err)
	}
	if got, want := len(t2.Commits), 2; got != want {
		t.Errorf("Trimmed tile length wrong: Got %v Want %v", got, want)
	}
	if got, want := len(t2.Traces["bar"].(*PerfTrace).Values), 2; got != want {
		t.Errorf("Failed to trim traces: Got %v Want %v", got, want)
	}
	if got, want := t2.Commits[0].Hash, "hash0"; got != want {
		t.Errorf("Failed to copy commit over: Got %v Want %v", got, want)
	}
	if got, want := t2.Commits[1].Hash, "hash1"; got != want {
		t.Errorf("Failed to copy commit over: Got %v Want %v", got, want)
	}

	// Test error conditions.
	t2, err = t1.Trim(1, 0)
	if err == nil {
		t.Errorf("Failed to raise error on Trim(1, 0).")
	}
	t2, err = t1.Trim(-1, 1)
	if err == nil {
		t.Errorf("Failed to raise error on Trim(-1, 1).")
	}
	t2, err = t1.Trim(-1, 1)
	if err == nil {
		t.Errorf("Failed to raise error on Trim(-1, 1).")
	}
	t2, err = t1.Trim(0, tiling.TILE_SIZE+1)
	if err == nil {
		t.Errorf("Failed to raise error on Trim(0, config.TILE_SIZE+1).")
	}
}

func TestMatchesWithIgnore(t *testing.T) {
	unittest.SmallTest(t)
	tr := NewPerfTrace()
	tr.Params_["p1"] = "v1"
	tr.Params_["p2"] = "v2"

	testCases := []struct {
		q      url.Values
		ignore []url.Values
		want   bool
	}{
		// Empty case.
		{
			q:      url.Values{},
			ignore: []url.Values{},
			want:   true,
		},
		// Only ignore.
		{
			q: url.Values{},
			ignore: []url.Values{
				{},
				{"p2": []string{"v2"}},
			},
			want: false,
		},
		// Not match query.
		{
			q:      url.Values{"p1": []string{"bad"}},
			ignore: []url.Values{},
			want:   false,
		},
		// Match query, fail to match ignore.
		{
			q: url.Values{"p1": []string{"v1"}},
			ignore: []url.Values{
				{"p1": []string{"bad"}},
			},
			want: true,
		},
		// Match query, and match ignore.
		{
			q: url.Values{"p1": []string{"v1"}},
			ignore: []url.Values{
				{"p1": []string{"v1"}},
			},
			want: false,
		},
		// Match query, and match one of many ignores.
		{
			q: url.Values{"p1": []string{"v1"}},
			ignore: []url.Values{
				{},
				{"p1": []string{"v1"}},
			},
			want: false,
		},
	}

	for _, tc := range testCases {
		if got, want := tiling.MatchesWithIgnores(tr, tc.q, tc.ignore...), tc.want; got != want {
			t.Errorf("MatchesWithIgnores(%v, %v, %v): Got %v Want %v", tr, tc.q, tc.ignore, got, want)
		}
	}
}

func TestSetAt(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		want float64
	}{
		{
			want: 1.0,
		},
		{
			want: MISSING_DATA_SENTINEL,
		},
		{
			want: math.MaxFloat64,
		},
		{
			want: math.SmallestNonzeroFloat64,
		},
	}
	tr := NewPerfTraceN(len(testCases))
	for i, tc := range testCases {
		if err := tr.SetAt(i, BytesFromFloat64(tc.want)); err != nil {
			t.Fatalf("SetAt(%d, %#v) failed: %s", i, BytesFromFloat64(tc.want), err)
		}
	}
	for i, tc := range testCases {
		if got, want := tr.Values[i], tc.want; got != want {
			t.Errorf("SetAt(%d, %#v)failed: Got %f Want %f", i, BytesFromFloat64(tc.want), got, want)
		}
	}
}
