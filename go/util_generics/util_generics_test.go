package util_generics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGet(t *testing.T) {
	someMap := map[string]int{
		"one": 1,
		"two": 2,
	}
	assert.Equal(t, 1, Get(someMap, "one", 99))           // key present
	assert.Equal(t, 99, Get(someMap, "four billion", 99)) // key absent

	anotherMap := map[string]string{
		"one": "uno",
		"two": "dos",
	}
	assert.Equal(t, "uno", Get(anotherMap, "one", "unknown"))              // key present
	assert.Equal(t, "unknown", Get(anotherMap, "four billion", "unknown")) // key absent
}
