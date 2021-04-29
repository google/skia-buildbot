package git

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils"
)

func TestCheckout(t *testing.T) {
	ctx, gb, commits := setup(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	c, err := NewCheckout(ctx, gb.Dir(), tmp)
	require.NoError(t, err)

	// Verify that we can run git commands.
	_, err = c.Git(ctx, "branch")
	require.NoError(t, err)

	// Verify that we have a working copy.
	_, err = c.Git(ctx, "status")
	require.NoError(t, err)
	_, err = c.Git(ctx, "checkout", MasterBranch)
	require.NoError(t, err)

	// Log.
	gotCommits, err := c.RevList(ctx, DefaultRemoteBranch)
	require.NoError(t, err)
	assertdeep.Equal(t, commits, gotCommits)

	// Add a commit on the remote.
	commit := gb.CommitGen(ctx, "somefile")

	// Verify that Update() succeeds.
	require.NoError(t, c.Update(ctx))

	// Verify that we got the new commit.
	got, err := c.RevParse(ctx, "HEAD")
	require.NoError(t, err)
	require.Equal(t, commit, got)

	// Verify that we correctly clean the repo.
	clean := func() bool {
		_, err = c.Git(ctx, "diff", "--no-ext-diff", "--exit-code", DefaultRemoteBranch)
		if err != nil {
			return false
		}
		out, err := c.Git(ctx, "ls-files", "--other", "--exclude-standard")
		require.NoError(t, err)
		untracked := strings.Fields(out)
		if len(untracked) != 0 {
			return false
		}
		h1, err := c.RevParse(ctx, MasterBranch)
		require.NoError(t, err)
		h2, err := c.RevParse(ctx, DefaultRemoteBranch)
		require.NoError(t, err)
		h3, err := c.RevParse(ctx, "HEAD")
		if h1 != h2 {
			return false
		}
		if h2 != h3 {
			return false
		}
		return true
	}
	updateAndAssertClean := func() {
		require.False(t, clean()) // Sanity check.
		require.NoError(t, c.Update(ctx))
		require.True(t, clean())
	}

	// 1. Local modification (no git add).
	require.NoError(t, ioutil.WriteFile(path.Join(c.Dir(), "somefile"), []byte("contents"), os.ModePerm))
	updateAndAssertClean()

	// 2. Local modification (with git add).
	require.NoError(t, ioutil.WriteFile(path.Join(c.Dir(), "somefile"), []byte("contents"), os.ModePerm))
	_, err = c.Git(ctx, "add", "somefile")
	require.NoError(t, err)
	updateAndAssertClean()

	// 3. Untracked file.
	require.NoError(t, ioutil.WriteFile(path.Join(c.Dir(), "untracked"), []byte("contents"), os.ModePerm))
	updateAndAssertClean()

	// 4. Committed locally.
	require.NoError(t, ioutil.WriteFile(path.Join(c.Dir(), "somefile"), []byte("contents"), os.ModePerm))
	_, err = c.Git(ctx, "commit", "-a", "-m", "msg")
	require.NoError(t, err)
	updateAndAssertClean()
}

func TestTempCheckout(t *testing.T) {
	ctx, gb, _ := setup(t)
	defer gb.Cleanup()

	c, err := NewTempCheckout(ctx, gb.Dir())
	require.NoError(t, err)

	// Verify that we can run git commands.
	_, err = c.Git(ctx, "branch")
	require.NoError(t, err)

	// Verify that we have a working copy.
	_, err = c.Git(ctx, "status")
	require.NoError(t, err)

	// Delete the checkout.
	c.Delete()

	// Verify that the directory is gone.
	_, err = os.Stat(c.Dir())
	require.True(t, os.IsNotExist(err))
}
