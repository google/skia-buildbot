package failures

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdd(t *testing.T) {
	f := Failures{}
	f.Add("include/core/SkColorSpace.h", "Bot-1")
	f.Add("include/core/SkColorSpace.h ", "Bot-2")
	f.Add("  include/core/SkColorSpace.h", "Bot-2")
	f.Add("include/core/SkRect.h", "Bot-2")
	f.Add("/COMMIT_MSG", "Bot-2")

	assert.Equal(t, 2, f["include/core/SkColorSpace.h"]["Bot-2"])
	assert.Equal(t, 1, f["include/core/SkColorSpace.h"]["Bot-1"])
	assert.Equal(t, 0, f["include/core/SkColorSpace.h"]["unknown bot"])
	assert.Equal(t, 0, f["/COMMIT_MSG"]["Bot-2"])

	assert.Equal(t, 3, f["include"]["Bot-2"])
	assert.Equal(t, 3, f["include/core"]["Bot-2"])
}
