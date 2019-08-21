package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestIsValidDigest(t *testing.T) {
	unittest.SmallTest(t)

	assert.False(t, IsValidDigest(""))
	assert.True(t, IsValidDigest("766923700b970e4e7ecf9508b8455e0d"))
	assert.True(t, IsValidDigest("766923700b970e4e7ecf9508b8455e0d"))
	assert.False(t, IsValidDigest("766923700b970e4e7ecf9508b8455e0x"))
	assert.False(t, IsValidDigest("766923700b970e4e7ECf9508b8455e0x"))
	assert.False(t, IsValidDigest("766923700b970e4e7ecf08b8455e0f"))
}

func TestIsValidDiffImgID(t *testing.T) {
	unittest.SmallTest(t)

	assert.False(t, IsValidDiffImgID(""))
	assert.False(t, IsValidDiffImgID("nope"))
	assert.False(t, IsValidDiffImgID("098f6b-1660f0783f4076284bc18c5f4bdc9608"))
	assert.False(t, IsValidDiffImgID("098f6bcd4621d373cade4e832627b4f6-1660f0"))
	assert.False(t, IsValidDiffImgID("HELLOWORLD21d373cade4e832627b4f6-1660f0783f4076284bc18c5f4bdc9608"))
	assert.False(t, IsValidDiffImgID("098f6bcd4621d373cade4e832627b4f6-HELLOWORLD4076284bc18c5f4bdc9608"))
	assert.True(t, IsValidDiffImgID("098f6bcd4621d373cade4e832627b4f6-1660f0783f4076284bc18c5f4bdc9608"))
}
