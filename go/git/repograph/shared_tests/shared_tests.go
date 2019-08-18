package shared_tests

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

// RepoImplRefresher is an interface used for testing which notifies a RepoImpl
// that there are new commits available.
type RepoImplRefresher interface {
	Refresh(...*vcsinfo.LongCommit)
}

// CommonSetup performs common setup.
func CommonSetup(t sktest.TestingT) (context.Context, *git_testutils.GitBuilder, func()) {
	ctx := context.Background()
	g := git_testutils.GitInit(t, ctx)
	return ctx, g, g.Cleanup
}

// GitSetup initializes a Git repo in a temporary directory with some commits.
// Returns the path of the temporary directory, the Graph object associated with
// the repo, and a slice of the commits which were added.
//
// The repo layout looks like this:
//
// c1--c2------c4--c5--
//       \-c3-----/
func GitSetup(t sktest.TestingT, ctx context.Context, g *git_testutils.GitBuilder, repo *repograph.Graph, rf RepoImplRefresher) []*repograph.Commit {
	c1hash := g.CommitGen(ctx, "myfile.txt")
	c1details, err := git.GitDir(g.Dir()).Details(ctx, c1hash)
	assert.NoError(t, err)
	rf.Refresh(c1details)
	assert.NoError(t, repo.Update(ctx))

	c1 := repo.Get("master")
	assert.NotNil(t, c1)
	assert.Equal(t, 0, len(c1.GetParents()))
	assert.False(t, util.TimeIsZero(c1.Timestamp))

	c2hash := g.CommitGen(ctx, "myfile.txt")
	c2details, err := git.GitDir(g.Dir()).Details(ctx, c2hash)
	assert.NoError(t, err)
	rf.Refresh(c2details)
	assert.NoError(t, repo.Update(ctx))
	c2 := repo.Get("master")
	assert.NotNil(t, c2)
	assert.Equal(t, 1, len(c2.GetParents()))
	assert.Equal(t, c1, c2.GetParents()[0])
	assert.Equal(t, []string{"master"}, repo.Branches())
	assert.False(t, util.TimeIsZero(c2.Timestamp))

	// Create a second branch.
	g.CreateBranchTrackBranch(ctx, "branch2", "origin/master")
	c3hash := g.CommitGen(ctx, "anotherfile.txt")
	c3details, err := git.GitDir(g.Dir()).Details(ctx, c3hash)
	assert.NoError(t, err)
	rf.Refresh(c3details)
	assert.NoError(t, repo.Update(ctx))
	c3 := repo.Get("branch2")
	assert.NotNil(t, c3)
	assert.Equal(t, c2, repo.Get("master"))
	assert.Equal(t, []string{"branch2", "master"}, repo.Branches())
	assert.False(t, util.TimeIsZero(c3.Timestamp))

	// Commit again to master.
	g.CheckoutBranch(ctx, "master")
	c4hash := g.CommitGen(ctx, "myfile.txt")
	c4details, err := git.GitDir(g.Dir()).Details(ctx, c4hash)
	assert.NoError(t, err)
	rf.Refresh(c4details)
	assert.NoError(t, repo.Update(ctx))
	assert.Equal(t, c3, repo.Get("branch2"))
	c4 := repo.Get("master")
	assert.NotNil(t, c4)
	assert.False(t, util.TimeIsZero(c4.Timestamp))

	// Merge branch1 into master.
	c5hash := g.MergeBranch(ctx, "branch2")
	c5details, err := git.GitDir(g.Dir()).Details(ctx, c5hash)
	assert.NoError(t, err)
	rf.Refresh(c5details)
	assert.NoError(t, repo.Update(ctx))
	assert.Equal(t, []string{"branch2", "master"}, repo.Branches())
	c5 := repo.Get("master")
	assert.NotNil(t, c5)
	assert.Equal(t, c3, repo.Get("branch2"))
	assert.False(t, util.TimeIsZero(c5.Timestamp))

	return []*repograph.Commit{c1, c2, c3, c4, c5}
}

// Assert that the given Commits are in reverse topological order.
func AssertTopoSorted(t sktest.TestingT, commits []*repograph.Commit) {
	// Collect all commits in a map so we can quickly check for their
	// presence in the commits slice.
	commitsMap := make(map[*repograph.Commit]bool, len(commits))
	for _, c := range commits {
		// Assert that each parent is not yet visited.
		parents := c.GetParents()
		for _, p := range parents {
			assert.False(t, commitsMap[p])
		}
		commitsMap[c] = true
	}

	// Ensure that we don't mix lines of history. Practically this means,
	// for each commit:
	// - If it has one parent and its parent has one child,
	//   they are adjacent in the sorted slice.
	// - If it has multiple parents, it is adjacent to one of them.
	// - If it has multiple children, it is adjacent to one of them.

	// Create a mapping from commits to their children.
	children := make(map[*repograph.Commit]map[*repograph.Commit]bool, len(commits))
	for _, c := range commits {
		for _, p := range c.GetParents() {
			// Don't include parents which aren't in the slice.
			if !commitsMap[p] {
				continue
			}
			subMap, ok := children[p]
			if !ok {
				subMap = map[*repograph.Commit]bool{}
				children[p] = subMap
			}
			subMap[c] = true
		}
	}

	// Verify that the above cases are true for each commit in the slice.
	for idx, c := range commits {
		// If the commit has one parent, and its parent has one child,
		// they should be adjacent.
		if len(c.GetParents()) == 1 && len(children[c.GetParents()[0]]) == 1 {
			// Expect that we're not at the end of the commits slice
			// since the parent should be listed after the current
			// commit.
			assert.True(t, len(commits) > idx+1)
			assert.Equal(t, c.GetParents()[0], commits[idx+1])
		}

		// If the commit has multiple parents, it should be adjacent to
		// one of them.
		if len(c.GetParents()) > 1 {
			expectParentAdjacent := false
			parentIsAdjacent := false
			for _, p := range c.GetParents() {
				// Only include parents which are in the slice.
				if commitsMap[p] {
					expectParentAdjacent = true
					if len(commits) > idx+1 && commits[idx+1] == p {
						parentIsAdjacent = true
						break
					}
				}
			}
			assert.Equal(t, expectParentAdjacent, parentIsAdjacent)
		}

		// If the commit has multiple children, it should be adjacent
		// to one of them.
		if len(children[c]) > 1 {
			assert.True(t, idx > 0)
			childIsAdjacent := false
			for child := range children[c] {
				if commits[idx-1] == child {
					childIsAdjacent = true
					break
				}
			}
			assert.True(t, childIsAdjacent)
		}
	}
}

// Assert that the given commit hashses are in reverse topological order.
func assertHashesTopoSorted(t sktest.TestingT, repo *repograph.Graph, hashes []string) {
	commits := make([]*repograph.Commit, 0, len(hashes))
	for _, hash := range hashes {
		commits = append(commits, repo.Get(hash))
	}
	AssertTopoSorted(t, commits)
}

func TestGraphWellFormed(t sktest.TestingT, ctx context.Context, g *git_testutils.GitBuilder, repo *repograph.Graph, rf RepoImplRefresher) {
	commits := GitSetup(t, ctx, g, repo, rf)

	c1 := commits[0]
	c2 := commits[1]
	c3 := commits[2]
	c4 := commits[3]
	c5 := commits[4]

	// Trace commits back to the beginning of time.
	assert.Equal(t, []*repograph.Commit{c4, c3}, c5.GetParents())
	assert.Equal(t, []*repograph.Commit{c2}, c4.GetParents())
	assert.Equal(t, []*repograph.Commit{c1}, c2.GetParents())
	assert.Equal(t, []*repograph.Commit(nil), c1.GetParents())
	assert.Equal(t, []*repograph.Commit{c2}, c3.GetParents())

	// Assert that each of the commits has the correct index.
	assert.Equal(t, 0, c1.Index)
	assert.Equal(t, 1, c2.Index)
	assert.Equal(t, 2, c3.Index)
	assert.Equal(t, 2, c4.Index)
	assert.Equal(t, 3, c5.Index)

	// Ensure that we can start in an empty dir and check out from scratch properly.
	tmp2, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp2)
	repo2, err := repograph.NewLocalGraph(ctx, g.Dir(), tmp2)
	assert.NoError(t, err)
	assert.NoError(t, repo2.Update(ctx))
	deepequal.AssertDeepEqual(t, repo.Branches(), repo2.Branches())
	m1 := repo.Get("master")
	m2 := repo2.Get("master")
	// Different implementations may or may not track branch info.
	for _, c := range repo2.GetAll() {
		c.Branches = repo.Get(c.Hash).Branches
	}
	deepequal.AssertDeepEqual(t, m1, m2)
}

func TestRecurse(t sktest.TestingT, ctx context.Context, g *git_testutils.GitBuilder, repo *repograph.Graph, rf RepoImplRefresher) {
	commits := GitSetup(t, ctx, g, repo, rf)

	c1 := commits[0]
	c2 := commits[1]
	c3 := commits[2]
	c4 := commits[3]
	c5 := commits[4]

	// Get the list of commits using head.Recurse(). Ensure that we get all
	// of the commits but don't get any duplicates.
	head := repo.Get("master")
	assert.NotNil(t, head)
	gotCommits := map[*repograph.Commit]bool{}
	assert.NoError(t, head.Recurse(func(c *repograph.Commit) error {
		assert.False(t, gotCommits[c])
		gotCommits[c] = true
		return nil
	}))
	assert.Equal(t, len(commits), len(gotCommits))
	for _, c := range commits {
		assert.True(t, gotCommits[c])
	}
	// AllCommits is the same thing as the above.
	allCommits, err := head.AllCommits()
	assert.NoError(t, err)
	assert.Equal(t, len(allCommits), len(gotCommits))

	// Verify that we properly return early when the passed-in function
	// return false.
	gotCommits = map[*repograph.Commit]bool{}
	assert.NoError(t, head.Recurse(func(c *repograph.Commit) error {
		gotCommits[c] = true
		if c == c3 || c == c4 {
			return repograph.ErrStopRecursing
		}
		return nil
	}))
	assert.False(t, gotCommits[c1])
	assert.False(t, gotCommits[c2])

	// Verify that we properly exit immediately when the passed-in function
	// returns an error.
	gotCommits = map[*repograph.Commit]bool{}
	assert.Error(t, head.Recurse(func(c *repograph.Commit) error {
		gotCommits[c] = true
		if c == c4 {
			return fmt.Errorf("STOP!")
		}
		return nil
	}))
	assert.False(t, gotCommits[c1])
	assert.False(t, gotCommits[c2])
	assert.False(t, gotCommits[c3])
	assert.True(t, gotCommits[c4])
	assert.True(t, gotCommits[c5])
}

func TestRecurseAllBranches(t sktest.TestingT, ctx context.Context, g *git_testutils.GitBuilder, repo *repograph.Graph, rf RepoImplRefresher) {
	commits := GitSetup(t, ctx, g, repo, rf)

	c1 := commits[0]
	c2 := commits[1]
	c3 := commits[2]
	c4 := commits[3]

	test := func() {
		gotCommits := map[*repograph.Commit]bool{}
		assert.NoError(t, repo.RecurseAllBranches(func(c *repograph.Commit) error {
			assert.False(t, gotCommits[c])
			gotCommits[c] = true
			return nil
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
	c5 := g.CommitGen(ctx, "anotherfile.txt")
	c5details, err := git.GitDir(g.Dir()).Details(ctx, c5)
	assert.NoError(t, err)
	rf.Refresh(c5details)
	assert.NoError(t, repo.Update(ctx))
	c := repo.Get("mybranch")
	assert.NotNil(t, c)
	commits = append(commits, c)
	test()

	// Verify that we don't revisit a branch whose HEAD is an ancestor of
	// a different branch HEAD.
	g.CreateBranchAtCommit(ctx, "ancestorbranch", c3.Hash)
	rf.Refresh()
	assert.NoError(t, repo.Update(ctx))
	test()

	// Verify that we still stop recursion when requested.
	gotCommits := map[*repograph.Commit]bool{}
	assert.NoError(t, repo.RecurseAllBranches(func(c *repograph.Commit) error {
		gotCommits[c] = true
		if c == c3 || c == c4 {
			return repograph.ErrStopRecursing
		}
		return nil
	}))
	assert.False(t, gotCommits[c1])
	assert.False(t, gotCommits[c2])

	// Verify that we error out properly.
	gotCommits = map[*repograph.Commit]bool{}
	assert.Error(t, repo.RecurseAllBranches(func(c *repograph.Commit) error {
		gotCommits[c] = true
		// Because of nondeterministic map iteration and the added
		// branches, we have to halt way back at c2 in order to have
		// a sane, deterministic test case.
		if c == c2 {
			return fmt.Errorf("STOP!")
		}
		return nil
	}))
	assert.False(t, gotCommits[c1])
	assert.True(t, gotCommits[c2])
}

func TestLogLinear(t sktest.TestingT, ctx context.Context, g *git_testutils.GitBuilder, repo *repograph.Graph, rf RepoImplRefresher) {
	commits := GitSetup(t, ctx, g, repo, rf)

	c1 := commits[0]
	c2 := commits[1]
	c3 := commits[2]
	c4 := commits[3]
	c5 := commits[4]

	gitdir := git.GitDir(g.Dir())
	test := func(from, to string, checkAgainstGit bool, expect ...*repograph.Commit) {
		if checkAgainstGit {
			// Ensure that our expectations match actual git results.
			cmd := []string{"--first-parent"}
			if from == "" {
				cmd = append(cmd, to)
			} else {
				cmd = append(cmd, "--ancestry-path", fmt.Sprintf("%s..%s", from, to))
			}
			hashes, err := gitdir.RevList(ctx, cmd...)
			assert.NoError(t, err)
			assert.Equal(t, len(hashes), len(expect))
			for i, h := range hashes {
				assert.Equal(t, h, expect[i].Hash)
			}
		}

		// Ensure that we get the expected results from the Graph.
		actual, err := repo.LogLinear(from, to)
		assert.NoError(t, err)
		deepequal.AssertDeepEqual(t, expect, actual)
	}

	// Get the full linear history from c5.
	test("", c5.Hash, true, c5, c4, c2, c1)
	// Get the linear history from c1 to c5. Like "git log", we don't
	// include the "from" commit in the results.
	test(c1.Hash, c5.Hash, true, c5, c4, c2)
	// c3 is not reachable via first-parents from c5. For some reason, git
	// actually returns c5.Hash, even though c5 has c4 as its first parent.
	// Ignore the check against git in this case.
	test(c3.Hash, c5.Hash, false)
}

func TestUpdateHistoryChanged(t sktest.TestingT, ctx context.Context, g *git_testutils.GitBuilder, repo *repograph.Graph, rf RepoImplRefresher) {
	commits := GitSetup(t, ctx, g, repo, rf)

	// c3 is the one commit on branch2.
	c3 := repo.Get("branch2")
	assert.NotNil(t, c3)
	assert.Equal(t, c3, commits[2]) // c3 from setup()

	// Change branch 2 to be based at c4 with one commit, c6.
	g.CheckoutBranch(ctx, "branch2")
	g.Reset(ctx, "--hard", commits[3].Hash) // c4 from setup()
	f := "myfile"
	c6hash := g.CommitGen(ctx, f)
	c6details, err := git.GitDir(g.Dir()).Details(ctx, c6hash)
	assert.NoError(t, err)
	rf.Refresh(c6details)
	assert.NoError(t, repo.Update(ctx))
	c6 := repo.Get("branch2")
	assert.NotNil(t, c6)
	assert.Equal(t, c6hash, c6.Hash)

	// Ensure that c3 is not reachable from c6.
	anc, err := repo.IsAncestor(c3.Hash, c6.Hash)
	assert.NoError(t, err)
	assert.False(t, anc)

	assert.NoError(t, c6.Recurse(func(c *repograph.Commit) error {
		assert.NotEqual(t, c, c3)
		return nil
	}))

	// Create a new branch, add some commits. Reset the old branches to
	// orphan some commits. Ensure that those are removed from the graph.
	g.CreateBranchAtCommit(ctx, "new", commits[0].Hash)
	c7 := g.CommitGen(ctx, "blah")
	c8 := g.CommitGen(ctx, "blah")
	g.CheckoutBranch(ctx, "master")
	g.Reset(ctx, "--hard", c8)
	c7details, err := git.GitDir(g.Dir()).Details(ctx, c7)
	assert.NoError(t, err)
	c8details, err := git.GitDir(g.Dir()).Details(ctx, c8)
	assert.NoError(t, err)
	rf.Refresh(c7details, c8details)
	assert.NoError(t, repo.Update(ctx))
	assert.NotNil(t, repo.Get(c7))
	assert.NotNil(t, repo.Get(c8))
	master := repo.Get("master")
	assert.NotNil(t, master)
	assert.Equal(t, c8, master.Hash)
	assert.NoError(t, repo.RecurseAllBranches(func(c *repograph.Commit) error {
		assert.NotEqual(t, c.Hash, commits[2].Hash)
		assert.NotEqual(t, c.Hash, commits[4].Hash)
		return nil
	}))
	assert.Nil(t, repo.Get(commits[2].Hash)) // Should be orphaned now.
	assert.Nil(t, repo.Get(commits[4].Hash)) // Should be orphaned now.

	// Delete branch2. Ensure that c6 disappears.
	g.UpdateRef(ctx, "-d", "refs/heads/branch2")
	rf.Refresh()
	assert.NoError(t, repo.Update(ctx))
	assert.Nil(t, repo.Get("branch2"))
	assert.Nil(t, repo.Get(c6hash))

	// Rewind a branch. Make sure that we correctly handle this case.
	removed := []string{c7, c8}
	for _, c := range removed {
		assert.NotNil(t, repo.Get(c))
	}
	g.UpdateRef(ctx, "refs/heads/master", commits[0].Hash)
	g.UpdateRef(ctx, "refs/heads/new", commits[0].Hash)
	rf.Refresh()
	assert.NoError(t, repo.Update(ctx))
	assert.NotNil(t, repo.Get("master"))
	assert.NotNil(t, repo.Get(commits[0].Hash))
	assert.Equal(t, commits[0].Hash, repo.Get(commits[0].Hash).Hash)
	for _, c := range removed {
		assert.Nil(t, repo.Get(c))
	}
}

func TestUpdateAndReturnCommitDiffs(t sktest.TestingT, ctx context.Context, g *git_testutils.GitBuilder, repo *repograph.Graph, rf RepoImplRefresher) {
	GitSetup(t, ctx, g, repo, rf)

	// The repo has commits, but GitSetup has already run Update(), so
	// there's nothing new.
	rf.Refresh()
	added, removed, err := repo.UpdateAndReturnCommitDiffs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(added), 0)
	assert.Equal(t, len(removed), 0)

	// No new commits.
	rf.Refresh()
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(added))
	assert.Equal(t, len(removed), 0)

	// Add a few commits, ensure that they get picked up.
	g.CheckoutBranch(ctx, "master")
	f := "myfile"
	new1 := g.CommitGen(ctx, f)
	new2 := g.CommitGen(ctx, f)
	new1details, err := git.GitDir(g.Dir()).Details(ctx, new1)
	assert.NoError(t, err)
	new2details, err := git.GitDir(g.Dir()).Details(ctx, new2)
	assert.NoError(t, err)
	rf.Refresh(new1details, new2details)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(added))
	assert.Equal(t, len(removed), 0)
	if added[0].Hash == new1 {
		assert.Equal(t, new2, added[1].Hash)
	} else {
		assert.Equal(t, new1, added[1].Hash)
		assert.Equal(t, new2, added[0].Hash)
	}

	// Add commits on both branches, ensure that they get picked up.
	new1 = g.CommitGen(ctx, f)
	g.CheckoutBranch(ctx, "branch2")
	new2 = g.CommitGen(ctx, "file2")
	new1details, err = git.GitDir(g.Dir()).Details(ctx, new1)
	assert.NoError(t, err)
	new2details, err = git.GitDir(g.Dir()).Details(ctx, new2)
	assert.NoError(t, err)
	rf.Refresh(new1details, new2details)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(added))
	assert.Equal(t, len(removed), 0)
	if added[0].Hash == new1 {
		assert.Equal(t, new2, added[1].Hash)
	} else {
		assert.Equal(t, new1, added[1].Hash)
		assert.Equal(t, new2, added[0].Hash)
	}

	// Add a new branch. Make sure that we don't get duplicate commits.
	g.CheckoutBranch(ctx, "master")
	g.CreateBranchTrackBranch(ctx, "branch3", "master")
	rf.Refresh()
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(added))
	assert.Equal(t, len(removed), 0)
	assert.Equal(t, 3, len(repo.BranchHeads()))

	// Make sure we get no duplicates if the branch heads aren't the same.
	g.Reset(ctx, "--hard", "master^")
	rf.Refresh()
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(added))
	assert.Equal(t, len(removed), 0)

	// Create a new branch.
	g.CheckoutBranch(ctx, "master")
	g.CreateBranchTrackBranch(ctx, "branch4", "master")
	rf.Refresh()
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(added))
	assert.Equal(t, len(removed), 0)

	// Add a commit on the new branch.
	new1 = g.CommitGen(ctx, f)
	new1details, err = git.GitDir(g.Dir()).Details(ctx, new1)
	assert.NoError(t, err)
	rf.Refresh(new1details)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(added))
	assert.Equal(t, len(removed), 0)
	assert.Equal(t, new1, added[0].Hash)

	// Add a merge commit. Because there were no commits on master in
	// between, the master branch head moves and now has the same hash as
	// "branch4", which means that there aren't any new commits even though
	// the branch head changed.
	g.CheckoutBranch(ctx, "master")
	mergeCommit := g.MergeBranch(ctx, "branch4")
	mergeCommitDetails, err := git.GitDir(g.Dir()).Details(ctx, mergeCommit)
	assert.NoError(t, err)
	rf.Refresh(mergeCommitDetails)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(added))
	assert.Equal(t, len(removed), 0)
	assert.Equal(t, mergeCommit, new1)

	// Create a new branch.
	g.CheckoutBranch(ctx, "master")
	g.CreateBranchTrackBranch(ctx, "branch5", "master")
	rf.Refresh()
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(added))
	assert.Equal(t, len(removed), 0)

	// Add a commit on the new branch.
	new1 = g.CommitGen(ctx, f)
	new1details, err = git.GitDir(g.Dir()).Details(ctx, new1)
	assert.NoError(t, err)
	rf.Refresh(new1details)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(added))
	assert.Equal(t, len(removed), 0)
	assert.Equal(t, new1, added[0].Hash)

	// Add a commit on the master branch.
	g.CheckoutBranch(ctx, "master")
	new1 = g.CommitGen(ctx, "file2")
	new1details, err = git.GitDir(g.Dir()).Details(ctx, new1)
	assert.NoError(t, err)
	rf.Refresh(new1details)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(added))
	assert.Equal(t, len(removed), 0)
	assert.Equal(t, new1, added[0].Hash)

	// Merge "branch5" into master. This should result in a new commit.
	mergeCommit = g.MergeBranch(ctx, "branch5")
	mergeCommitDetails, err = git.GitDir(g.Dir()).Details(ctx, mergeCommit)
	assert.NoError(t, err)
	rf.Refresh(mergeCommitDetails)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(added))
	assert.Equal(t, len(removed), 0)
	assert.Equal(t, mergeCommit, added[0].Hash)

	// Reset all branches to the initial commit.
	var c0 string
	assert.NoError(t, repo.Get("master").Recurse(func(c *repograph.Commit) error {
		if len(c.Parents) == 0 {
			c0 = c.Hash
			return repograph.ErrStopRecursing
		}
		return nil
	}))
	for _, branch := range []string{"master", "branch2", "branch3", "branch4", "branch5"} {
		g.CheckoutBranch(ctx, branch)
		g.Reset(ctx, "--hard", c0)
	}
	rf.Refresh()
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(added))
	assert.Equal(t, 12, len(removed))

	// Add some new commits, some of which share an ancestor. Ensure that
	// the added list doesn't double-count the shared commit.
	g.CheckoutBranch(ctx, "master")
	shared := g.CommitGen(ctx, "f")
	master := g.CommitGen(ctx, "f")
	g.CheckoutBranch(ctx, "branch2")
	g.Reset(ctx, "--hard", shared)
	branch2 := g.CommitGen(ctx, "f2")
	sharedDetails, err := git.GitDir(g.Dir()).Details(ctx, shared)
	masterDetails, err := git.GitDir(g.Dir()).Details(ctx, master)
	branch2Details, err := git.GitDir(g.Dir()).Details(ctx, branch2)
	rf.Refresh(sharedDetails, masterDetails, branch2Details)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(added))
	assert.Equal(t, 0, len(removed))
}

func TestRevList(t sktest.TestingT, ctx context.Context, gb *git_testutils.GitBuilder, repo *repograph.Graph, rf RepoImplRefresher) {
	commits := git_testutils.GitSetup(ctx, gb)
	tmpDir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)
	d1 := path.Join(tmpDir, "1")
	assert.NoError(t, os.Mkdir(d1, os.ModePerm))
	co, err := git.NewCheckout(ctx, gb.Dir(), d1)
	assert.NoError(t, err)
	d2 := path.Join(tmpDir, "2")
	assert.NoError(t, os.Mkdir(d2, os.ModePerm))
	g, err := repograph.NewLocalGraph(ctx, gb.Dir(), d2)
	assert.NoError(t, err)
	assert.NoError(t, g.Update(ctx))

	check := func(from, to string, expectOrig []string) {
		expect := util.CopyStringSlice(expectOrig)
		revs, err := co.RevList(ctx, fmt.Sprintf("%s..%s", from, to))
		assert.NoError(t, err)
		// Sanity check; assert that the commits returned from git are
		// in reverse topological order.
		assertHashesTopoSorted(t, g, revs)

		// Topological sorting is not deterministic, so we can't compare
		// the slices directly.
		sort.Strings(expect)
		sort.Strings(revs)
		deepequal.AssertDeepEqual(t, expect, revs)

		revs, err = g.RevList(from, to)
		assert.NoError(t, err)
		assertHashesTopoSorted(t, g, revs)
		sort.Strings(revs)
		deepequal.AssertDeepEqual(t, expect, revs)
	}

	check(commits[0], commits[4], commits[1:])
	check(commits[1], commits[2], commits[2:3])
	check(commits[1], commits[4], commits[2:5])
	check(commits[2], commits[4], commits[3:5])
	check(commits[3], commits[4], []string{commits[2], commits[4]})
}

func TestBranchMembership(t sktest.TestingT, ctx context.Context, gb *git_testutils.GitBuilder, repo *repograph.Graph, rf RepoImplRefresher) {
	commits := GitSetup(t, ctx, gb, repo, rf)
	c1 := commits[0]
	c2 := commits[1]
	c3 := commits[2]
	c4 := commits[3]
	c5 := commits[4]
	test := func(c *repograph.Commit, branches ...string) {
		assert.Equal(t, len(branches), len(c.Branches))
		for _, b := range branches {
			assert.True(t, c.Branches[b])
		}
	}

	// Update branch info. Ensure that all commits were updated with the
	// correct branch membership.
	repo.UpdateBranchInfo()
	test(c1, "master", "branch2")
	test(c2, "master", "branch2")
	test(c3, "branch2") // c3 is reachable from master, but not via first-parent.
	test(c4, "master")
	test(c5, "master")

	// Add a branch.
	gb.CreateBranchTrackBranch(ctx, "b3", "master")
	rf.Refresh()
	assert.NoError(t, repo.Update(ctx))
	repo.UpdateBranchInfo()
	test(c1, "master", "branch2", "b3")
	test(c2, "master", "branch2", "b3")
	test(c3, "branch2") // c3 is reachable from b3, but not via first-parent.
	test(c4, "master", "b3")
	test(c5, "master", "b3")

	// Reset b3 to branch2.
	gb.Reset(ctx, "--hard", "branch2")
	rf.Refresh()
	assert.NoError(t, repo.Update(ctx))
	repo.UpdateBranchInfo()
	test(c1, "master", "branch2", "b3")
	test(c2, "master", "branch2", "b3")
	test(c3, "branch2", "b3")
	test(c4, "master")
	test(c5, "master")

	// Delete b3. We should get the same results as before it was added.
	gb.CheckoutBranch(ctx, "master")
	gb.UpdateRef(ctx, "-d", "refs/heads/b3")
	rf.Refresh()
	assert.NoError(t, repo.Update(ctx))
	repo.UpdateBranchInfo()
	test(c1, "master", "branch2")
	test(c2, "master", "branch2")
	test(c3, "branch2")
	test(c4, "master")
	test(c5, "master")

	// Add a commit.
	c6hash := gb.CommitGen(ctx, "blah")
	c6details, err := git.GitDir(gb.Dir()).Details(ctx, c6hash)
	assert.NoError(t, err)
	rf.Refresh(c6details)
	assert.NoError(t, repo.Update(ctx))
	repo.UpdateBranchInfo()
	c6 := repo.Get(c6hash)
	assert.NotNil(t, c6)
	test(c1, "master", "branch2")
	test(c2, "master", "branch2")
	test(c3, "branch2")
	test(c4, "master")
	test(c5, "master")
	test(c6, "master")
}
