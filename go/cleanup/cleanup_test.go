package cleanup

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCleanup(t *testing.T) {

	interval := 200 * time.Millisecond

	// Verify that both the tick and cleanup functions get called as
	// expected.
	count := 0
	cleanup := false
	Repeat(interval, func(_ context.Context) {
		count++
		require.False(t, cleanup)
	}, func() {
		require.False(t, cleanup)
		cleanup = true
	})
	time.Sleep(10 * interval)
	Cleanup()
	require.True(t, count >= 4)
	require.True(t, cleanup)

	// Multiple registered funcs.
	reset()

	n := 5
	counts := make([]int, 0, n)
	cleanups := make([]bool, 0, n)
	for i := 0; i < n; i++ {
		counts = append(counts, 0)
		cleanups = append(cleanups, false)
	}
	for i := 0; i < n; i++ {
		idx := i
		Repeat(interval, func(_ context.Context) {
			counts[idx]++
			require.False(t, cleanups[idx])
		}, func() {
			require.False(t, cleanups[idx])
			cleanups[idx] = true
		})
	}
	time.Sleep(10 * interval)
	Cleanup()
	for i := 0; i < n; i++ {
		require.True(t, counts[i] >= 4)
		require.True(t, cleanups[i])
	}
}
