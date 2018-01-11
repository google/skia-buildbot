package failures

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
)

func TestAddPredict(t *testing.T) {
	testutils.SmallTest(t)
	f := Failures{}
	f.Add("include/core/SkColorSpace.h", "Bot-1")
	f.Add("include/core/SkColorSpace.h ", "Bot-2")
	f.Add("  include/core/SkColorSpace.h", "Bot-2")
	f.Add("include/core/SkRect.h", "Bot-2")
	f.Add("include/core/SkOval.h", "Bot-3")
	f.Add("/COMMIT_MSG", "Bot-2")
	f.Add("include/utils/SkHelper.h", "Bot-5")

	assert.Equal(t, 2, f["include/core/SkColorSpace.h"]["Bot-2"])
	assert.Equal(t, 1, f["include/core/SkColorSpace.h"]["Bot-1"])
	assert.Equal(t, 0, f["include/core/SkColorSpace.h"]["unknown bot"])
	assert.Equal(t, 0, f["/COMMIT_MSG"]["Bot-2"])

	assert.Equal(t, 3, f["include"]["Bot-2"])
	assert.Equal(t, 3, f["include/core"]["Bot-2"])

	p := f.Predict([]string{"include/core/SkColorSpace.h"})
	assert.Equal(t, []*Summary{
		{"Bot-2", 2},
		{"Bot-1", 1},
	}, p)
	p = f.Predict([]string{"include/core/some-other-file-in-core.h"})
	assert.Equal(t, []*Summary{
		{"Bot-2", 3},
		{"Bot-1", 1},
		{"Bot-3", 1},
	}, p)
	p = f.Predict([]string{"include/gpu/GrSomething.h"})
	assert.Equal(t, []*Summary{
		{"Bot-2", 3},
		{"Bot-1", 1},
		{"Bot-3", 1},
		{"Bot-5", 1},
	}, p)
	p = f.Predict([]string{"src/core/GrSomething.cpp"})
	assert.Equal(t, []*Summary{}, p)
}
