package store

import (
	"testing"

	"go.skia.org/infra/go/testutils"

	"github.com/stretchr/testify/assert"
)

func TestMedia(t *testing.T) {
	testutils.SmallTest(t)
	assert.Equal(t, "CPU", string(CPU))
	assert.Equal(t, "pdf.pdf", mediaProps[PDF].filename)
	assert.Equal(t, "abcd-GPU", cacheKey("abcd", GPU))
}
