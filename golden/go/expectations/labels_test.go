package expectations

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils/unittest"
)

func TestLabel_String_Success(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, UntriagedStr, UntriagedInt.String())
	assert.Equal(t, PositiveStr, PositiveInt.String())
	assert.Equal(t, NegativeStr, NegativeInt.String())
}

func TestLabelFromString_KnownLabelStr_ReturnsCorrespondingLabel(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, UntriagedInt, LabelFromString(UntriagedStr))
	assert.Equal(t, PositiveInt, LabelFromString(PositiveStr))
	assert.Equal(t, NegativeInt, LabelFromString(NegativeStr))
}

func TestLabelFromString_UnknownLabelStr_ReturnsUntriaged(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, UntriagedInt, LabelFromString("unknown label"))
}

func TestValidLabel_KnownLabelStr_ReturnsTrue(t *testing.T) {
	unittest.SmallTest(t)
	assert.True(t, ValidLabelStr(UntriagedStr))
	assert.True(t, ValidLabelStr(PositiveStr))
	assert.True(t, ValidLabelStr(NegativeStr))

}

func TestValidLabel_UnknownLabelStr_ReturnsFalse(t *testing.T) {
	unittest.SmallTest(t)
	assert.False(t, ValidLabelStr("unknown label"))
}
