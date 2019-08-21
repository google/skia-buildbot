package mapper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestIsValidImgID(t *testing.T) {
	unittest.SmallTest(t)

	// Arbitrary MD5 digests
	assert.True(t, IsValidImgID("098f6bcd4621d373cade4e832627b4f6"))
	assert.True(t, IsValidImgID("1660f0783f4076284bc18c5f4bdc9608"))

	// Invalid MD5 digest
	assert.False(t, IsValidImgID("nope"))
}
