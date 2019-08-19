package repograph

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/git"
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

	repo, err := git.NewRepo(ctx, g.Dir(), tmp)
	assert.NoError(t, err)

	repoImpl, err := NewLocalRepoImpl(ctx, repo)
	assert.NoError(t, err)
	g1, err := NewWithRepoImpl(ctx, repoImpl)
	assert.NoError(t, err)
	ri := repoImpl.(*localRepoImpl)

	// Add some commits.
	for i := 0; i < 10; i++ {
		g.CommitGen(ctx, "dummy.txt")
	}
	assert.NoError(t, g1.Update(ctx))
	assert.Equal(t, 10, g1.Len())
	assert.Equal(t, 10, len(ri.commits))
	assert.Equal(t, 1, len(ri.branches))

	// Build a new Graph.
	buf := &bytes.Buffer{}
	assert.NoError(t, g1.WriteGob(buf))
	g2, err := NewFromGob(ctx, buf, NewMemCacheRepoImpl(nil, nil))
	assert.NoError(t, err)

	// Assert that we get the same branches and commits.
	deepequal.AssertDeepEqual(t, g1.branches, g2.branches)
	deepequal.AssertDeepEqual(t, g2.commits, g2.commits)
}
