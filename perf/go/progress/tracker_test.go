package progress

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

var testDate = time.Date(2020, 04, 01, 0, 0, 0, 0, time.UTC)

func TestTracker_Add_ProgressAppearsInCacheAndMetrics(t *testing.T) {
	unittest.SmallTest(t)

	tr, err := NewTracker("/")
	require.NoError(t, err)
	p := New()
	tr.Add(p)

	assert.Equal(t, 1, tr.cache.Len())

	assert.Equal(t, int64(0), tr.numEntriesInCache.Get())
	tr.singleStep()
	assert.Equal(t, int64(1), tr.numEntriesInCache.Get())
}

func TestTracker_ProgressIsFinished_ProgressStillAppearsInCacheAndMetrics(t *testing.T) {
	unittest.SmallTest(t)

	tr, err := NewTracker("/")
	require.NoError(t, err)
	p := New()
	tr.Add(p)
	p.Finished(nil)

	tr.singleStep()

	// Still there because it hasn't passed the expiration date.
	assert.Equal(t, 1, tr.cache.Len())
	assert.Equal(t, int64(1), tr.numEntriesInCache.Get())
}

func TestTracker_TimeAdvancesPastExpirationOfFinishedProgress_ProgressNoLongerAppearsInCacheAndMetrics(t *testing.T) {
	unittest.SmallTest(t)

	tr, err := NewTracker("/")
	require.NoError(t, err)
	p := New()
	p.Finished(nil)
	tr.Add(p)

	timeNow = func() time.Time {
		return testDate
	}

	// This pass will mark the time the Progress finished in the cache entry.
	tr.singleStep()

	timeNow = func() time.Time {
		return testDate.Add(2 * cacheDuration)
	}

	// This pass will evict the Progress from the cache.
	tr.singleStep()

	assert.Equal(t, 0, tr.cache.Len())
	assert.Equal(t, int64(0), tr.numEntriesInCache.Get())
}
