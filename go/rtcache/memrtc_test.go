package rtcache

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemReadThroughCacheGetSunnyDay checks that we cache the values after reading them
// from the passed in read-through function.
func TestMemReadThroughCacheGetSunnyDay(t *testing.T) {

	const alpha = "alpha"
	const beta = "beta"

	called, rtFn := countingReadThroughFn(t)

	rtc, err := New(rtFn, 10, 10)
	require.NoError(t, err)

	assert.Empty(t, rtc.Keys())

	v, err := rtc.Get(context.Background(), alpha)
	require.NoError(t, err)
	assert.Equal(t, "alphaalpha", v)
	v, err = rtc.Get(context.Background(), beta)
	require.NoError(t, err)
	assert.Equal(t, "betabeta", v)

	// Make sure we've only called the rtFn once each.
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
	assert.Equal(t, 2, rtc.Len())
}

// TestMemReadThroughCacheGetAllSunnyDay checks that GetAll returns correctly when either both
// things are cached or both things are not cached.
func TestMemReadThroughCacheGetAllSunnyDay(t *testing.T) {

	const alpha = "alpha"
	const beta = "beta"

	called, rtFn := countingReadThroughFn(t)

	rtc, err := New(rtFn, 10, 10)
	require.NoError(t, err)

	v, err := rtc.GetAll(context.Background(), []string{alpha, beta})
	require.NoError(t, err)
	assert.Equal(t, []interface{}{"alphaalpha", "betabeta"}, v)

	// Make sure we've only called the rtFn once each.
	assert.Equal(t, map[string]int{
		alpha: 1,
		beta:  1,
	}, called)

	// Check a few more times to make sure it is cached:
	v, err = rtc.GetAll(context.Background(), []string{alpha, beta})
	require.NoError(t, err)
	assert.Equal(t, []interface{}{"alphaalpha", "betabeta"}, v)

	v, err = rtc.GetAll(context.Background(), []string{alpha, beta})
	require.NoError(t, err)
	assert.Equal(t, []interface{}{"alphaalpha", "betabeta"}, v)

	// Make sure we cache the calls after the first
	assert.Equal(t, map[string]int{
		alpha: 1,
		beta:  1,
	}, called)

	assert.Len(t, rtc.Keys(), 2)
	assert.Contains(t, rtc.Keys(), alpha)
	assert.Contains(t, rtc.Keys(), beta)
}

// TestMemReadThroughCacheGetAllPartial checks that GetAll behaves correctly when some things
// are cached, but other things are not
func TestMemReadThroughCacheGetAllPartial(t *testing.T) {

	const alpha = "alpha"
	const beta = "beta"

	called, rtFn := countingReadThroughFn(t)

	rtc, err := New(rtFn, 10, 10)
	require.NoError(t, err)

	v, err := rtc.GetAll(context.Background(), []string{alpha})
	require.NoError(t, err)
	assert.Equal(t, []interface{}{"alphaalpha"}, v)

	// Make sure we've only called the rtFn once each.
	assert.Equal(t, map[string]int{
		alpha: 1,
	}, called)

	// Check again to make sure it is cached:
	v, err = rtc.GetAll(context.Background(), []string{beta, alpha})
	require.NoError(t, err)
	assert.Equal(t, []interface{}{"betabeta", "alphaalpha"}, v)

	// Make sure we cache the calls after the first
	assert.Equal(t, map[string]int{
		alpha: 1,
		beta:  1,
	}, called)
}

// TestMemReadThroughCacheRace checks that multiple concurrent calls to Get
func TestMemReadThroughCacheRace(t *testing.T) {

	const alpha = "alpha"
	const beta = "beta"

	rtFn := func(ctx context.Context, id []string) ([]interface{}, error) {
		assert.NotNil(t, ctx)
		return []interface{}{"racerace"}, nil
	}

	rtc, err := New(rtFn, 10, 3)
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

	const alpha = "alpha"
	const beta = "beta"

	called, rtFn := countingReadThroughFn(t)

	rtc, err := New(rtFn, 10, 10)
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

	const alpha = "alpha"

	called := map[string]int{}

	// Fails on the first call, succeeds on subsequent calls.
	rtFn := func(ctx context.Context, ids []string) ([]interface{}, error) {
		assert.NotNil(t, ctx)
		assert.Len(t, ids, 1)
		id := ids[0]
		called[id] = called[id] + 1
		if called[id] == 1 {
			return nil, errors.New("oops")
		}
		return []interface{}{id + id}, nil
	}

	rtc, err := New(rtFn, 10, 10)
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

// TestMemReadThroughCacheGetAllErrors checks that if a worker function call returns error, we will
// try it again later
func TestMemReadThroughCacheGetAllErrors(t *testing.T) {

	const alpha = "alpha"
	const beta = "beta"

	called := 0

	// Fails on the first call, succeeds on subsequent calls.
	rtFn := func(ctx context.Context, ids []string) ([]interface{}, error) {
		assert.NotNil(t, ctx)
		called++
		if called == 1 {
			return nil, errors.New("oops")
		}
		rv := make([]interface{}, 0, len(ids))
		for _, id := range ids {
			rv = append(rv, id+id)
		}
		return rv, nil
	}

	rtc, err := New(rtFn, 10, 10)
	require.NoError(t, err)

	_, err = rtc.GetAll(context.Background(), []string{alpha, beta})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "oops")

	assert.Equal(t, 1, called)

	v, err := rtc.GetAll(context.Background(), []string{alpha, beta})
	require.NoError(t, err)
	assert.Equal(t, []interface{}{"alphaalpha", "betabeta"}, v)

	// Make sure we have called it twice, because the first time was an error.
	assert.Equal(t, 2, called)
}

// countingReadThroughFn returns a ReadThroughFunc that simply increments a count for each
// time it has seen a given id and returns (id+id) as the "computed value". The first return
// value is a map of the ids to the respective counts.
func countingReadThroughFn(t *testing.T) (map[string]int, ReadThroughFunc) {
	called := map[string]int{}

	rtFn := func(ctx context.Context, ids []string) ([]interface{}, error) {
		assert.NotNil(t, ctx)
		rv := make([]interface{}, 0, len(ids))
		for _, id := range ids {
			called[id]++
			rv = append(rv, id+id)
		}
		return rv, nil
	}
	return called, rtFn
}
