package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidDigest(t *testing.T) {

	assert.False(t, IsValidDigest(""))
	assert.True(t, IsValidDigest("766923700b970e4e7ecf9508b8455e0d"))
	assert.True(t, IsValidDigest("766923700b970e4e7ecf9508b8455e0d"))
	assert.False(t, IsValidDigest("766923700b970e4e7ecf9508b8455e0x"))
	assert.False(t, IsValidDigest("766923700b970e4e7ECf9508b8455e0x"))
	assert.False(t, IsValidDigest("766923700b970e4e7ecf08b8455e0f"))
}
