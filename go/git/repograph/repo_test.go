package repograph

import (
	"context"
	"io/ioutil"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/deepequal"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestSerialize(t *testing.T) {
	unittest.MediumTest(t)
	ctx := context.Background()
	g := git_testutils.GitInit(t, ctx)
	defer g.Cleanup()

	tmp1, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp1)

	repo, err := NewLocalGraph(ctx, g.Dir(), tmp1)
	assert.NoError(t, err)

	// Add some commits.
	for i := 0; i < 10; i++ {
		g.CommitGen(ctx, "dummy.txt")
	}
	assert.NoError(t, repo.Update(ctx))
	assert.Equal(t, 10, repo.Len())

	tmp2, err := ioutil.TempDir("", "")
	defer testutils.RemoveAll(t, tmp2)
	cacheFile := path.Join(tmp2, CACHE_FILE)
	assert.NoError(t, writeCacheFile(repo, cacheFile))
	repo2 := &Graph{
		repoImpl: repo.repoImpl,
	}
	assert.NoError(t, initFromFile(repo2, cacheFile))
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, repo, repo2)
}
