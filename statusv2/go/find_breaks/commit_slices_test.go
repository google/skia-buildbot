package find_breaks

import (
	"io/ioutil"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
)

// setupHelper is a shared function used for reducing boilerplate when setting
// up test inputs. The provided func is used to build the git repo which will
// be used by the test.
func setupHelper(t *testing.T, setup func(*git_testutils.GitBuilder)) (*repograph.Graph, func()) {
	testutils.MediumTest(t)

	gb := git_testutils.GitInit(t)
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	setup(gb)
	cleanup := func() {
		gb.Cleanup()
		testutils.RemoveAll(t, wd)
	}
	repo, err := repograph.NewGraph(gb.RepoUrl(), wd)
	assert.NoError(t, err)
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
	repo, cleanup := setupHelper(t, func(gb *git_testutils.GitBuilder) {
		ts := now.Add(-30 * time.Minute)
		a = gb.CommitGenAt("file", ts)

		ts = ts.Add(2 * time.Minute)
		b = gb.CommitGenAt("file", ts)

		ts = ts.Add(2 * time.Minute)
		c = gb.CommitGenAt("file", ts)

		ts = ts.Add(2 * time.Minute)
		d = gb.CommitGenAt("file", ts)

		ts = ts.Add(2 * time.Minute)
		e = gb.CommitGenAt("file", ts)
	})
	defer cleanup()

	// Make sure we get all of the commits in one slice.
	slices := commitSlices(repo, time.Time{}, now)
	assert.Equal(t, 1, len(slices))
	assert.Equal(t, 5, len(slices[0]))
	testutils.AssertDeepEqual(t, []string{a, b, c, d, e}, slices[0])

	// Make sure the timestamp cutoffs work.
	end := now.Add(-22 * time.Minute)
	start := now.Add(-30 * time.Minute)
	slices = commitSlices(repo, start, end)
	assert.Equal(t, 1, len(slices))
	assert.Equal(t, 4, len(slices[0]))
	testutils.AssertDeepEqual(t, []string{a, b, c, d}, slices[0])

	// Test the edges of the timestamp cutoffs.
	slices = commitSlices(repo, start.Add(2*time.Second), end.Add(-2*time.Second))
	assert.Equal(t, 1, len(slices))
	assert.Equal(t, 3, len(slices[0]))
	testutils.AssertDeepEqual(t, []string{b, c, d}, slices[0])

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
	repo, cleanup := setupHelper(t, func(gb *git_testutils.GitBuilder) {
		ts := time.Now().Add(-30 * time.Minute)
		a = gb.CommitGenAt("file", ts)

		ts = ts.Add(2 * time.Minute)
		b = gb.CommitGenAt("file", ts)

		ts = ts.Add(2 * time.Minute)
		gb.CreateBranchTrackBranch("otherBranch", "master")
		c = gb.CommitGenAt("file", ts)

		ts = ts.Add(2 * time.Minute)
		gb.CheckoutBranch("master")
		d = gb.CommitGenAt("file", ts)
	})
	defer cleanup()

	// Entire repo. We should get two slices.
	slices := commitSlices(repo, time.Time{}, time.Now())
	assert.Equal(t, 2, len(slices))
	assert.Equal(t, 3, len(slices[0]))
	assert.Equal(t, 3, len(slices[1]))
	testutils.AssertDeepEqual(t, []string{a, b, d}, slices[0])
	testutils.AssertDeepEqual(t, []string{a, b, c}, slices[1])
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
	repo, cleanup := setupHelper(t, func(gb *git_testutils.GitBuilder) {
		ts := time.Now().Add(-30 * time.Minute)
		a = gb.CommitGenAt("file", ts)

		ts = ts.Add(2 * time.Minute)
		gb.CreateOrphanBranch("branch2")
		gb.AddGen("file2")
		b = gb.CommitGenAt("file2", ts)

		ts = ts.Add(2 * time.Minute)
		gb.CheckoutBranch("master")
		c = gb.MergeBranch("branch2")
		_, err := git.GitDir(gb.Dir()).Git("branch", "-D", "branch2")
		assert.NoError(t, err)

		ts = ts.Add(2 * time.Minute)
		d = gb.CommitGenAt("file", ts)
	})
	defer cleanup()

	// Entire repo. We should get two slices.
	slices := commitSlices(repo, time.Time{}, time.Now())
	assert.Equal(t, 2, len(slices))
	assert.Equal(t, 3, len(slices[0]))
	assert.Equal(t, 3, len(slices[1]))
	testutils.AssertDeepEqual(t, []string{a, c, d}, slices[0])
	testutils.AssertDeepEqual(t, []string{b, c, d}, slices[1])
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
	repo, cleanup := setupHelper(t, func(gb *git_testutils.GitBuilder) {
		ts := time.Now().Add(-30 * time.Minute)
		a = gb.CommitGenAt("file", ts)

		ts = ts.Add(2 * time.Minute)
		b = gb.CommitGenAt("file", ts)

		ts = ts.Add(2 * time.Minute)
		c = gb.CommitGenAt("file", ts)

		ts = ts.Add(2 * time.Minute)
		gb.CreateBranchAtCommit("branch2", b)
		d = gb.CommitGenAt("file2", ts)

		ts = ts.Add(2 * time.Minute)
		gb.CheckoutBranch("master")
		e = gb.MergeBranch("branch2")
		_, err := git.GitDir(gb.Dir()).Git("branch", "-D", "branch2")
		assert.NoError(t, err)

		ts = ts.Add(2 * time.Minute)
		f = gb.CommitGenAt("file", ts)
	})
	defer cleanup()

	// Entire repo. We should get two slices.
	slices := commitSlices(repo, time.Time{}, time.Now())
	assert.Equal(t, 2, len(slices))
	assert.Equal(t, 5, len(slices[0]))
	assert.Equal(t, 5, len(slices[1]))
	testutils.AssertDeepEqual(t, []string{a, b, c, e, f}, slices[0])
	testutils.AssertDeepEqual(t, []string{a, b, d, e, f}, slices[1])
}
