package git

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils"
)

func TestCheckout(t *testing.T) {
	ctx, gb, commits := setup(t)
	defer gb.Cleanup()

	tmp, err := os.MkdirTemp("", "")
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
	_, err = c.Git(ctx, "checkout", MainBranch)
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
		h1, err := c.RevParse(ctx, MainBranch)
		require.NoError(t, err)
		h2, err := c.RevParse(ctx, DefaultRemoteBranch)
		require.NoError(t, err)
		h3, err := c.RevParse(ctx, "HEAD")
		require.NoError(t, err)
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
	require.NoError(t, os.WriteFile(path.Join(c.Dir(), "somefile"), []byte("contents"), os.ModePerm))
	updateAndAssertClean()

	// 2. Local modification (with git add).
	require.NoError(t, os.WriteFile(path.Join(c.Dir(), "somefile"), []byte("contents"), os.ModePerm))
	_, err = c.Git(ctx, "add", "somefile")
	require.NoError(t, err)
	updateAndAssertClean()

	// 3. Untracked file.
	require.NoError(t, os.WriteFile(path.Join(c.Dir(), "untracked"), []byte("contents"), os.ModePerm))
	updateAndAssertClean()

	// 4. Committed locally.
	require.NoError(t, os.WriteFile(path.Join(c.Dir(), "somefile"), []byte("contents"), os.ModePerm))
	_, err = c.Git(ctx, "commit", "-a", "-m", "msg")
	require.NoError(t, err)
	updateAndAssertClean()
}

func TestCheckout_IsDirty(t *testing.T) {
	ctx, gb, _ := setup(t)
	defer gb.Cleanup()

	test := func(name string, expectDirty bool, fn func(*testing.T, CheckoutDir)) {
		t.Run(name, func(t *testing.T) {
			tmp, err := os.MkdirTemp("", "")
			require.NoError(t, err)
			defer testutils.RemoveAll(t, tmp)
			c, err := NewCheckout(ctx, gb.Dir(), tmp)
			require.NoError(t, err)
			fn(t, c)
			dirty, status, err := c.IsDirty(ctx)
			require.NoError(t, err)
			require.Equal(t, expectDirty, dirty, status)
		})
	}

	test("clean", false, func(t *testing.T, c CheckoutDir) {})
	test("unstaged_changes", true, func(t *testing.T, c CheckoutDir) {
		require.NoError(t, os.WriteFile(filepath.Join(c.Dir(), checkedInFile), []byte("blahblah"), os.ModePerm))
	})
	test("untracked_file", true, func(t *testing.T, c CheckoutDir) {
		require.NoError(t, os.WriteFile(filepath.Join(c.Dir(), "untracked-file"), []byte("blahblah"), os.ModePerm))
	})
	test("ahead_of_main", true, func(t *testing.T, c CheckoutDir) {
		require.NoError(t, os.WriteFile(filepath.Join(c.Dir(), checkedInFile), []byte("blahblah"), os.ModePerm))
		_, err := c.Git(ctx, "commit", "-a", "-m", "updated file")
		require.NoError(t, err)
	})
	test("behind_main", false, func(t *testing.T, c CheckoutDir) {
		_, err := c.Git(ctx, "reset", "--hard", "HEAD^")
		require.NoError(t, err)
	})
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
