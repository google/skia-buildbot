package git

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
)

func TestCheckout(t *testing.T) {
	ctx, gb, commits := setup(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	c, err := NewCheckout(ctx, gb.Dir(), tmp)
	assert.NoError(t, err)

	// Verify that we can run git commands.
	_, err = c.Git(ctx, "branch")
	assert.NoError(t, err)

	// Verify that we have a working copy.
	_, err = c.Git(ctx, "status")
	assert.NoError(t, err)
	_, err = c.Git(ctx, "checkout", "master")
	assert.NoError(t, err)

	// Log.
	gotCommits, err := c.RevList(ctx, "origin/master")
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, commits, gotCommits)

	// Add a commit on the remote.
	commit := gb.CommitGen(ctx, "somefile")

	// Verify that Update() succeeds.
	assert.NoError(t, c.Update(ctx))

	// Verify that we got the new commit.
	got, err := c.RevParse(ctx, "HEAD")
	assert.NoError(t, err)
	assert.Equal(t, commit, got)

	// Verify that we correctly clean the repo.
	clean := func() bool {
		_, err = c.Git(ctx, "diff", "--no-ext-diff", "--exit-code", "origin/master")
		if err != nil {
			return false
		}
		out, err := c.Git(ctx, "ls-files", "--other", "--exclude-standard")
		assert.NoError(t, err)
		untracked := strings.Fields(out)
		if len(untracked) != 0 {
			return false
		}
		h1, err := c.RevParse(ctx, "master")
		assert.NoError(t, err)
		h2, err := c.RevParse(ctx, "origin/master")
		assert.NoError(t, err)
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
		assert.False(t, clean()) // Sanity check.
		assert.NoError(t, c.Update(ctx))
		assert.True(t, clean())
	}

	// 1. Local modification (no git add).
	assert.NoError(t, ioutil.WriteFile(path.Join(c.Dir(), "somefile"), []byte("contents"), os.ModePerm))
	updateAndAssertClean()

	// 2. Local modification (with git add).
	assert.NoError(t, ioutil.WriteFile(path.Join(c.Dir(), "somefile"), []byte("contents"), os.ModePerm))
	_, err = c.Git(ctx, "add", "somefile")
	assert.NoError(t, err)
	updateAndAssertClean()

	// 3. Untracked file.
	assert.NoError(t, ioutil.WriteFile(path.Join(c.Dir(), "untracked"), []byte("contents"), os.ModePerm))
	updateAndAssertClean()

	// 4. Committed locally.
	assert.NoError(t, ioutil.WriteFile(path.Join(c.Dir(), "somefile"), []byte("contents"), os.ModePerm))
	_, err = c.Git(ctx, "commit", "-a", "-m", "msg")
	assert.NoError(t, err)
	updateAndAssertClean()
}

func TestTempCheckout(t *testing.T) {
	ctx, gb, _ := setup(t)
	defer gb.Cleanup()

	c, err := NewTempCheckout(ctx, gb.Dir())
	assert.NoError(t, err)

	// Verify that we can run git commands.
	_, err = c.Git(ctx, "branch")
	assert.NoError(t, err)

	// Verify that we have a working copy.
	_, err = c.Git(ctx, "status")
	assert.NoError(t, err)

	// Delete the checkout.
	c.Delete()

	// Verify that the directory is gone.
	_, err = os.Stat(c.Dir())
	assert.True(t, os.IsNotExist(err))
}
