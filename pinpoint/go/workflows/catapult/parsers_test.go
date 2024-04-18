package catapult

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/pinpoint/go/compare"
)

func TestParseImprovementDirection_Up_0(t *testing.T) {
	assert.Equal(t, int32(0), parseImprovementDir(compare.Up))
}

func TestParseImprovementDirection_Down_1(t *testing.T) {
	assert.Equal(t, int32(1), parseImprovementDir(compare.Down))
}
func TestParseImprovementDirection_UnknownDir_4(t *testing.T) {
	assert.Equal(t, int32(4), parseImprovementDir(compare.UnknownDir))
}
