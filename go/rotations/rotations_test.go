package rotations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGetRotationJS(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, []string{"me@chromium.org"}, getRotationJS("document.write('me')"))
	assert.Equal(t, []string{"a@chromium.org", "b@chromium.org", "c@chromium.org", "d@chromium.org"}, getRotationJS("document.write('a, b, c, d')"))
	assert.Equal(t, []string{}, getRotationJS(""))
	assert.Equal(t, []string{}, getRotationJS("document.write('')"))
	assert.Equal(t, []string{"me@chromium.org"}, getRotationJS("\n document.write('me  ')\n\n  "))
}
