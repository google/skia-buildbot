package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMedia(t *testing.T) {
	assert.Equal(t, "CPU", string(CPU))
	assert.Equal(t, "pdf.pdf", mediaProps[PDF].filename)
	assert.Equal(t, "abcd-GPU", cacheKey("abcd", GPU))
}
