package roller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGetSheriffJS(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, []string{"me@chromium.org"}, getSheriffJS("document.write('me')"))
	assert.Equal(t, []string{"a@chromium.org", "b@chromium.org", "c@chromium.org", "d@chromium.org"}, getSheriffJS("document.write('a, b, c, d')"))
	assert.Equal(t, []string{}, getSheriffJS(""))
	assert.Equal(t, []string{}, getSheriffJS("document.write('')"))
	assert.Equal(t, []string{"me@chromium.org"}, getSheriffJS("\n document.write('me  ')\n\n  "))
}
