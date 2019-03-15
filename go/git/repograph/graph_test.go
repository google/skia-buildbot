package repograph

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
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

	repo, err := NewLocalGraph(ctx, g.Dir(), tmp)
	assert.NoError(t, err)
	assert.NoError(t, repo.Update(ctx))

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
	assert.Equal(t, []*Commit(nil), c1.GetParents())
	assert.Equal(t, []*Commit{c2}, c3.GetParents())

	// Ensure that we can start in an empty dir and check out from scratch properly.
	tmp2, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp2)
	repo2, err := NewLocalGraph(ctx, g.Dir(), tmp2)
	assert.NoError(t, err)
	assert.NoError(t, repo2.Update(ctx))
	deepequal.AssertDeepEqual(t, repo.Branches(), repo2.Branches())
	m1 := repo.Get("master")
	m2 := repo2.Get("master")
	// These will confuse AssertDeepEqual.
	for _, c := range repo.commitsData {
		c.repo = nil
	}
	for _, c := range repo2.commitsData {
		c.repo = nil
	}
	deepequal.AssertDeepEqual(t, m1, m2)
}

func TestSerialize(t *testing.T) {
	testutils.MediumTest(t)
	ctx, g, repo, _, cleanup := gitSetup(t)
	defer cleanup()

	repo2, err := NewLocalGraph(ctx, g.Dir(), path.Dir(repo.repo.Dir()))
	assert.NoError(t, err)
	assert.NoError(t, repo2.Update(ctx))

	deepequal.AssertDeepEqual(t, repo, repo2)
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
	assert.NoError(t, head.Recurse(func(c *Commit) error {
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
	gotCommits = map[*Commit]bool{}
	assert.NoError(t, head.Recurse(func(c *Commit) error {
		gotCommits[c] = true
		if c == c3 || c == c4 {
			return ErrStopRecursing
		}
		return nil
	}))
	assert.False(t, gotCommits[c1])
	assert.False(t, gotCommits[c2])

	// Verify that we properly exit immediately when the passed-in function
	// returns an error.
	gotCommits = map[*Commit]bool{}
	assert.Error(t, head.Recurse(func(c *Commit) error {
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
		assert.NoError(t, repo.RecurseAllBranches(func(c *Commit) error {
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
	assert.NoError(t, repo.RecurseAllBranches(func(c *Commit) error {
		gotCommits[c] = true
		if c == c3 || c == c4 {
			return ErrStopRecursing
		}
		return nil
	}))
	assert.False(t, gotCommits[c1])
	assert.False(t, gotCommits[c2])

	// Verify that we error out properly.
	gotCommits = map[*Commit]bool{}
	assert.Error(t, repo.RecurseAllBranches(func(c *Commit) error {
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

	assert.NoError(t, c6.Recurse(func(c *Commit) error {
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
	assert.NoError(t, repo.Update(ctx))
	assert.NotNil(t, repo.Get(c7))
	assert.NotNil(t, repo.Get(c8))
	master := repo.Get("master")
	assert.NotNil(t, master)
	assert.Equal(t, c8, master.Hash)
	assert.NoError(t, repo.RecurseAllBranches(func(c *Commit) error {
		assert.NotEqual(t, c.Hash, commits[2].Hash)
		assert.NotEqual(t, c.Hash, commits[4].Hash)
		return nil
	}))
	assert.Nil(t, repo.Get(commits[2].Hash)) // Should be orphaned now.
	assert.Nil(t, repo.Get(commits[4].Hash)) // Should be orphaned now.

	// Delete branch2. Ensure that c6 disappears.
	sklog.Error("Deleting branch2")
	g.UpdateRef(ctx, "-d", "refs/heads/branch2")
	assert.NoError(t, repo.Update(ctx))
	assert.Nil(t, repo.Get("branch2"))
	assert.Nil(t, repo.Get(c6hash))

	// Rewind a branch. Make sure that we correctly handle this case.
	sklog.Error("Rewinding master")
	removed := []string{c7, c8}
	for _, c := range removed {
		assert.NotNil(t, repo.Get(c))
	}
	g.UpdateRef(ctx, "refs/heads/master", commits[0].Hash)
	g.UpdateRef(ctx, "refs/heads/new", commits[0].Hash)
	assert.NoError(t, repo.Update(ctx))
	assert.NotNil(t, repo.Get("master"))
	assert.NotNil(t, repo.Get(commits[0].Hash))
	assert.Equal(t, commits[0].Hash, repo.Get(commits[0].Hash).Hash)
	for _, c := range removed {
		assert.Nil(t, repo.Get(c))
	}
}

func TestUpdateAndReturnNewCommits(t *testing.T) {
	testutils.LargeTest(t)
	ctx, g, repo, _, cleanup := gitSetup(t)
	defer cleanup()

	// The repo has commits, but gitSetup has already run Update(), so
	// there's nothing new.
	newCommits, err := repo.UpdateAndReturnNewCommits(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(newCommits), 0)

	// No new commits.
	newCommits, err = repo.UpdateAndReturnNewCommits(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(newCommits))

	// Add a few commits, ensure that they get picked up.
	g.CheckoutBranch(ctx, "master")
	f := "myfile"
	new1 := g.CommitGen(ctx, f)
	new2 := g.CommitGen(ctx, f)
	newCommits, err = repo.UpdateAndReturnNewCommits(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(newCommits))
	if newCommits[0].Hash == new1 {
		assert.Equal(t, new2, newCommits[1].Hash)
	} else {
		assert.Equal(t, new1, newCommits[1].Hash)
		assert.Equal(t, new2, newCommits[0].Hash)
	}

	// Add commits on both branches, ensure that they get picked up.
	new1 = g.CommitGen(ctx, f)
	g.CheckoutBranch(ctx, "branch2")
	new2 = g.CommitGen(ctx, "file2")
	newCommits, err = repo.UpdateAndReturnNewCommits(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(newCommits))
	if newCommits[0].Hash == new1 {
		assert.Equal(t, new2, newCommits[1].Hash)
	} else {
		assert.Equal(t, new1, newCommits[1].Hash)
		assert.Equal(t, new2, newCommits[0].Hash)
	}

	// Add a new branch. Make sure that we don't get duplicate commits.
	g.CheckoutBranch(ctx, "master")
	g.CreateBranchTrackBranch(ctx, "branch3", "master")
	newCommits, err = repo.UpdateAndReturnNewCommits(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(newCommits))
	assert.Equal(t, 3, len(repo.BranchHeads()))

	// Make sure we get no duplicates if the branch heads aren't the same.
	g.Reset(ctx, "--hard", "master^")
	newCommits, err = repo.UpdateAndReturnNewCommits(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(newCommits))

	// Create a new branch.
	g.CheckoutBranch(ctx, "master")
	g.CreateBranchTrackBranch(ctx, "branch4", "master")
	newCommits, err = repo.UpdateAndReturnNewCommits(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(newCommits))

	// Add a commit on the new branch.
	new1 = g.CommitGen(ctx, f)
	newCommits, err = repo.UpdateAndReturnNewCommits(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(newCommits))
	assert.Equal(t, new1, newCommits[0].Hash)

	// Add a merge commit. Because there were no commits on master in
	// between, the master branch head moves and now has the same hash as
	// "branch4", which means that there aren't any new commits even though
	// the branch head changed.
	g.CheckoutBranch(ctx, "master")
	mergeCommit := g.MergeBranch(ctx, "branch4")
	newCommits, err = repo.UpdateAndReturnNewCommits(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(newCommits))
	assert.Equal(t, mergeCommit, new1)

	// Create a new branch.
	g.CheckoutBranch(ctx, "master")
	g.CreateBranchTrackBranch(ctx, "branch5", "master")
	newCommits, err = repo.UpdateAndReturnNewCommits(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(newCommits))

	// Add a commit on the new branch.
	new1 = g.CommitGen(ctx, f)
	newCommits, err = repo.UpdateAndReturnNewCommits(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(newCommits))
	assert.Equal(t, new1, newCommits[0].Hash)

	// Add a commit on the master branch.
	g.CheckoutBranch(ctx, "master")
	new1 = g.CommitGen(ctx, "file2")
	newCommits, err = repo.UpdateAndReturnNewCommits(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(newCommits))
	assert.Equal(t, new1, newCommits[0].Hash)

	// Merge "branch5" into master. This should result in a new commit.
	mergeCommit = g.MergeBranch(ctx, "branch5")
	newCommits, err = repo.UpdateAndReturnNewCommits(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(newCommits))
	assert.Equal(t, mergeCommit, newCommits[0].Hash)
}

func TestRevList(t *testing.T) {
	testutils.MediumTest(t)

	ctx := context.Background()
	gb := git_testutils.GitInit(t, ctx)
	defer gb.Cleanup()
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
	g, err := NewLocalGraph(ctx, gb.Dir(), d2)
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

// Assert that the given Commits are in reverse topological order.
func assertTopoSorted(t *testing.T, commits []*Commit) {
	// Collect all commits in a map so we can quickly check for their
	// presence in the commits slice.
	commitsMap := make(map[*Commit]bool, len(commits))
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
	children := make(map[*Commit]map[*Commit]bool, len(commits))
	for _, c := range commits {
		for _, p := range c.parents {
			// Don't include parents which aren't in the slice.
			if !commitsMap[p] {
				continue
			}
			subMap, ok := children[p]
			if !ok {
				subMap = map[*Commit]bool{}
				children[p] = subMap
			}
			subMap[c] = true
		}
	}

	// Verify that the above cases are true for each commit in the slice.
	for idx, c := range commits {
		// If the commit has one parent, and its parent has one child,
		// they should be adjacent.
		if len(c.parents) == 1 && len(children[c.parents[0]]) == 1 {
			// Expect that we're not at the end of the commits slice
			// since the parent should be listed after the current
			// commit.
			assert.True(t, len(commits) > idx+1)
			assert.Equal(t, c.parents[0], commits[idx+1])
		}

		// If the commit has multiple parents, it should be adjacent to
		// one of them.
		if len(c.parents) > 1 {
			expectParentAdjacent := false
			parentIsAdjacent := false
			for _, p := range c.parents {
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
			for child, _ := range children[c] {
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
func assertHashesTopoSorted(t *testing.T, repo *Graph, hashes []string) {
	commits := make([]*Commit, 0, len(hashes))
	for _, hash := range hashes {
		commits = append(commits, repo.Get(hash))
	}
	assertTopoSorted(t, commits)
}

func TestTopoSort(t *testing.T) {
	testutils.LargeTest(t)

	ctx := context.Background()

	check := func(commits []*Commit) {
		sorted := TopologicalSort(commits)

		// Ensure that all of the commits are in the resulting slice.
		assert.Equal(t, len(commits), len(sorted))
		found := make(map[*Commit]bool, len(commits))
		for _, c := range sorted {
			found[c] = true
		}
		for _, c := range commits {
			assert.True(t, found[c])
		}
		assertTopoSorted(t, sorted)
	}
	checkGraph := func(g *Graph) {
		sets := util.PowerSet(len(g.commitsData))
		for _, set := range sets {
			inp := make([]*Commit, 0, len(g.commitsData))
			for _, idx := range set {
				inp = append(inp, g.commitsData[idx])
			}
			check(inp)
		}
	}
	checkGitBuilder := func(gb *git_testutils.GitBuilder, expect []string) {
		tmpDir, err := ioutil.TempDir("", "")
		assert.NoError(t, err)
		defer testutils.RemoveAll(t, tmpDir)
		g, err := NewLocalGraph(ctx, gb.Dir(), tmpDir)
		assert.NoError(t, err)
		assert.NoError(t, g.Update(ctx))
		checkGraph(g)

		// Test stability by verifying that we get the expected ordering
		// for the entire repo, multiple times.
		for i := 0; i < 10; i++ {
			sorted := CommitSlice(TopologicalSort(g.commitsData)).Hashes()
			deepequal.AssertDeepEqual(t, expect, sorted)
		}
	}

	// Test topological sorting using the default test repo.
	{
		gb := git_testutils.GitInit(t, ctx)
		defer gb.Cleanup()
		commits := git_testutils.GitSetup(ctx, gb)

		// GitSetup doesn't wait between commits, which means that the
		// timestamps might be equal. Adjust the expectations
		// accordingly.
		expect := []string{commits[4], commits[3], commits[2], commits[1], commits[0]}
		tmpDir, err := ioutil.TempDir("", "")
		assert.NoError(t, err)
		defer testutils.RemoveAll(t, tmpDir)
		g, err := NewLocalGraph(ctx, gb.Dir(), tmpDir)
		assert.NoError(t, err)
		assert.NoError(t, g.Update(ctx))
		c3 := g.Get(commits[3])
		c2 := g.Get(commits[2])
		if c3.Timestamp.Equal(c2.Timestamp) && c3.Hash < c2.Hash {
			expect[1], expect[2] = expect[2], expect[1]
		}
		checkGitBuilder(gb, expect)
	}

	// Verify that we use the timestamp as a tie-breaker.
	{
		gb := git_testutils.GitInit(t, ctx)
		defer gb.Cleanup()
		gb.Add(ctx, "file0", "contents")
		ts := time.Unix(1552403492, 0)
		c0 := gb.CommitMsgAt(ctx, "Initial commit", ts)
		assert.Equal(t, c0, "c48b90c8ccc70b4d2bd146e4f708c398f78e2dd6") // Hashes are deterministic.
		ts = ts.Add(2 * time.Second)
		gb.Add(ctx, "file1", "contents")
		c1 := gb.CommitMsgAt(ctx, "Child 1", ts)
		gb.CreateBranchAtCommit(ctx, "otherbranch", c0)
		ts = ts.Add(2 * time.Second)
		gb.Add(ctx, "file2", "contents")
		c2 := gb.CommitMsgAt(ctx, "Child 2", ts)
		// c1 and c2 both have c0 as a parent. The topological ordering
		// is ambiguous unless we use the timestamp as a tie-breaker, in
		// which case c2 should always come before c1, since it is more
		// recent.
		checkGitBuilder(gb, []string{c2, c1, c0})
	}

	// Same as above, but the child commits have the same timestamp. Verify
	// that we use the commit hash as a secondary tie-breaker.
	{
		gb := git_testutils.GitInit(t, ctx)
		defer gb.Cleanup()
		gb.Add(ctx, "file0", "contents")
		ts := time.Unix(1552403492, 0)
		c0 := gb.CommitMsgAt(ctx, "Initial commit", ts)
		assert.Equal(t, c0, "c48b90c8ccc70b4d2bd146e4f708c398f78e2dd6") // Hashes are deterministic.
		ts = ts.Add(2 * time.Second)
		gb.Add(ctx, "file1", "contents")
		c1 := gb.CommitMsgAt(ctx, "Child 1", ts)
		assert.Equal(t, c1, "dc24e5b042cdcf995a182815ef37f659e8ec20cc")
		gb.CreateBranchAtCommit(ctx, "otherbranch", c0)
		gb.Add(ctx, "file2", "contents")
		c2 := gb.CommitMsgAt(ctx, "Child 2", ts)
		assert.Equal(t, c2, "06d29d7f828723c79c9eba25696111c628ab5a7e")
		// c1 and c2 both have c0 as a parent, and they both have the
		// same timestamp. The topological ordering is ambiguous even
		// with the timestamp as a tie-breaker, so we have to use the
		// commit hash.
		checkGitBuilder(gb, []string{c1, c2, c0})
	}

	// Extend the above to ensure that, in the case of a merge, we follow
	// the parent with the newer timestamp.
	{
		gb := git_testutils.GitInit(t, ctx)
		defer gb.Cleanup()
		gb.Add(ctx, "file0", "contents")
		ts := time.Unix(1552403492, 0)
		c0 := gb.CommitMsgAt(ctx, "Initial commit", ts)
		assert.Equal(t, c0, "c48b90c8ccc70b4d2bd146e4f708c398f78e2dd6") // Hashes are deterministic.
		ts = ts.Add(2 * time.Second)
		gb.Add(ctx, "file1", "contents")
		c1 := gb.CommitMsgAt(ctx, "Child 1", ts)
		assert.Equal(t, c1, "dc24e5b042cdcf995a182815ef37f659e8ec20cc")
		gb.CreateBranchAtCommit(ctx, "otherbranch", c0)
		gb.Add(ctx, "file2", "contents")
		c2 := gb.CommitMsgAt(ctx, "Child 2", ts)
		assert.Equal(t, c2, "06d29d7f828723c79c9eba25696111c628ab5a7e")
		gb.CheckoutBranch(ctx, "master")
		c3 := gb.CommitGen(ctx, "file1")
		ts = ts.Add(10 * time.Second)
		c4 := gb.CommitGenAt(ctx, "file1", ts)
		gb.CheckoutBranch(ctx, "otherbranch")
		c5 := gb.CommitGen(ctx, "file2")
		c6 := gb.CommitGenAt(ctx, "file2", ts.Add(-time.Second))
		gb.CheckoutBranch(ctx, "master")
		c7 := gb.MergeBranch(ctx, "otherbranch")
		checkGitBuilder(gb, []string{c7, c4, c3, c1, c6, c5, c2, c0})
	}
}

func TestIsAncestor(t *testing.T) {
	testutils.MediumTest(t)

	ctx := context.Background()
	gb := git_testutils.GitInit(t, ctx)
	defer gb.Cleanup()
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
	g, err := NewLocalGraph(ctx, gb.Dir(), d2)
	assert.NoError(t, err)
	assert.NoError(t, g.Update(ctx))

	sklog.Infof("4. %s", commits[4][:4])
	sklog.Infof("    |    \\")
	sklog.Infof("3. %s   |", commits[3][:4])
	sklog.Infof("    |     |")
	sklog.Infof("    | 2. %s", commits[2][:4])
	sklog.Infof("    |     |")
	sklog.Infof("    |    /")
	sklog.Infof("1. %s", commits[1][:4])
	sklog.Infof("    |")
	sklog.Infof("0. %s", commits[0][:4])

	check := func(a, b string, expect bool) {
		// Compare against actual git.
		got, err := co.IsAncestor(ctx, a, b)
		assert.NoError(t, err)
		assert.Equal(t, expect, got)
		got, err = g.IsAncestor(a, b)
		assert.NoError(t, err)
		assert.Equal(t, expect, got)
	}
	check(commits[0], commits[0], true)
	check(commits[0], commits[1], true)
	check(commits[0], commits[2], true)
	check(commits[0], commits[3], true)
	check(commits[0], commits[4], true)

	check(commits[1], commits[0], false)
	check(commits[1], commits[1], true)
	check(commits[1], commits[2], true)
	check(commits[1], commits[3], true)
	check(commits[1], commits[4], true)

	check(commits[2], commits[0], false)
	check(commits[2], commits[1], false)
	check(commits[2], commits[2], true)
	check(commits[2], commits[3], false)
	check(commits[2], commits[4], true)

	check(commits[3], commits[0], false)
	check(commits[3], commits[1], false)
	check(commits[3], commits[2], false)
	check(commits[3], commits[3], true)
	check(commits[3], commits[4], true)

	check(commits[4], commits[0], false)
	check(commits[4], commits[1], false)
	check(commits[4], commits[2], false)
	check(commits[4], commits[3], false)
	check(commits[4], commits[4], true)
}

func TestGitStore(t *testing.T) {
	testutils.LargeTest(t)

	ctx := context.Background()
	gb := git_testutils.GitInit(t, ctx)
	defer gb.Cleanup()
	commits := git_testutils.GitSetup(ctx, gb)

	// Push commits into BT.
	btConf := &gitstore.BTConfig{
		ProjectID:  "fake-project",
		InstanceID: "fake-instance",
		TableID:    "repograph-gitstore",
	}
	assert.NoError(t, gitstore.InitBT(btConf))
	gs, err := gitstore.NewBTGitStore(ctx, btConf, gb.RepoUrl())
	assert.NoError(t, err)

	repo := git.GitDir(gb.Dir())

	// Wait for GitStore to be up to date.
	btWait := func() {
		expect, err := repo.Branches(ctx)
		assert.NoError(t, err)
		for {
			time.Sleep(10 * time.Millisecond)
			actual, err := gs.GetBranches(ctx)
			assert.NoError(t, err)
			allMatch := true
			for _, expectBranch := range expect {
				actualBranch, ok := actual[expectBranch.Name]
				if !ok || actualBranch.Head != expectBranch.Head {
					allMatch = false
					break
				}
			}
			if allMatch {
				break
			}
		}
	}

	// Add the given commits to the GitStore.
	addCommitsToGitStore := func(commits []string) {
		details := make([]*vcsinfo.LongCommit, 0, len(commits))
		for _, c := range commits {
			d, err := repo.Details(ctx, c)
			assert.NoError(t, err)
			details = append(details, d)
		}
		assert.NoError(t, gs.Put(ctx, details))
		branches, err := repo.Branches(ctx)
		assert.NoError(t, err)
		putBranches := make(map[string]string, len(branches))
		for _, branch := range branches {
			putBranches[branch.Name] = branch.Head
		}
		assert.NoError(t, gs.PutBranches(ctx, putBranches))
		btWait()
	}

	addCommitsToGitStore(commits)

	// Create the graph.
	g, err := NewGitStoreGraph(ctx, gs)
	assert.NoError(t, err)

	check := func(commits []string) {
		for _, hash := range commits {
			c := g.Get(hash)
			assert.NotNil(t, c)
			d, err := repo.Details(ctx, hash)
			assert.NoError(t, err)
			d.Timestamp = d.Timestamp.UTC()
			if len(d.Parents) == 0 && len(c.LongCommit.Parents) == 0 {
				d.Parents = c.LongCommit.Parents
			}
			deepequal.AssertDeepEqual(t, d, c.LongCommit)
		}
	}
	check(commits)

	// Add some new commits.
	c1 := gb.CommitGen(ctx, "file1")
	c2 := gb.CommitGen(ctx, "file1")
	gb.CreateBranchTrackBranch(ctx, "otherbranch", "master")
	c3 := gb.CommitGen(ctx, "file2")
	addCommitsToGitStore([]string{c1, c2, c3})
	assert.NoError(t, g.Update(ctx))
	check([]string{c1, c2, c3})
	assert.Equal(t, c2, g.Get("master").Hash)
	assert.Equal(t, c3, g.Get("otherbranch").Hash)

	// Add a commit in the future. It shouldn't get included in the commits
	// initially returned by GitStore.RangeByTime(). Ensure that we still
	// end up with the new commit.
	now := time.Now().Add(time.Second)
	c4 := gb.CommitGenAt(ctx, "file2", now.Add(900*time.Hour))
	addCommitsToGitStore([]string{c4})
	indexCommits, err := gs.RangeByTime(ctx, now.Add(-30*time.Minute), now, gb.RepoUrl())
	assert.NoError(t, err)
	// Sanity check; ensure that the commit doesn't show up.
	for _, c := range indexCommits {
		assert.NotEqual(t, c4, c.Hash)
	}
	// Verify that the commit still shows up.
	assert.NoError(t, g.Update(ctx))
	check([]string{c4})
}
