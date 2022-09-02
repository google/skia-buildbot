package repograph

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
)

// TestSerializeLocalRepo tests the serialization of localRepoImpl to/from disk.
func TestSerializeLocalRepo(t *testing.T) {
	ctx := context.Background()
	g := git_testutils.GitInit(t, ctx)
	defer g.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repo, err := git.NewRepo(ctx, g.Dir(), tmp)
	require.NoError(t, err)

	repoImpl, err := NewLocalRepoImpl(ctx, repo)
	require.NoError(t, err)
	g1, err := NewWithRepoImpl(ctx, repoImpl)
	require.NoError(t, err)
	ri := repoImpl.(*localRepoImpl)

	// Add some commits.
	for i := 0; i < 10; i++ {
		g.CommitGen(ctx, "dummy.txt")
	}
	require.NoError(t, g1.Update(ctx))
	require.Equal(t, 10, g1.Len())
	require.Equal(t, 10, len(ri.commits))
	require.Equal(t, 1, len(ri.branches))

	// Build a new Graph.
	buf := &bytes.Buffer{}
	require.NoError(t, g1.WriteGob(buf))
	g2, err := NewFromGob(ctx, buf, NewMemCacheRepoImpl(nil, nil))
	require.NoError(t, err)

	// Assert that we get the same branches and commits.
	assertdeep.Equal(t, g1.branches, g2.branches)
	assertdeep.Equal(t, g2.commits, g2.commits)
}
