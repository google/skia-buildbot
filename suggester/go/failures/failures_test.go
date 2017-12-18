package failures

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/suggester/go/dsconst"
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

func TestStore(t *testing.T) {
	testutils.MediumTest(t)

	cleanup := testutil.InitDatastore(t, dsconst.FLAKY_RANGES)
	defer cleanup()

	ctx := context.Background()
	g := git_testutils.GitInit(t, ctx)
	defer g.Cleanup()

	git := &git.Checkout{git.GitDir(g.Dir())}
	_ = New(nil, nil, ds.DS, git, nil, g.RepoUrl())
}
