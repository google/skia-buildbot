package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortedKeys(t *testing.T) {
	t.Run("nil map", func(t *testing.T) {
		var m map[string]int
		assert.Nil(t, SortedKeys(m))
	})

	t.Run("empty map", func(t *testing.T) {
		m := map[string]int{}
		assert.Empty(t, SortedKeys(m))
	})

	t.Run("sorted keys string", func(t *testing.T) {
		m := map[string]int{
			"c": 3,
			"a": 1,
			"b": 2,
		}
		assert.Equal(t, []string{"a", "b", "c"}, SortedKeys(m))
	})

	t.Run("sorted keys int", func(t *testing.T) {
		m := map[int]string{
			3: "c",
			1: "a",
			2: "b",
		}
		assert.Equal(t, []int{1, 2, 3}, SortedKeys(m))
	})
}

func TestSortedRange(t *testing.T) {
	t.Run("nil map", func(t *testing.T) {
		var m map[string]int
		called := false
		for range SortedRange(m) {
			called = true
		}
		assert.False(t, called)
	})

	t.Run("empty map", func(t *testing.T) {
		m := map[string]int{}
		called := false
		for range SortedRange(m) {
			called = true
		}
		assert.False(t, called)
	})

	t.Run("iterate sorted", func(t *testing.T) {
		m := map[string]int{
			"c": 3,
			"a": 1,
			"b": 2,
		}
		var keys []string
		var values []int
		for k, v := range SortedRange(m) {
			keys = append(keys, k)
			values = append(values, v)
		}
		assert.Equal(t, []string{"a", "b", "c"}, keys)
		assert.Equal(t, []int{1, 2, 3}, values)
	})

	t.Run("early exit", func(t *testing.T) {
		m := map[string]int{
			"c": 3,
			"a": 1,
			"b": 2,
		}
		var keys []string
		for k := range SortedRange(m) {
			keys = append(keys, k)
			if k == "b" {
				break
			}
		}
		assert.Equal(t, []string{"a", "b"}, keys)
	})
}
