package types

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/tiling"
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
	if got, want := g2.(*GoldenTrace).Values[0], "a digest"; got != want {
		t.Errorf("Digest not copied correctly: Got %v Want %v", got, want)
	}
	if got, want := g2.(*GoldenTrace).Values[6], "another digest"; got != want {
		t.Errorf("Digest not copied correctly: Got %v Want %v", got, want)
	}

	// Test Grow.
	g = NewGoldenTraceN(N)
	g.Values[0] = "foo"
	g.Grow(2*N, tiling.FILL_BEFORE)
	if got, want := g.Values[N], "foo"; got != want {
		t.Errorf("Grow didn't FILL_BEFORE correctly: Got %v Want %v", got, want)
	}

	g = NewGoldenTraceN(N)
	g.Values[0] = "foo"
	g.Grow(2*N, tiling.FILL_AFTER)
	if got, want := g.Values[0], "foo"; got != want {
		t.Errorf("Grow didn't FILL_AFTER correctly: Got %v Want %v", got, want)
	}

	// Test Trim
	g = NewGoldenTraceN(N)
	g.Values[1] = "foo"
	if err := g.Trim(1, 3); err != nil {
		t.Fatalf("Trim Failed: %s", err)
	}
	if got, want := g.Values[0], "foo"; got != want {
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

func TestTryBotResults(t *testing.T) {
	K_1, K_2, K_3 := "key1", "key2", "key3"
	V_1, V_2, V_3 := "val1", "val2", "val3"
	T_1, T_2, T_3 := "test1", "test2", "test3"
	PARAMS_1 := map[string]string{PRIMARY_KEY_FIELD: T_1}
	PARAMS_2 := map[string]string{PRIMARY_KEY_FIELD: T_2}
	PARAMS_3 := map[string]string{PRIMARY_KEY_FIELD: T_3}

	tbResult := NewTryBotResults()
	now := time.Now().Unix()
	tbResult.Update(K_1, T_1, V_1, PARAMS_1, now)
	tbResult.Update(K_2, T_2, V_2, PARAMS_2, now)
	tbResult.Update(K_3, T_3, V_3, PARAMS_3, now)

	assert.Equal(t, &TBResult{Test: T_1, Digest: V_1, Params: PARAMS_1, TS: now}, tbResult[K_1])
	assert.Equal(t, &TBResult{Test: T_2, Digest: V_2, Params: PARAMS_2, TS: now}, tbResult[K_2])
	assert.Equal(t, &TBResult{Test: T_3, Digest: V_3, Params: PARAMS_3, TS: now}, tbResult[K_3])

	time.Sleep(time.Second)
	newNow := time.Now().Unix()
	tbResult.Update(K_3, T_3, V_2, PARAMS_3, newNow)
	assert.Equal(t, &TBResult{Test: T_1, Digest: V_1, Params: PARAMS_1, TS: now}, tbResult[K_1])
	assert.Equal(t, &TBResult{Test: T_2, Digest: V_2, Params: PARAMS_2, TS: now}, tbResult[K_2])
	assert.Equal(t, &TBResult{Test: T_3, Digest: V_2, Params: PARAMS_3, TS: newNow}, tbResult[K_3])
}
