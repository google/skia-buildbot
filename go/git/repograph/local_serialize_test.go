package repograph

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

// TestSerializeLocalRepo tests the serialization of localRepoImpl to/from disk.
func TestSerializeLocalRepo(t *testing.T) {
	unittest.MediumTest(t)
	ctx := context.Background()
	g := git_testutils.GitInit(t, ctx)
	defer g.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repoImpl, err := NewLocalRepoImpl(ctx, g.Dir(), tmp)
	assert.NoError(t, err)
	repo, err := NewWithRepoImpl(ctx, repoImpl)
	assert.NoError(t, err)
	ri := repoImpl.(*localRepoImpl)

	// Add some commits.
	for i := 0; i < 10; i++ {
		g.CommitGen(ctx, "dummy.txt")
	}
	assert.NoError(t, repo.Update(ctx))
	assert.Equal(t, 10, repo.Len())
	assert.Equal(t, 10, len(ri.commits))
	assert.Equal(t, 1, len(ri.branches))

	// Copy the cache file to a new loction.
	cacheFile := path.Join(tmp, "cache-file.cpy")
	b, err := ioutil.ReadFile(path.Join(ri.Repo.Dir(), CACHE_FILE))
	assert.NoError(t, err)
	assert.NoError(t, ioutil.WriteFile(cacheFile, b, os.ModePerm))

	// Assert that we get the same branches and commits.
	branches, commits, err := initFromFile(cacheFile)
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, ri.branches, branches)
	deepequal.AssertDeepEqual(t, ri.commits, commits)
}
