package attic

import (
	"testing"

	"skia.googlesource.com/buildbot.git/perf/go/types"
)

func TestGoldenTrace(t *testing.T) {
	N := 5
	// Test NewGoldenTrace.
	g := NewGoldenTraceN(N)
	if got, want := g.Len(), N; got != want {
		t.Errorf("Wrong Values Size: Got %v Want %v", got, want)
	}
	if got, want := len(g.Params_), 0; got != want {
		t.Errorf("Wrong Params_ initial size: Got %v Want %v", got, want)
	}

	g.Values[0] = "a digest"

	if got, want := g.IsMissing(1), true; got != want {
		t.Errorf("All values should start as missing: Got %v Want %v", got, want)
	}
	if got, want := g.IsMissing(0), false; got != want {
		t.Errorf("Set values shouldn't be missing: Got %v Want %v", got, want)
	}

	// Test Merge.
	M := 7
	gm := NewGoldenTraceN(M)
	gm.Values[1] = "another digest"
	g2 := g.Merge(gm)
	if got, want := g2.Len(), N+M; got != want {
		t.Errorf("Merge length wrong: Got %v Want %v", got, want)
	}
	if got, want := g2.(*GoldenTrace).Values[0], Digest("a digest"); got != want {
		t.Errorf("Digest not copied correctly: Got %v Want %v", got, want)
	}
	if got, want := g2.(*GoldenTrace).Values[6], Digest("another digest"); got != want {
		t.Errorf("Digest not copied correctly: Got %v Want %v", got, want)
	}

	// Test Grow.
	g = NewGoldenTraceN(N)
	g.Values[0] = "foo"
	g.Grow(2*N, types.FILL_BEFORE)
	if got, want := g.Values[N], Digest("foo"); got != want {
		t.Errorf("Grow didn't FILL_BEFORE correctly: Got %v Want %v", got, want)
	}

	g = NewGoldenTraceN(N)
	g.Values[0] = "foo"
	g.Grow(2*N, types.FILL_AFTER)
	if got, want := g.Values[0], Digest("foo"); got != want {
		t.Errorf("Grow didn't FILL_AFTER correctly: Got %v Want %v", got, want)
	}
}
