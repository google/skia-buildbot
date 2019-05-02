package types

import (
	"testing"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
)

func TestGoldenTrace(t *testing.T) {
	testutils.SmallTest(t)
	N := 5
	// Test NewGoldenTrace.
	g := NewGoldenTraceN(N)
	if got, want := g.Len(), N; got != want {
		t.Errorf("Wrong Digests Size: Got %v Want %v", got, want)
	}
	if got, want := len(g.Keys), 0; got != want {
		t.Errorf("Wrong Keys initial size: Got %v Want %v", got, want)
	}

	g.Digests[0] = "a digest"

	if got, want := g.IsMissing(1), true; got != want {
		t.Errorf("All values should start as missing: Got %v Want %v", got, want)
	}
	if got, want := g.IsMissing(0), false; got != want {
		t.Errorf("Set values shouldn't be missing: Got %v Want %v", got, want)
	}

	// Test Merge.
	M := 7
	gm := NewGoldenTraceN(M)
	gm.Digests[1] = "another digest"
	g2 := g.Merge(gm)
	if got, want := g2.Len(), N+M; got != want {
		t.Errorf("Merge length wrong: Got %v Want %v", got, want)
	}
	if got, want := g2.(*GoldenTrace).Digests[0], "a digest"; got != want {
		t.Errorf("Digest not copied correctly: Got %v Want %v", got, want)
	}
	if got, want := g2.(*GoldenTrace).Digests[6], "another digest"; got != want {
		t.Errorf("Digest not copied correctly: Got %v Want %v", got, want)
	}

	// Test Grow.
	g = NewGoldenTraceN(N)
	g.Digests[0] = "foo"
	g.Grow(2*N, tiling.FILL_BEFORE)
	if got, want := g.Digests[N], "foo"; got != want {
		t.Errorf("Grow didn't FILL_BEFORE correctly: Got %v Want %v", got, want)
	}

	g = NewGoldenTraceN(N)
	g.Digests[0] = "foo"
	g.Grow(2*N, tiling.FILL_AFTER)
	if got, want := g.Digests[0], "foo"; got != want {
		t.Errorf("Grow didn't FILL_AFTER correctly: Got %v Want %v", got, want)
	}

	// Test Trim
	g = NewGoldenTraceN(N)
	g.Digests[1] = "foo"
	if err := g.Trim(1, 3); err != nil {
		t.Fatalf("Trim Failed: %s", err)
	}
	if got, want := g.Digests[0], "foo"; got != want {
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

func TestSetAt(t *testing.T) {
	testutils.SmallTest(t)
	testCases := []struct {
		want string
	}{
		{
			want: "",
		},
		{
			want: "abcd",
		},
		{
			want: MISSING_DIGEST,
		},
	}
	tr := NewGoldenTraceN(len(testCases))
	for i, tc := range testCases {
		if err := tr.SetAt(i, []byte(tc.want)); err != nil {
			t.Fatalf("SetAt(%d, %#v) failed: %s", i, []byte(tc.want), err)
		}
	}
	for i, tc := range testCases {
		if got, want := tr.Digests[i], tc.want; got != want {
			t.Errorf("SetAt(%d, %#v)failed: Got %s Want %s", i, []byte(tc.want), got, want)
		}
	}
}
