package atomic_miss_cache

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

type backingCache struct {
	cache map[string]*string
}

func (c *backingCache) Get(ctx context.Context, key string) (Value, error) {
	val, ok := c.cache[key]
	if !ok {
		return nil, ErrNoSuchEntry
	}
	return val, nil
}

func (c *backingCache) Set(ctx context.Context, key string, val Value) error {
	c.cache[key] = val.(*string)
	return nil
}

func (c *backingCache) Delete(ctx context.Context, key string) error {
	delete(c.cache, key)
	return nil
}

func TestAtomicMissCache(t *testing.T) {

	ctx := context.Background()

	// Basic tests for the cache.
	test := func(c *AtomicMissCache) {
		k := "key"
		got, err := c.Get(ctx, k)
		require.Equal(t, ErrNoSuchEntry, err)
		require.Nil(t, got)
		val1Str := "hello"
		val1 := &val1Str
		require.NoError(t, c.Set(ctx, k, val1))
		got, err = c.Get(ctx, k)
		require.NoError(t, err)
		require.True(t, val1 == got)

		// We shouldn't set the new value in SetIfUnset.
		val2Str := "world"
		val2 := &val2Str
		got, err = c.SetIfUnset(ctx, k, func(ctx context.Context) (Value, error) {
			return val2, nil
		})
		require.NoError(t, err)
		require.True(t, val1 == got)
		got, err = c.Get(ctx, k)
		require.NoError(t, err)
		require.True(t, val1 == got)
		require.False(t, val2 == got)

		// Ensure that we do actually SetIfUnset.
		got, err = c.SetIfUnset(ctx, "key2", func(ctx context.Context) (Value, error) {
			return val2, nil
		})
		require.NoError(t, err)
		require.True(t, val2 == got)
		got, err = c.Get(ctx, "key2")
		require.NoError(t, err)
		require.True(t, val2 == got)

		// Ensure that the error is passed back.
		got, err = c.SetIfUnset(ctx, "key3", func(ctx context.Context) (Value, error) {
			return val2, errors.New("fail")
		})
		require.EqualError(t, err, "fail")
		require.Nil(t, got)
		got, err = c.Get(ctx, "key3")
		require.Equal(t, ErrNoSuchEntry, err)
		require.Nil(t, got)
	}

	// The cache should work with or without a backing cache.
	c1 := New(nil)
	test(c1)
	bc := &backingCache{
		cache: map[string]*string{},
	}
	c2 := New(bc)
	test(c2)

	// Test that we read and write through to the backing cache.
	val3Str := "hi"
	val3 := &val3Str
	key4 := "key4"
	require.NoError(t, bc.Set(ctx, key4, val3))
	got, err := c2.Get(ctx, key4)
	require.NoError(t, err)
	require.True(t, val3 == got)

	val4Str := "blah"
	val4 := &val4Str
	require.NoError(t, c2.Set(ctx, key4, val4))
	got, err = c2.Get(ctx, key4)
	require.NoError(t, err)
	require.True(t, val4 == got)
	got, err = bc.Get(ctx, key4)
	require.NoError(t, err)
	require.True(t, val4 == got)

	// The backing cache has a value but the cache itself doesn't. Ensure
	// that we pull it from the backing cache and don't overwrite it.
	key5 := "key5"
	require.NoError(t, bc.Set(ctx, key5, val3))
	got, err = c2.SetIfUnset(ctx, key5, func(ctx context.Context) (Value, error) {
		return val4, nil
	})
	require.NoError(t, err)
	require.True(t, val3 == got)
	got, err = c2.Get(ctx, key5)
	require.NoError(t, err)
	require.True(t, val3 == got)

	// Delete an entry.
	require.NoError(t, c2.Delete(ctx, key5))
	got, err = c2.Get(ctx, key5)
	require.Equal(t, ErrNoSuchEntry, err)
	require.Nil(t, got)
	got, err = bc.Get(ctx, key5)
	require.Equal(t, ErrNoSuchEntry, err)
	require.Nil(t, got)
}

func TestAtomicMissCacheLocking(t *testing.T) {

	ctx := context.Background()
	wait := make(chan struct{})
	c := New(nil)
	k1 := "k1"
	v1 := &struct{}{}

	go func() {
		// Wait for the below func to start, indicating that the cache
		// entry should be locked.
		<-wait
		got, err := c.Get(ctx, k1)
		require.NoError(t, err)
		require.True(t, v1 == got)
	}()

	got, err := c.SetIfUnset(ctx, k1, func(ctx context.Context) (Value, error) {
		// Signal the other goroutine to start.
		wait <- struct{}{}
		return v1, nil
	})
	require.NoError(t, err)
	require.True(t, v1 == got)
}

type miniEntry struct {
	val int
}

func cacheLen(c *AtomicMissCache) int {
	length := 0
	c.ForEach(context.Background(), func(_ context.Context, _ string, _ Value) {
		length++
	})
	return length
}

func TestAtomicMissCacheForEach(t *testing.T) {

	ctx := context.Background()
	c := New(nil)
	for i := 0; i < 10; i++ {
		require.NoError(t, c.Set(ctx, strconv.Itoa(i), &miniEntry{val: i}))
	}
	got := map[string]bool{}
	c.ForEach(ctx, func(ctx context.Context, key string, value Value) {
		got[key] = true
	})
	require.Equal(t, 10, len(got))
	require.NoError(t, c.Cleanup(ctx, func(ctx context.Context, key string, value Value) bool {
		i, err := strconv.Atoi(key)
		require.NoError(t, err)
		return i >= 5
	}))
	require.Equal(t, 5, cacheLen(c))
	got = map[string]bool{}
	c.ForEach(ctx, func(ctx context.Context, key string, value Value) {
		got[key] = true
		i, err := strconv.Atoi(key)
		require.NoError(t, err)
		require.False(t, i >= 5)
	})
	require.Equal(t, 5, len(got))
}
