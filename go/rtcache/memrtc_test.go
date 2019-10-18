package rtcache

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

// TestMemReadThroughCacheSunnyDay checks that we cache the values after reading them
// from the passed in worker function
func TestMemReadThroughCacheSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	const alpha = "alpha"
	const beta = "beta"

	called := map[string]int{}

	workerFn := func(ctx context.Context, id string) (interface{}, error) {
		assert.NotNil(t, ctx)
		called[id]++
		return id + id, nil
	}

	rtc, err := New(workerFn, 10, 10)
	require.NoError(t, err)

	assert.Empty(t, rtc.Keys())

	v, err := rtc.Get(context.Background(), alpha)
	require.NoError(t, err)
	assert.Equal(t, "alphaalpha", v)
	v, err = rtc.Get(context.Background(), beta)
	require.NoError(t, err)
	assert.Equal(t, "betabeta", v)

	// Make sure we've only called the rtf once each.
	assert.Equal(t, map[string]int{
		alpha: 1,
		beta:  1,
	}, called)

	// Check a few more times to make sure it is cached:
	v, err = rtc.Get(context.Background(), alpha)
	require.NoError(t, err)
	assert.Equal(t, "alphaalpha", v)
	v, err = rtc.Get(context.Background(), alpha)
	require.NoError(t, err)
	assert.Equal(t, "alphaalpha", v)
	v, err = rtc.Get(context.Background(), beta)
	require.NoError(t, err)
	assert.Equal(t, "betabeta", v)

	// Make sure we cache the calls after the first
	assert.Equal(t, map[string]int{
		alpha: 1,
		beta:  1,
	}, called)

	assert.Len(t, rtc.Keys(), 2)
	assert.Contains(t, rtc.Keys(), alpha)
	assert.Contains(t, rtc.Keys(), beta)
}

// TestMemReadThroughCacheRace checks that multiple concurrent calls to Get
func TestMemReadThroughCacheRace(t *testing.T) {
	unittest.SmallTest(t)

	const alpha = "alpha"
	const beta = "beta"

	workerFn := func(ctx context.Context, id string) (interface{}, error) {
		assert.NotNil(t, ctx)
		return id + id, nil
	}

	rtc, err := New(workerFn, 10, 3)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		s := alpha
		if i%2 == 0 {
			s = beta
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := rtc.Get(context.Background(), s)
			require.NoError(t, err)
		}()
	}

	wg.Wait()
}

// TestMemReadThroughCacheRemove checks that if we call remove, we re-fetch the value
// on the next call
func TestMemReadThroughCacheRemove(t *testing.T) {
	unittest.SmallTest(t)

	const alpha = "alpha"
	const beta = "beta"

	called := map[string]int{}

	workerFn := func(ctx context.Context, id string) (interface{}, error) {
		assert.NotNil(t, ctx)
		called[id]++
		return id + id, nil
	}

	rtc, err := New(workerFn, 10, 10)
	require.NoError(t, err)

	v, err := rtc.Get(context.Background(), alpha)
	require.NoError(t, err)
	assert.Equal(t, "alphaalpha", v)

	assert.True(t, rtc.Contains(alpha))
	// If we remove keys that don't exist, it shouldn't crash.
	rtc.Remove([]string{alpha, beta})
	assert.False(t, rtc.Contains(alpha))

	v, err = rtc.Get(context.Background(), alpha)
	require.NoError(t, err)
	assert.Equal(t, "alphaalpha", v)

	// Make sure we have called it twice, because we removed the cached value
	// after the first fetch.
	assert.Equal(t, map[string]int{
		alpha: 2,
	}, called)
}

// TestMemReadThroughCacheGetErrors checks that if a worker function call returns error, we will
// try it again later
func TestMemReadThroughCacheGetErrors(t *testing.T) {
	unittest.SmallTest(t)

	const alpha = "alpha"

	called := map[string]int{}

	// Fails on the first call, succeeds on subsequent calls.
	workerFn := func(ctx context.Context, id string) (interface{}, error) {
		assert.NotNil(t, ctx)
		called[id]++
		if called[id] == 1 {
			return nil, errors.New("oops")
		}
		return id + id, nil
	}

	rtc, err := New(workerFn, 10, 10)
	require.NoError(t, err)

	_, err = rtc.Get(context.Background(), alpha)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "oops")

	v, err := rtc.Get(context.Background(), alpha)
	require.NoError(t, err)
	assert.Equal(t, "alphaalpha", v)

	// This should be a cached call
	v, err = rtc.Get(context.Background(), alpha)
	require.NoError(t, err)
	assert.Equal(t, "alphaalpha", v)

	// Make sure we have called it twice, because the first time was an error.
	assert.Equal(t, map[string]int{
		alpha: 2,
	}, called)
}
