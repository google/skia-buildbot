package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestMedia(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, "CPU", string(CPU))
	assert.Equal(t, "pdf.pdf", mediaProps[PDF].filename)
	assert.Equal(t, "abcd-GPU", cacheKey("abcd", GPU))
}
