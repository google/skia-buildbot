package expectations

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils/unittest"
)

func TestLabelInt_String_Success(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, Untriaged, UntriagedInt.String())
	assert.Equal(t, Positive, PositiveInt.String())
	assert.Equal(t, Negative, NegativeInt.String())
}

func TestLabelIntFromString_KnownLabel_ReturnsCorrespondingLabelInt(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, UntriagedInt, LabelIntFromString(Untriaged))
	assert.Equal(t, PositiveInt, LabelIntFromString(Positive))
	assert.Equal(t, NegativeInt, LabelIntFromString(Negative))
}

func TestLabelFromString_UnknownLabel_ReturnsUntriagedInt(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, UntriagedInt, LabelIntFromString("unknown label"))
}

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
