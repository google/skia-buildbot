package repograph

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"
	"testing"

	assert "github.com/stretchr/testify/require"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

// gitSetup initializes a Git repo in a temporary directory with some commits.
// Returns the path of the temporary directory, the Graph object associated with
// the repo, and a slice of the commits which were added.
//
// The repo layout looks like this:
//
// c1--c2------c4--c5--
//       \-c3-----/
func gitSetup(t *testing.T) (context.Context, *git_testutils.GitBuilder, *Graph, []*Commit, func()) {
	ctx := context.Background()
	g := git_testutils.GitInit(t, ctx)
	g.CommitGen(ctx, "myfile.txt")

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	repo, err := NewGraph(ctx, g.Dir(), tmp)
	assert.NoError(t, err)

	c1 := repo.Get("master")
	assert.NotNil(t, c1)
	assert.Equal(t, 0, len(c1.GetParents()))
	assert.False(t, util.TimeIsZero(c1.Timestamp))

	g.CommitGen(ctx, "myfile.txt")
	assert.NoError(t, repo.Update(ctx))
	c2 := repo.Get("master")
	assert.NotNil(t, c2)
	assert.Equal(t, 1, len(c2.GetParents()))
	assert.Equal(t, c1, c2.GetParents()[0])
	assert.Equal(t, []string{"master"}, repo.Branches())
	assert.False(t, util.TimeIsZero(c2.Timestamp))

	// Create a second branch.
	g.CreateBranchTrackBranch(ctx, "branch2", "origin/master")
	g.CommitGen(ctx, "anotherfile.txt")
	assert.NoError(t, repo.Update(ctx))
	c3 := repo.Get("branch2")
	assert.NotNil(t, c3)
	assert.Equal(t, c2, repo.Get("master"))
	assert.Equal(t, []string{"branch2", "master"}, repo.Branches())
	assert.False(t, util.TimeIsZero(c3.Timestamp))

	// Commit again to master.
	g.CheckoutBranch(ctx, "master")
	g.CommitGen(ctx, "myfile.txt")
	assert.NoError(t, repo.Update(ctx))
	assert.Equal(t, c3, repo.Get("branch2"))
	c4 := repo.Get("master")
	assert.NotNil(t, c4)
	assert.False(t, util.TimeIsZero(c4.Timestamp))

	// Merge branch1 into master.
	g.MergeBranch(ctx, "branch2")
	assert.NoError(t, repo.Update(ctx))
	assert.Equal(t, []string{"branch2", "master"}, repo.Branches())
	c5 := repo.Get("master")
	assert.NotNil(t, c5)
	assert.Equal(t, c3, repo.Get("branch2"))
	assert.False(t, util.TimeIsZero(c5.Timestamp))

	return ctx, g, repo, []*Commit{c1, c2, c3, c4, c5}, func() {
		g.Cleanup()
		testutils.RemoveAll(t, tmp)
	}
}

func TestGraph(t *testing.T) {
	testutils.MediumTest(t)
	ctx, g, repo, commits, cleanup := gitSetup(t)
	defer cleanup()

	c1 := commits[0]
	c2 := commits[1]
	c3 := commits[2]
	c4 := commits[3]
	c5 := commits[4]

	// Trace commits back to the beginning of time.
	assert.Equal(t, []*Commit{c4, c3}, c5.GetParents())
	assert.Equal(t, []*Commit{c2}, c4.GetParents())
	assert.Equal(t, []*Commit{c1}, c2.GetParents())
	assert.Equal(t, []*Commit{}, c1.GetParents())
	assert.Equal(t, []*Commit{c2}, c3.GetParents())

	// Ensure that we can start in an empty dir and check out from scratch properly.
	tmp2, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp2)
	repo2, err := NewGraph(ctx, g.Dir(), tmp2)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, repo.Branches(), repo2.Branches())
	m1 := repo.Get("master")
	m2 := repo2.Get("master")
	// These will confuse AssertDeepEqual.
	m1.repo = nil
	m2.repo = nil
	testutils.AssertDeepEqual(t, m1, m2)
}

func TestSerialize(t *testing.T) {
	testutils.MediumTest(t)
	ctx, g, repo, _, cleanup := gitSetup(t)
	defer cleanup()

	repo2, err := NewGraph(ctx, g.Dir(), path.Dir(repo.repo.Dir()))
	assert.NoError(t, err)

	testutils.AssertDeepEqual(t, repo, repo2)
}

func TestRecurse(t *testing.T) {
	testutils.LargeTest(t)
	_, _, repo, commits, cleanup := gitSetup(t)
	defer cleanup()

	c1 := commits[0]
	c2 := commits[1]
	c3 := commits[2]
	c4 := commits[3]
	c5 := commits[4]

	// Get the list of commits using head.Recurse(). Ensure that we get all
	// of the commits but don't get any duplicates.
	head := repo.Get("master")
	assert.NotNil(t, head)
	gotCommits := map[*Commit]bool{}
	assert.NoError(t, head.Recurse(func(c *Commit) (bool, error) {
		assert.False(t, gotCommits[c])
		gotCommits[c] = true
		return true, nil
	}))
	assert.Equal(t, len(commits), len(gotCommits))
	for _, c := range commits {
		assert.True(t, gotCommits[c])
	}

	// Verify that we properly return early when the passed-in function
	// return false.
	gotCommits = map[*Commit]bool{}
	assert.NoError(t, head.Recurse(func(c *Commit) (bool, error) {
		gotCommits[c] = true
		if c == c3 || c == c4 {
			return false, nil
		}
		return true, nil
	}))
	assert.False(t, gotCommits[c1])
	assert.False(t, gotCommits[c2])

	// Verify that we properly exit immediately when the passed-in function
	// returns an error.
	gotCommits = map[*Commit]bool{}
	assert.Error(t, head.Recurse(func(c *Commit) (bool, error) {
		gotCommits[c] = true
		if c == c4 {
			return false, fmt.Errorf("STOP!")
		}
		return true, nil
	}))
	assert.False(t, gotCommits[c1])
	assert.False(t, gotCommits[c2])
	assert.False(t, gotCommits[c3])
	assert.True(t, gotCommits[c4])
	assert.True(t, gotCommits[c5])
}

func TestRecurseAllBranches(t *testing.T) {
	testutils.LargeTest(t)
	ctx, g, repo, commits, cleanup := gitSetup(t)
	defer cleanup()

	c1 := commits[0]
	c2 := commits[1]
	c3 := commits[2]
	c4 := commits[3]

	test := func() {
		gotCommits := map[*Commit]bool{}
		assert.NoError(t, repo.RecurseAllBranches(func(c *Commit) (bool, error) {
			assert.False(t, gotCommits[c])
			gotCommits[c] = true
			return true, nil
		}))
		assert.Equal(t, len(commits), len(gotCommits))
		for _, c := range commits {
			assert.True(t, gotCommits[c])
		}
	}

	// Get the list of commits using head.RecurseAllBranches(). Ensure that
	// we get all of the commits but don't get any duplicates.
	test()

	// The above used only one branch. Add a branch and ensure that we see
	// its commits too.
	g.CreateBranchTrackBranch(ctx, "mybranch", "origin/master")
	g.CommitGen(ctx, "anotherfile.txt")
	assert.NoError(t, repo.Update(ctx))
	c := repo.Get("mybranch")
	assert.NotNil(t, c)
	commits = append(commits, c)
	test()

	// Verify that we don't revisit a branch whose HEAD is an ancestor of
	// a different branch HEAD.
	g.CreateBranchAtCommit(ctx, "ancestorbranch", c3.Hash)
	assert.NoError(t, repo.Update(ctx))
	test()

	// Verify that we still stop recursion when requested.
	gotCommits := map[*Commit]bool{}
	assert.NoError(t, repo.RecurseAllBranches(func(c *Commit) (bool, error) {
		gotCommits[c] = true
		if c == c3 || c == c4 {
			return false, nil
		}
		return true, nil
	}))
	assert.False(t, gotCommits[c1])
	assert.False(t, gotCommits[c2])

	// Verify that we error out properly.
	gotCommits = map[*Commit]bool{}
	assert.Error(t, repo.RecurseAllBranches(func(c *Commit) (bool, error) {
		gotCommits[c] = true
		// Because of nondeterministic map iteration and the added
		// branches, we have to halt way back at c2 in order to have
		// a sane, deterministic test case.
		if c == c2 {
			return false, fmt.Errorf("STOP!")
		}
		return true, nil
	}))
	assert.False(t, gotCommits[c1])
	assert.True(t, gotCommits[c2])
}

func TestFindCommit(t *testing.T) {
	testutils.LargeTest(t)
	_, g1, repo1, commits1, cleanup1 := gitSetup(t)
	defer cleanup1()
	_, g2, repo2, commits2, cleanup2 := gitSetup(t)
	defer cleanup2()

	m := Map{
		g1.Dir(): repo1,
		g2.Dir(): repo2,
	}

	tc := []struct {
		hash string
		url  string
		repo *Graph
		err  bool
	}{
		{
			hash: commits1[0].Hash,
			url:  g1.Dir(),
			repo: repo1,
			err:  false,
		},
		{
			hash: commits1[len(commits1)-1].Hash,
			url:  g1.Dir(),
			repo: repo1,
			err:  false,
		},
		{
			hash: commits2[0].Hash,
			url:  g2.Dir(),
			repo: repo2,
			err:  false,
		},
		{
			hash: commits2[len(commits2)-1].Hash,
			url:  g2.Dir(),
			repo: repo2,
			err:  false,
		},
		{
			hash: "",
			err:  true,
		},
		{
			hash: "abcdef",
			err:  true,
		},
	}
	for _, c := range tc {
		commit, url, repo, err := m.FindCommit(c.hash)
		if c.err {
			assert.Error(t, err)
		} else {
			assert.Nil(t, err)
			assert.NotNil(t, commit)
			assert.Equal(t, c.hash, commit.Hash)
			assert.Equal(t, c.url, url)
			assert.Equal(t, c.repo, repo)
		}
	}
}

func TestUpdateHistoryChanged(t *testing.T) {
	testutils.LargeTest(t)
	ctx, g, repo, commits, cleanup := gitSetup(t)
	defer cleanup()

	// c3 is the one commit on branch2.
	c3 := repo.Get("branch2")
	assert.NotNil(t, c3)
	assert.Equal(t, c3, commits[2]) // c3 from setup()

	// Change branch 2 to be based at c4 with one commit, c6.
	g.CheckoutBranch(ctx, "branch2")
	g.Reset(ctx, "--hard", commits[3].Hash) // c4 from setup()
	f := "myfile"
	c6hash := g.CommitGen(ctx, f)

	assert.NoError(t, repo.Update(ctx))
	c6 := repo.Get("branch2")
	assert.NotNil(t, c6)
	assert.Equal(t, c6hash, c6.Hash)

	// Ensure that c3 is not reachable from c6.
	anc, err := repo.repo.IsAncestor(ctx, c3.Hash, c6.Hash)
	assert.NoError(t, err)
	assert.False(t, anc)

	assert.NoError(t, c6.Recurse(func(c *Commit) (bool, error) {
		assert.NotEqual(t, c, c3)
		return true, nil
	}))
}

func TestGetNewCommits(t *testing.T) {
	testutils.LargeTest(t)
	ctx, g, repo, commits, cleanup := gitSetup(t)
	defer cleanup()

	// No supplied branch heads, all commits are new.
	newCommits, err := repo.GetNewCommits(nil)
	assert.NoError(t, err)
	assert.Equal(t, len(newCommits), len(commits))

	// No new commits.
	branchHeads := repo.BranchHeads()
	newCommits, err = repo.GetNewCommits(branchHeads)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(newCommits))
	branchHeads = repo.BranchHeads()

	// Add a few commits, ensure that they get picked up.
	g.CheckoutBranch(ctx, "master")
	f := "myfile"
	new1 := g.CommitGen(ctx, f)
	new2 := g.CommitGen(ctx, f)
	assert.NoError(t, repo.Update(ctx))
	newCommits, err = repo.GetNewCommits(branchHeads)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(newCommits))
	assert.Equal(t, new1, newCommits[1].Hash)
	assert.Equal(t, new2, newCommits[0].Hash)
	branchHeads = repo.BranchHeads()

	// Add commits on both branches, ensure that they get picked up.
	new1 = g.CommitGen(ctx, f)
	g.CheckoutBranch(ctx, "branch2")
	new2 = g.CommitGen(ctx, "file2")
	assert.NoError(t, repo.Update(ctx))
	newCommits, err = repo.GetNewCommits(branchHeads)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(newCommits))
	if newCommits[0].Hash == new1 {
		assert.Equal(t, new2, newCommits[1].Hash)
	} else {
		assert.Equal(t, new1, newCommits[1].Hash)
		assert.Equal(t, new2, newCommits[0].Hash)
	}
	branchHeads = repo.BranchHeads()

	// Add a new branch. Make sure that we don't get duplicate commits.
	g.CheckoutBranch(ctx, "master")
	g.CreateBranchTrackBranch(ctx, "branch3", "master")
	assert.NoError(t, repo.Update(ctx))
	newCommits, err = repo.GetNewCommits(branchHeads)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(newCommits))
	assert.Equal(t, 3, len(repo.BranchHeads()))

	// Make sure we get no duplicates if the branch heads aren't the same.
	g.Reset(ctx, "--hard", "master^")
	assert.NoError(t, repo.Update(ctx))
	newCommits, err = repo.GetNewCommits(branchHeads)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(newCommits))
	assert.Equal(t, 3, len(repo.BranchHeads()))
}
