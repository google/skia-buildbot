package testutils

import (
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"testing"
)

func TestRoundFloat32ToDecimalPlace_RoundsWhereAppropopriate(t *testing.T) {
	unittest.SmallTest(t)
	// rounds down correctly
	assert.Equal(t, float32(1.0), RoundFloat32ToDecimalPlace(1.23456789, 0))
	assert.Equal(t, float32(1.2), RoundFloat32ToDecimalPlace(1.23456789, 1))
	assert.Equal(t, float32(1.23), RoundFloat32ToDecimalPlace(1.23456789, 2))
	// rounds up correctly
	assert.Equal(t, float32(1.235), RoundFloat32ToDecimalPlace(1.23456789, 3))
	assert.Equal(t, float32(1.2346), RoundFloat32ToDecimalPlace(1.23456789, 4))
	assert.Equal(t, float32(1.23457), RoundFloat32ToDecimalPlace(1.23456789, 5))
	assert.Equal(t, float32(1.234568), RoundFloat32ToDecimalPlace(1.23456789, 6))
}
