package cache

import (
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {
	c := NewDiffCache()

	assert.False(t, true)
}
