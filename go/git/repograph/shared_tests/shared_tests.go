package shared_tests

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
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
	t0 := time.Unix(1564963200, 0) // Arbitrary time to fix commit hashes.
	ts := t0
	fileNum := 0
	doGit := func(hash string) *vcsinfo.LongCommit {
		ts = ts.Add(time.Second)
		fileNum++
		details, err := git.GitDir(g.Dir()).Details(ctx, hash)
		require.NoError(t, err)
		return details
	}
	commit := func() *vcsinfo.LongCommit {
		return doGit(g.CommitGenAt(ctx, fmt.Sprintf("file%d", fileNum), ts))
	}
	merge := func(branch string) *vcsinfo.LongCommit {
		return doGit(g.MergeBranchAt(ctx, branch, ts))
	}

	c1details := commit()
	rf.Refresh(c1details)
	require.NoError(t, repo.Update(ctx))

	c1 := repo.Get(git.MainBranch)
	require.NotNil(t, c1)
	require.Equal(t, 0, len(c1.GetParents()))
	require.False(t, util.TimeIsZero(c1.Timestamp))

	c2details := commit()
	rf.Refresh(c2details)
	require.NoError(t, repo.Update(ctx))
	c2 := repo.Get(git.MainBranch)
	require.NotNil(t, c2)
	require.Equal(t, 1, len(c2.GetParents()))
	require.Equal(t, c1, c2.GetParents()[0])
	require.Equal(t, []string{git.MainBranch}, repo.Branches())
	require.False(t, util.TimeIsZero(c2.Timestamp))

	// Create a second branch.
	g.CreateBranchTrackBranch(ctx, "branch2", git.DefaultRemoteBranch)
	c3details := commit()
	rf.Refresh(c1details, c2details, c3details)
	require.NoError(t, repo.Update(ctx))
	c3 := repo.Get("branch2")
	require.NotNil(t, c3)
	require.Equal(t, c2, repo.Get(git.MainBranch))
	require.Equal(t, []string{"branch2", git.MainBranch}, repo.Branches())
	require.False(t, util.TimeIsZero(c3.Timestamp))

	// Commit again to the main branch.
	g.CheckoutBranch(ctx, git.MainBranch)
	c4details := commit()
	rf.Refresh(c4details)
	require.NoError(t, repo.Update(ctx))
	require.Equal(t, c3, repo.Get("branch2"))
	c4 := repo.Get(git.MainBranch)
	require.NotNil(t, c4)
	require.False(t, util.TimeIsZero(c4.Timestamp))

	// Merge branch2 into main.
	c5details := merge("branch2")
	rf.Refresh(c1details, c2details, c3details, c4details, c5details)
	require.NoError(t, repo.Update(ctx))
	require.Equal(t, []string{"branch2", git.MainBranch}, repo.Branches())
	c5 := repo.Get(git.MainBranch)
	require.NotNil(t, c5)
	require.Equal(t, c3, repo.Get("branch2"))
	require.False(t, util.TimeIsZero(c5.Timestamp))

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
			require.False(t, commitsMap[p])
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
			require.True(t, len(commits) > idx+1)
			require.Equal(t, c.GetParents()[0], commits[idx+1])
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
			require.Equal(t, expectParentAdjacent, parentIsAdjacent)
		}

		// If the commit has multiple children, it should be adjacent
		// to one of them.
		if len(children[c]) > 1 {
			require.True(t, idx > 0)
			childIsAdjacent := false
			for child := range children[c] {
				if commits[idx-1] == child {
					childIsAdjacent = true
					break
				}
			}
			require.True(t, childIsAdjacent)
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
	require.Equal(t, []*repograph.Commit{c4, c3}, c5.GetParents())
	require.Equal(t, []*repograph.Commit{c2}, c4.GetParents())
	require.Equal(t, []*repograph.Commit{c1}, c2.GetParents())
	require.Equal(t, []*repograph.Commit(nil), c1.GetParents())
	require.Equal(t, []*repograph.Commit{c2}, c3.GetParents())

	// Assert that each of the commits has the correct index.
	require.Equal(t, 0, c1.Index)
	require.Equal(t, 1, c2.Index)
	require.Equal(t, 2, c3.Index)
	require.Equal(t, 2, c4.Index)
	require.Equal(t, 3, c5.Index)

	// Ensure that we can start in an empty dir and check out from scratch properly.
	tmp2, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp2)
	repo2, err := repograph.NewLocalGraph(ctx, g.Dir(), tmp2)
	require.NoError(t, err)
	require.NoError(t, repo2.Update(ctx))
	assertdeep.Equal(t, repo.Branches(), repo2.Branches())
	m1 := repo.Get(git.MainBranch)
	m2 := repo2.Get(git.MainBranch)
	// Different implementations may or may not track branch info.
	for _, c := range repo2.GetAll() {
		c.Branches = repo.Get(c.Hash).Branches
	}
	assertdeep.Equal(t, m1, m2)
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
	head := repo.Get(git.MainBranch)
	require.NotNil(t, head)
	gotCommits := map[*repograph.Commit]bool{}
	require.NoError(t, head.Recurse(func(c *repograph.Commit) error {
		require.False(t, gotCommits[c])
		gotCommits[c] = true
		return nil
	}))
	require.Equal(t, len(commits), len(gotCommits))
	for _, c := range commits {
		require.True(t, gotCommits[c])
	}
	// AllCommits is the same thing as the above.
	allCommits, err := head.AllCommits()
	require.NoError(t, err)
	require.Equal(t, len(allCommits), len(gotCommits))

	// Verify that we properly return early when the passed-in function
	// return false.
	gotCommits = map[*repograph.Commit]bool{}
	require.NoError(t, head.Recurse(func(c *repograph.Commit) error {
		gotCommits[c] = true
		if c == c3 || c == c4 {
			return repograph.ErrStopRecursing
		}
		return nil
	}))
	require.False(t, gotCommits[c1])
	require.False(t, gotCommits[c2])

	// Verify that we properly exit immediately when the passed-in function
	// returns an error.
	gotCommits = map[*repograph.Commit]bool{}
	require.Error(t, head.Recurse(func(c *repograph.Commit) error {
		gotCommits[c] = true
		if c == c4 {
			return fmt.Errorf("STOP!")
		}
		return nil
	}))
	require.False(t, gotCommits[c1])
	require.False(t, gotCommits[c2])
	require.False(t, gotCommits[c3])
	require.True(t, gotCommits[c4])
	require.True(t, gotCommits[c5])
}

func TestRecurseAllBranches(t sktest.TestingT, ctx context.Context, g *git_testutils.GitBuilder, repo *repograph.Graph, rf RepoImplRefresher) {
	commits := GitSetup(t, ctx, g, repo, rf)

	c1 := commits[0]
	c2 := commits[1]
	c3 := commits[2]
	c4 := commits[3]

	test := func() {
		gotCommits := map[*repograph.Commit]bool{}
		require.NoError(t, repo.RecurseAllBranches(func(c *repograph.Commit) error {
			require.False(t, gotCommits[c])
			gotCommits[c] = true
			return nil
		}))
		require.Equal(t, len(commits), len(gotCommits))
		for _, c := range commits {
			require.True(t, gotCommits[c])
		}
	}

	// Get the list of commits using head.RecurseAllBranches(). Ensure that
	// we get all of the commits but don't get any duplicates.
	test()

	// The above used only one branch. Add a branch and ensure that we see
	// its commits too.
	g.CreateBranchTrackBranch(ctx, "mybranch", git.DefaultRemoteBranch)
	c5 := g.CommitGen(ctx, "anotherfile.txt")
	c5details, err := git.GitDir(g.Dir()).Details(ctx, c5)
	require.NoError(t, err)
	rf.Refresh(c5details)
	require.NoError(t, repo.Update(ctx))
	c := repo.Get("mybranch")
	require.NotNil(t, c)
	commits = append(commits, c)
	test()

	// Verify that we don't revisit a branch whose HEAD is an ancestor of
	// a different branch HEAD.
	g.CreateBranchAtCommit(ctx, "ancestorbranch", c3.Hash)
	rf.Refresh()
	require.NoError(t, repo.Update(ctx))
	test()

	// Verify that we still stop recursion when requested.
	gotCommits := map[*repograph.Commit]bool{}
	require.NoError(t, repo.RecurseAllBranches(func(c *repograph.Commit) error {
		gotCommits[c] = true
		if c == c3 || c == c4 {
			return repograph.ErrStopRecursing
		}
		return nil
	}))
	require.False(t, gotCommits[c1])
	require.False(t, gotCommits[c2])

	// Verify that we error out properly.
	gotCommits = map[*repograph.Commit]bool{}
	require.Error(t, repo.RecurseAllBranches(func(c *repograph.Commit) error {
		gotCommits[c] = true
		// Because of nondeterministic map iteration and the added
		// branches, we have to halt way back at c2 in order to have
		// a sane, deterministic test case.
		if c == c2 {
			return fmt.Errorf("STOP!")
		}
		return nil
	}))
	require.False(t, gotCommits[c1])
	require.True(t, gotCommits[c2])
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
				cmd = append(cmd, "--ancestry-path", git.LogFromTo(from, to))
			}
			hashes, err := gitdir.RevList(ctx, cmd...)
			require.NoError(t, err)
			require.Equal(t, len(hashes), len(expect))
			for i, h := range hashes {
				require.Equal(t, h, expect[i].Hash)
			}
		}

		// Ensure that we get the expected results from the Graph.
		actual, err := repo.LogLinear(from, to)
		require.NoError(t, err)
		assertdeep.Equal(t, expect, actual)
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
	require.NotNil(t, c3)
	require.Equal(t, c3, commits[2]) // c3 from setup()

	// Change branch 2 to be based at c4 with one commit, c6.
	g.CheckoutBranch(ctx, "branch2")
	g.Reset(ctx, "--hard", commits[3].Hash) // c4 from setup()
	f := "myfile"
	c6hash := g.CommitGen(ctx, f)
	c6details, err := git.GitDir(g.Dir()).Details(ctx, c6hash)
	require.NoError(t, err)
	rf.Refresh(c6details)
	require.NoError(t, repo.Update(ctx))
	c6 := repo.Get("branch2")
	require.NotNil(t, c6)
	require.Equal(t, c6hash, c6.Hash)

	// Ensure that c3 is not reachable from c6.
	anc, err := repo.IsAncestor(c3.Hash, c6.Hash)
	require.NoError(t, err)
	require.False(t, anc)

	require.NoError(t, c6.Recurse(func(c *repograph.Commit) error {
		require.NotEqual(t, c, c3)
		return nil
	}))

	// Create a new branch, add some commits. Reset the old branches to
	// orphan some commits. Ensure that those are removed from the graph.
	g.CreateBranchAtCommit(ctx, "new", commits[0].Hash)
	c7 := g.CommitGen(ctx, "blah")
	c8 := g.CommitGen(ctx, "blah")
	g.CheckoutBranch(ctx, git.MainBranch)
	g.Reset(ctx, "--hard", c8)
	c7details, err := git.GitDir(g.Dir()).Details(ctx, c7)
	require.NoError(t, err)
	c8details, err := git.GitDir(g.Dir()).Details(ctx, c8)
	require.NoError(t, err)
	rf.Refresh(c7details, c8details)
	require.NoError(t, repo.Update(ctx))
	require.NotNil(t, repo.Get(c7))
	require.NotNil(t, repo.Get(c8))
	main := repo.Get(git.MainBranch)
	require.NotNil(t, main)
	require.Equal(t, c8, main.Hash)
	require.NoError(t, repo.RecurseAllBranches(func(c *repograph.Commit) error {
		require.NotEqual(t, c.Hash, commits[2].Hash)
		require.NotEqual(t, c.Hash, commits[4].Hash)
		return nil
	}))
	require.Nil(t, repo.Get(commits[2].Hash)) // Should be orphaned now.
	require.Nil(t, repo.Get(commits[4].Hash)) // Should be orphaned now.

	// Delete branch2. Ensure that c6 disappears.
	g.UpdateRef(ctx, "-d", "refs/heads/branch2")
	rf.Refresh()
	require.NoError(t, repo.Update(ctx))
	require.Nil(t, repo.Get("branch2"))
	require.Nil(t, repo.Get(c6hash))

	// Rewind a branch. Make sure that we correctly handle this case.
	removed := []string{c7, c8}
	for _, c := range removed {
		require.NotNil(t, repo.Get(c))
	}
	g.UpdateRef(ctx, git.DefaultRef, commits[0].Hash)
	g.UpdateRef(ctx, "refs/heads/new", commits[0].Hash)
	rf.Refresh()
	require.NoError(t, repo.Update(ctx))
	require.NotNil(t, repo.Get(git.MainBranch))
	require.NotNil(t, repo.Get(commits[0].Hash))
	require.Equal(t, commits[0].Hash, repo.Get(commits[0].Hash).Hash)
	for _, c := range removed {
		require.Nil(t, repo.Get(c))
	}
}

func TestUpdateAndReturnCommitDiffs(t sktest.TestingT, ctx context.Context, g *git_testutils.GitBuilder, repo *repograph.Graph, rf RepoImplRefresher) {
	GitSetup(t, ctx, g, repo, rf)

	// The repo has commits, but GitSetup has already run Update(), so
	// there's nothing new.
	rf.Refresh()
	added, removed, err := repo.UpdateAndReturnCommitDiffs(ctx)
	require.NoError(t, err)
	require.Equal(t, len(added), 0)
	require.Equal(t, len(removed), 0)

	// No new commits.
	rf.Refresh()
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(added))
	require.Equal(t, len(removed), 0)

	// Add a few commits, ensure that they get picked up.
	g.CheckoutBranch(ctx, git.MainBranch)
	f := "myfile"
	new1 := g.CommitGen(ctx, f)
	new2 := g.CommitGen(ctx, f)
	new1details, err := git.GitDir(g.Dir()).Details(ctx, new1)
	require.NoError(t, err)
	new2details, err := git.GitDir(g.Dir()).Details(ctx, new2)
	require.NoError(t, err)
	rf.Refresh(new1details, new2details)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, len(added))
	require.Equal(t, len(removed), 0)
	if added[0].Hash == new1 {
		require.Equal(t, new2, added[1].Hash)
	} else {
		require.Equal(t, new1, added[1].Hash)
		require.Equal(t, new2, added[0].Hash)
	}

	// Add commits on both branches, ensure that they get picked up.
	new1 = g.CommitGen(ctx, f)
	g.CheckoutBranch(ctx, "branch2")
	new2 = g.CommitGen(ctx, "file2")
	new1details, err = git.GitDir(g.Dir()).Details(ctx, new1)
	require.NoError(t, err)
	new2details, err = git.GitDir(g.Dir()).Details(ctx, new2)
	require.NoError(t, err)
	rf.Refresh(new1details, new2details)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, len(added))
	require.Equal(t, len(removed), 0)
	if added[0].Hash == new1 {
		require.Equal(t, new2, added[1].Hash)
	} else {
		require.Equal(t, new1, added[1].Hash)
		require.Equal(t, new2, added[0].Hash)
	}

	// Add a new branch. Make sure that we don't get duplicate commits.
	g.CheckoutBranch(ctx, git.MainBranch)
	g.CreateBranchTrackBranch(ctx, "branch3", git.MainBranch)
	rf.Refresh()
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(added))
	require.Equal(t, len(removed), 0)
	require.Equal(t, 3, len(repo.BranchHeads()))

	// Make sure we get no duplicates if the branch heads aren't the same.
	g.Reset(ctx, "--hard", git.MainBranch+"^")
	rf.Refresh()
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(added))
	require.Equal(t, len(removed), 0)

	// Create a new branch.
	g.CheckoutBranch(ctx, git.MainBranch)
	g.CreateBranchTrackBranch(ctx, "branch4", git.MainBranch)
	rf.Refresh()
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(added))
	require.Equal(t, len(removed), 0)

	// Add a commit on the new branch.
	new1 = g.CommitGen(ctx, f)
	new1details, err = git.GitDir(g.Dir()).Details(ctx, new1)
	require.NoError(t, err)
	rf.Refresh(new1details)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(added))
	require.Equal(t, len(removed), 0)
	require.Equal(t, new1, added[0].Hash)

	// Add a merge commit. Because there were no commits on main in
	// between, the main branch head moves and now has the same hash as
	// "branch4", which means that there aren't any new commits even though
	// the branch head changed.
	g.CheckoutBranch(ctx, git.MainBranch)
	mergeCommit := g.MergeBranch(ctx, "branch4")
	mergeCommitDetails, err := git.GitDir(g.Dir()).Details(ctx, mergeCommit)
	require.NoError(t, err)
	rf.Refresh(mergeCommitDetails)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(added))
	require.Equal(t, len(removed), 0)
	require.Equal(t, mergeCommit, new1)

	// Create a new branch.
	g.CheckoutBranch(ctx, git.MainBranch)
	g.CreateBranchTrackBranch(ctx, "branch5", git.MainBranch)
	rf.Refresh()
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(added))
	require.Equal(t, len(removed), 0)

	// Add a commit on the new branch.
	new1 = g.CommitGen(ctx, f)
	new1details, err = git.GitDir(g.Dir()).Details(ctx, new1)
	require.NoError(t, err)
	rf.Refresh(new1details)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(added))
	require.Equal(t, len(removed), 0)
	require.Equal(t, new1, added[0].Hash)

	// Add a commit on the main branch.
	g.CheckoutBranch(ctx, git.MainBranch)
	new1 = g.CommitGen(ctx, "file2")
	new1details, err = git.GitDir(g.Dir()).Details(ctx, new1)
	require.NoError(t, err)
	rf.Refresh(new1details)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(added))
	require.Equal(t, len(removed), 0)
	require.Equal(t, new1, added[0].Hash)

	// Merge "branch5" into main. This should result in a new commit.
	mergeCommit = g.MergeBranch(ctx, "branch5")
	mergeCommitDetails, err = git.GitDir(g.Dir()).Details(ctx, mergeCommit)
	require.NoError(t, err)
	rf.Refresh(mergeCommitDetails)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(added))
	require.Equal(t, len(removed), 0)
	require.Equal(t, mergeCommit, added[0].Hash)

	// Reset all branches to the initial commit.
	var c0 string
	require.NoError(t, repo.Get(git.MainBranch).Recurse(func(c *repograph.Commit) error {
		if len(c.Parents) == 0 {
			c0 = c.Hash
			return repograph.ErrStopRecursing
		}
		return nil
	}))
	for _, branch := range []string{git.MainBranch, "branch2", "branch3", "branch4", "branch5"} {
		g.CheckoutBranch(ctx, branch)
		g.Reset(ctx, "--hard", c0)
	}
	rf.Refresh()
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(added))
	require.Equal(t, 12, len(removed))

	// Add some new commits, some of which share an ancestor. Ensure that
	// the added list doesn't double-count the shared commit.
	g.CheckoutBranch(ctx, git.MainBranch)
	shared := g.CommitGen(ctx, "f")
	main := g.CommitGen(ctx, "f")
	g.CheckoutBranch(ctx, "branch2")
	g.Reset(ctx, "--hard", shared)
	branch2 := g.CommitGen(ctx, "f2")
	sharedDetails, err := git.GitDir(g.Dir()).Details(ctx, shared)
	mainDetails, err := git.GitDir(g.Dir()).Details(ctx, main)
	branch2Details, err := git.GitDir(g.Dir()).Details(ctx, branch2)
	rf.Refresh(sharedDetails, mainDetails, branch2Details)
	added, removed, err = repo.UpdateAndReturnCommitDiffs(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, len(added))
	require.Equal(t, 0, len(removed))
}

func TestRevList(t sktest.TestingT, ctx context.Context, gb *git_testutils.GitBuilder, repo *repograph.Graph, rf RepoImplRefresher) {
	commits := git_testutils.GitSetup(ctx, gb)
	tmpDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)
	d1 := path.Join(tmpDir, "1")
	require.NoError(t, os.Mkdir(d1, os.ModePerm))
	co, err := git.NewCheckout(ctx, gb.Dir(), d1)
	require.NoError(t, err)
	d2 := path.Join(tmpDir, "2")
	require.NoError(t, os.Mkdir(d2, os.ModePerm))
	g, err := repograph.NewLocalGraph(ctx, gb.Dir(), d2)
	require.NoError(t, err)
	require.NoError(t, g.Update(ctx))

	check := func(from, to string, expectOrig []string) {
		expect := util.CopyStringSlice(expectOrig)
		revs, err := co.RevList(ctx, git.LogFromTo(from, to))
		require.NoError(t, err)
		// Sanity check; assert that the commits returned from git are
		// in reverse topological order.
		assertHashesTopoSorted(t, g, revs)

		// Topological sorting is not deterministic, so we can't compare
		// the slices directly.
		sort.Strings(expect)
		sort.Strings(revs)
		assertdeep.Equal(t, expect, revs)

		revs, err = g.RevList(from, to)
		require.NoError(t, err)
		assertHashesTopoSorted(t, g, revs)
		sort.Strings(revs)
		assertdeep.Equal(t, expect, revs)
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
		require.Equal(t, len(branches), len(c.Branches))
		for _, b := range branches {
			require.True(t, c.Branches[b])
		}
	}
	up := func(expect ...*repograph.Commit) {
		actual := repo.UpdateBranchInfo()
		// Some implementations of RepoImpl call UpdateBranchInfo() in
		// their UpdateCallback(), so it's possible that when the test
		// calls UpdateBranchInfo(), no commits are changed.
		if len(actual) == 0 {
			return
		}
		require.Equal(t, len(actual), len(expect))
		commitMap := make(map[string]*vcsinfo.LongCommit, len(actual))
		for _, c := range actual {
			commitMap[c.Hash] = c
		}
		for _, c := range expect {
			_, ok := commitMap[c.Hash]
			require.True(t, ok, "%s not modified", c.Hash)
		}
		// Verify that we deduplicated the branch maps.
		maps := map[string]uintptr{}
		for _, c := range actual {
			keys := util.StringSet(c.Branches).Keys()
			sort.Strings(keys)
			str := strings.Join(keys, ",")
			ptr := reflect.ValueOf(c.Branches).Pointer()
			if exist, ok := maps[str]; ok {
				require.Equal(t, exist, ptr)
			} else {
				maps[str] = ptr
			}
		}
	}

	// Update branch info. Ensure that all commits were updated with the
	// correct branch membership.
	up(c1, c2, c3, c4, c5)
	test(c1, git.MainBranch, "branch2")
	test(c2, git.MainBranch, "branch2")
	test(c3, "branch2") // c3 is reachable from main, but not via first-parent.
	test(c4, git.MainBranch)
	test(c5, git.MainBranch)

	// Add a branch.
	gb.CreateBranchTrackBranch(ctx, "b3", git.MainBranch)
	rf.Refresh()
	require.NoError(t, repo.Update(ctx))
	up(c1, c2, c4, c5)
	test(c1, git.MainBranch, "branch2", "b3")
	test(c2, git.MainBranch, "branch2", "b3")
	test(c3, "branch2") // c3 is reachable from b3, but not via first-parent.
	test(c4, git.MainBranch, "b3")
	test(c5, git.MainBranch, "b3")

	// Reset b3 to branch2.
	gb.Reset(ctx, "--hard", "branch2")
	rf.Refresh()
	require.NoError(t, repo.Update(ctx))
	up(c3, c4, c5)
	test(c1, git.MainBranch, "branch2", "b3")
	test(c2, git.MainBranch, "branch2", "b3")
	test(c3, "branch2", "b3")
	test(c4, git.MainBranch)
	test(c5, git.MainBranch)

	// Reset branch2 to c4.
	gb.CheckoutBranch(ctx, "branch2")
	gb.Reset(ctx, "--hard", c4.Hash)
	rf.Refresh()
	require.NoError(t, repo.Update(ctx))
	up(c3, c4)
	test(c1, git.MainBranch, "branch2", "b3")
	test(c2, git.MainBranch, "branch2", "b3")
	test(c3, "b3")
	test(c4, git.MainBranch, "branch2")
	test(c5, git.MainBranch)

	// Delete b3. We should get the same results as before it was added.
	gb.CheckoutBranch(ctx, git.MainBranch)
	gb.UpdateRef(ctx, "-d", "refs/heads/b3")
	rf.Refresh()
	require.NoError(t, repo.Update(ctx))
	up(c1, c2, c3)
	test(c1, git.MainBranch, "branch2")
	test(c2, git.MainBranch, "branch2")
	test(c3)
	test(c4, git.MainBranch, "branch2")
	test(c5, git.MainBranch)

	// Add a commit.
	c6hash := gb.CommitGenAt(ctx, "blah", c5.Timestamp.Add(time.Second))
	c6details, err := git.GitDir(gb.Dir()).Details(ctx, c6hash)
	require.NoError(t, err)
	rf.Refresh(c6details)
	require.NoError(t, repo.Update(ctx))
	c6 := repo.Get(c6hash)
	require.NotNil(t, c6)
	up(c6)
	test(c1, git.MainBranch, "branch2")
	test(c2, git.MainBranch, "branch2")
	test(c3)
	test(c4, git.MainBranch, "branch2")
	test(c5, git.MainBranch)
	test(c6, git.MainBranch)
}
