package util

import (
	"fmt"
	"strconv"

	"github.com/stretchr/testify/assert"
)

type myTestType struct {
	A int
	B string
}

func UnitTestCodec() LRUCodec {
	return JSONCodec(&myTestType{})
}

func UnitTestLRUCache(t assert.TestingT, cache LRUCache) {
	purge(t, cache)
	N := 256
	for i := 0; i < N; i++ {
		cache.Add(strconv.Itoa(i), i)
	}

	// Make sure out keys are correct
	assert.Equal(t, N, cache.Len())
	cacheKeys := cache.Keys()
	assert.Equal(t, N, len(cacheKeys))
	for _, k := range cacheKeys {
		assert.IsType(t, "", k)
		v, ok := cache.Get(k)
		assert.True(t, ok)
		assert.IsType(t, 0, v)
		assert.Equal(t, k, strconv.Itoa(v.(int)))
	}

	for i := 0; i < N; i++ {
		found, ok := cache.Get(strconv.Itoa(i))
		assert.True(t, ok)
		assert.IsType(t, 0, found)
		assert.Equal(t, found.(int), i)
	}

	for i := 0; i < N; i++ {
		_, ok := cache.Get(strconv.Itoa(i))
		assert.True(t, ok)
		oldLen := cache.Len()
		cache.Remove(strconv.Itoa(i))
		assert.Equal(t, oldLen-1, cache.Len())
	}
	assert.Equal(t, 0, cache.Len())

	// Add some TestStructs to make sure the codec works.
	for i := 0; i < N; i++ {
		strKey := "structkey-" + strconv.Itoa(i)

		ts := &myTestType{
			A: i,
			B: fmt.Sprintf("Val %d", i),
		}
		cache.Add(strKey, ts)
		assert.Equal(t, i+1, cache.Len())
		foundTS, ok := cache.Get(strKey)
		assert.True(t, ok)
		assert.IsType(t, &myTestType{}, foundTS)
		assert.Equal(t, ts, foundTS)
	}
}

func purge(t assert.TestingT, cache LRUCache) {
	for _, k := range cache.Keys() {
		cache.Remove(k)
	}
	assert.Equal(t, 0, cache.Len())
}
