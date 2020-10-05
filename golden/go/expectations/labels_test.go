package expectations

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils/unittest"
)

func TestValidLabel_KnownLabel_ReturnsTrue(t *testing.T) {
	unittest.SmallTest(t)
	assert.True(t, ValidLabel(Untriaged))
	assert.True(t, ValidLabel(Positive))
	assert.True(t, ValidLabel(Negative))

}

func TestValidLabel_UnknownLabel_ReturnsFalse(t *testing.T) {
	unittest.SmallTest(t)
	assert.False(t, ValidLabel("unknown label"))
}
