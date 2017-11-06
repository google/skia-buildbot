package repeat_cleanup

import (
	"context"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func TestQuit(t *testing.T) {
	testutils.SmallTest(t)

	// Verify that both the tick and cleanup functions get called as
	// expected.
	Init(context.Background())
	count := 0
	cleanup := false
	Repeat(time.Millisecond, func() {
		count++
	}, func() {
		cleanup = true
	})
	time.Sleep(10 * time.Millisecond)
	Cancel()
	assert.True(t, count >= 4)
	assert.True(t, cleanup)

	// Multiple registered funcs.
	Init(context.Background())
	n := 5
	counts := make([]int, 0, n)
	cleanups := make([]bool, 0, n)
	for i := 0; i < n; i++ {
		idx := i
		counts = append(counts, 0)
		cleanups = append(cleanups, false)
		Repeat(time.Millisecond, func() {
			counts[idx]++
		}, func() {
			cleanups[idx] = true
		})
	}
	time.Sleep(10 * time.Millisecond)
	Cancel()
	for i := 0; i < n; i++ {
		assert.True(t, counts[i] >= 4)
		assert.True(t, cleanups[i])
	}
}
