package find_breaks

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
)

// setupHelper is a shared function used for reducing boilerplate when setting
// up test inputs. The provided func is used to build the git repo which will
// be used by the test.
func setupHelper(t *testing.T, setup func(context.Context, *git_testutils.GitBuilder)) (*repograph.Graph, func()) {
	testutils.LargeTest(t)
	ctx := context.Background()
	gb := git_testutils.GitInit(t, ctx)
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	setup(ctx, gb)
	cleanup := func() {
		gb.Cleanup()
		testutils.RemoveAll(t, wd)
	}
	repo, err := repograph.NewGraph(ctx, gb.RepoUrl(), wd)
	assert.NoError(t, err)
	assert.NoError(t, repo.Update(ctx))
	return repo, cleanup
}

// TestCommitSlices1 uses a simple, single-branch git repo:
//
//      e
//      |
//      d
//      |
//      c
//      |
//      b
//      |
//      a
//
func TestCommitSlices1(t *testing.T) {
	now := time.Now().Round(time.Second)
	var a, b, c, d, e string
	repo, cleanup := setupHelper(t, func(ctx context.Context, gb *git_testutils.GitBuilder) {
		ts := now.Add(-30 * time.Minute)
		a = gb.CommitGenAt(ctx, "file", ts)

		ts = ts.Add(2 * time.Minute)
		b = gb.CommitGenAt(ctx, "file", ts)

		ts = ts.Add(2 * time.Minute)
		c = gb.CommitGenAt(ctx, "file", ts)

		ts = ts.Add(2 * time.Minute)
		d = gb.CommitGenAt(ctx, "file", ts)

		ts = ts.Add(2 * time.Minute)
		e = gb.CommitGenAt(ctx, "file", ts)
	})
	defer cleanup()

	// Make sure we get all of the commits in one slice.
	slices := commitSlices(repo, time.Time{}, now)
	assert.Equal(t, 1, len(slices))
	assert.Equal(t, 5, len(slices[0]))
	deepequal.AssertDeepEqual(t, []string{a, b, c, d, e}, slices[0])

	// Make sure the timestamp cutoffs work.
	end := now.Add(-22 * time.Minute)
	start := now.Add(-30 * time.Minute)
	slices = commitSlices(repo, start, end)
	assert.Equal(t, 1, len(slices))
	assert.Equal(t, 4, len(slices[0]))
	deepequal.AssertDeepEqual(t, []string{a, b, c, d}, slices[0])

	// Test the edges of the timestamp cutoffs.
	slices = commitSlices(repo, start.Add(2*time.Second), end.Add(-2*time.Second))
	assert.Equal(t, 1, len(slices))
	assert.Equal(t, 3, len(slices[0]))
	deepequal.AssertDeepEqual(t, []string{b, c, d}, slices[0])

	// We shouldn't return empty slices.
	slices = commitSlices(repo, now.Add(30*time.Minute), now.Add(60*time.Minute))
	assert.Equal(t, 0, len(slices))
}

// TestCommitSlices2 uses a git repo with two diverging branches:
//
//      d   c
//      | /
//      b
//      |
//      a
//
func TestCommitSlices2(t *testing.T) {
	var a, b, c, d string
	repo, cleanup := setupHelper(t, func(ctx context.Context, gb *git_testutils.GitBuilder) {
		ts := time.Now().Add(-30 * time.Minute)
		a = gb.CommitGenAt(ctx, "file", ts)

		ts = ts.Add(2 * time.Minute)
		b = gb.CommitGenAt(ctx, "file", ts)

		ts = ts.Add(2 * time.Minute)
		gb.CreateBranchTrackBranch(ctx, "otherBranch", "master")
		c = gb.CommitGenAt(ctx, "file", ts)

		ts = ts.Add(2 * time.Minute)
		gb.CheckoutBranch(ctx, "master")
		d = gb.CommitGenAt(ctx, "file", ts)
	})
	defer cleanup()

	// Entire repo. We should get two slices.
	slices := commitSlices(repo, time.Time{}, time.Now())
	assert.Equal(t, 2, len(slices))
	assert.Equal(t, 3, len(slices[0]))
	assert.Equal(t, 3, len(slices[1]))
	deepequal.AssertDeepEqual(t, []string{a, b, d}, slices[0])
	deepequal.AssertDeepEqual(t, []string{a, b, c}, slices[1])
}

// TestCommitSlices3 uses a git repo with two merging branches:
//
//      d
//      |
//      c
//      | \
//      a   b
//
func TestCommitSlices3(t *testing.T) {
	var a, b, c, d string
	repo, cleanup := setupHelper(t, func(ctx context.Context, gb *git_testutils.GitBuilder) {
		ts := time.Now().Add(-30 * time.Minute)
		a = gb.CommitGenAt(ctx, "file", ts)

		ts = ts.Add(2 * time.Minute)
		gb.CreateOrphanBranch(ctx, "branch2")
		gb.AddGen(ctx, "file2")
		b = gb.CommitGenAt(ctx, "file2", ts)

		ts = ts.Add(2 * time.Minute)
		gb.CheckoutBranch(ctx, "master")
		c = gb.MergeBranch(ctx, "branch2")
		_, err := git.GitDir(gb.Dir()).Git(ctx, "branch", "-D", "branch2")
		assert.NoError(t, err)

		ts = ts.Add(2 * time.Minute)
		d = gb.CommitGenAt(ctx, "file", ts)
	})
	defer cleanup()

	// Entire repo. We should get two slices.
	slices := commitSlices(repo, time.Time{}, time.Now())
	assert.Equal(t, 2, len(slices))
	assert.Equal(t, 3, len(slices[0]))
	assert.Equal(t, 3, len(slices[1]))
	deepequal.AssertDeepEqual(t, []string{a, c, d}, slices[0])
	deepequal.AssertDeepEqual(t, []string{b, c, d}, slices[1])
}

// TestCommitSlices4 uses a git repo with a branch which diverges and then
// merges again:
//
//      f
//      |
//      e
//      | \
//      |   d
//      |   |
//      c   |
//      | /
//      b
//      |
//      a
//
func TestCommitSlices4(t *testing.T) {
	var a, b, c, d, e, f string
	repo, cleanup := setupHelper(t, func(ctx context.Context, gb *git_testutils.GitBuilder) {
		ts := time.Now().Add(-30 * time.Minute)
		a = gb.CommitGenAt(ctx, "file", ts)

		ts = ts.Add(2 * time.Minute)
		b = gb.CommitGenAt(ctx, "file", ts)

		ts = ts.Add(2 * time.Minute)
		c = gb.CommitGenAt(ctx, "file", ts)

		ts = ts.Add(2 * time.Minute)
		gb.CreateBranchAtCommit(ctx, "branch2", b)
		d = gb.CommitGenAt(ctx, "file2", ts)

		ts = ts.Add(2 * time.Minute)
		gb.CheckoutBranch(ctx, "master")
		e = gb.MergeBranch(ctx, "branch2")
		_, err := git.GitDir(gb.Dir()).Git(ctx, "branch", "-D", "branch2")
		assert.NoError(t, err)

		ts = ts.Add(2 * time.Minute)
		f = gb.CommitGenAt(ctx, "file", ts)
	})
	defer cleanup()

	// Entire repo. We should get two slices.
	slices := commitSlices(repo, time.Time{}, time.Now())
	assert.Equal(t, 2, len(slices))
	assert.Equal(t, 5, len(slices[0]))
	assert.Equal(t, 5, len(slices[1]))
	deepequal.AssertDeepEqual(t, []string{a, b, c, e, f}, slices[0])
	deepequal.AssertDeepEqual(t, []string{a, b, d, e, f}, slices[1])
}
