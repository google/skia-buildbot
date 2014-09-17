package types

import (
	"testing"

	"skia.googlesource.com/buildbot.git/perf/go/config"
)

func TestMerge(t *testing.T) {
	t1 := NewTile()
	t1.Scale = 1
	t1.TileIndex = 20
	t1.Commits[1].Hash = "hash1"

	t2 := NewTile()
	t2.Scale = 1
	t2.TileIndex = 21
	t2.Commits[1].Hash = "hash33"
	t2.Commits[2].Hash = "hash34"

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
	merged := Merge(t1, t2)
	if got, want := len(merged.Traces["foo"].(*PerfTrace).Values), 2*config.TILE_SIZE; got != want {
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
	if got, want := len(merged.Traces["foo"].(*PerfTrace).Values), 2*config.TILE_SIZE; got != want {
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
		{128, 0.3},
		{129, 0.4},
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
		{128, 0.5},
		{129, 0.6},
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
	g.Grow(2*N, FILL_BEFORE)
	if got, want := g.Values[N], 3.1; got != want {
		t.Errorf("Grow didn't FILL_BEFORE correctly: Got %v Want %v", got, want)
	}

	g = NewPerfTraceN(N)
	g.Values[0] = 1.3
	g.Grow(2*N, FILL_AFTER)
	if got, want := g.Values[0], 1.3; got != want {
		t.Errorf("Grow didn't FILL_AFTER correctly: Got %v Want %v", got, want)
	}
}
