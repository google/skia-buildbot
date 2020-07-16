package expectations

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils/unittest"
)

func TestLabel_String_Success(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, UntriagedStr, Untriaged.String())
	assert.Equal(t, PositiveStr, Positive.String())
	assert.Equal(t, NegativeStr, Negative.String())
}

func TestLabelFromString_KnownLabelStr_ReturnsCorrespondingLabel(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, Untriaged, LabelFromString(UntriagedStr))
	assert.Equal(t, Positive, LabelFromString(PositiveStr))
	assert.Equal(t, Negative, LabelFromString(NegativeStr))
}

func TestLabelFromString_UnknownLabelStr_ReturnsUntriaged(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, Untriaged, LabelFromString("unknown label"))
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
