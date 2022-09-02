package throttler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/skcq/go/config"
)

var (
	epochTestTime = int64(1598467386)
)

func TestThrottler(t *testing.T) {

	testRepo1 := "test-repo1"
	testRepo2 := "test-repo2"
	timeNowFunc = func() time.Time {
		return time.Unix(epochTestTime, 0).UTC()
	}
	throttler := NewThrottler()

	// Throttler config for 2 commits every 100 seconds.
	throttlerCfgRepo1 := &config.ThrottlerCfg{
		MaxBurst:       2,
		BurstDelaySecs: 100,
	}

	// There should be no throttling in both repos with no commits stored.
	require.False(t, throttler.Throttle(testRepo1, time.Unix(epochTestTime, 0).UTC()))
	require.False(t, throttler.Throttle(testRepo2, time.Unix(epochTestTime, 0).UTC()))

	// Add 1 commit to repo1 made 50s ago.
	throttler.UpdateThrottler(testRepo1, time.Unix(epochTestTime-50, 0).UTC(), throttlerCfgRepo1)
	// repo1 should have no throttling.
	require.False(t, throttler.Throttle(testRepo1, time.Unix(epochTestTime, 0).UTC()))

	// Add 2nd commit to repo1 made 20s ago.
	throttler.UpdateThrottler(testRepo1, time.Unix(epochTestTime-20, 0).UTC(), throttlerCfgRepo1)
	// repo1 should be throttled because there have been 2 commits in the last 100s.
	require.True(t, throttler.Throttle(testRepo1, time.Unix(epochTestTime, 0).UTC()))
	// repo2 should not be throttled because all commits have been in repo1.
	require.False(t, throttler.Throttle(testRepo2, time.Unix(epochTestTime, 0).UTC()))

	// Update the current time to move beyond the 100s of commit1.
	timeNowFunc = func() time.Time {
		return time.Unix(epochTestTime+51, 0).UTC()
	}
	// A new commit in repo1 should no longer be throttled.
	require.False(t, throttler.Throttle(testRepo1, time.Unix(epochTestTime, 0).UTC()))
	// repo2 should not be throttled because all commits have been in repo1.
	require.False(t, throttler.Throttle(testRepo2, time.Unix(epochTestTime, 0).UTC()))
}
