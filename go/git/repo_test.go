package git

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func setup(t *testing.T) (context.Context, *git_testutils.GitBuilder, []string) {
	unittest.LargeTest(t)

	// Create a local git repo to play with.
	ctx := context.Background()
	g := git_testutils.GitInit(t, ctx)
	commits := make([]string, 10)
	for i := 0; i < 10; i++ {
		commits[9-i] = g.CommitGenMsg(ctx, "somefile", fmt.Sprintf("Commit Title #%d\n\nCommit Body #%d", i, i))
	}
	return ctx, g, commits
}

func TestRepo(t *testing.T) {
	ctx, gb, commits := setup(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	r, err := NewRepo(ctx, gb.Dir(), tmp)
	require.NoError(t, err)

	// Verify that we can run git commands.
	_, err = r.Git(ctx, "branch")
	require.NoError(t, err)

	// Verify that we don't have a working copy.
	_, err = r.Git(ctx, "status")
	require.Error(t, err)
	_, err = r.Git(ctx, "checkout", MasterBranch)
	require.Error(t, err)

	// Log.
	gotCommits, err := r.RevList(ctx, MasterBranch)
	require.NoError(t, err)
	assertdeep.Equal(t, commits, gotCommits)

	// Add a commit on the remote.
	c := gb.CommitGen(ctx, "somefile")

	// Verify that Update() succeeds.
	require.NoError(t, r.Update(ctx))

	// Verify that we got the new commit.
	got, err := r.RevParse(ctx, c)
	require.NoError(t, err)
	require.Equal(t, c, strings.TrimSpace(got))

	// Verify that we can create a Checkout from the Repo. No need to test
	// the Checkout since that struct has its own tests.
	tmp2, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp2)
	_, err = r.Checkout(ctx, tmp2)
	require.NoError(t, err)
}
