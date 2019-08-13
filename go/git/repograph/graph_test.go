package repograph

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

func TestTopoSort(t *testing.T) {
	unittest.LargeTest(t)

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
		commitsList := make([]*Commit, 0, len(g.commits))
		for _, c := range g.commits {
			commitsList = append(commitsList, c)
		}
		sets := util.PowerSet(len(commitsList))
		for _, set := range sets {
			inp := make([]*Commit, 0, len(commitsList))
			for _, idx := range set {
				inp = append(inp, commitsList[idx])
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
			commitsList := make([]*Commit, 0, len(g.commits))
			for _, c := range g.commits {
				commitsList = append(commitsList, c)
			}
			sorted := CommitSlice(TopologicalSort(commitsList)).Hashes()
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
	unittest.MediumTest(t)

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

func TestMapUpdate(t *testing.T) {
	unittest.LargeTest(t)
	ctx, gb1, g1, r1, cleanup1 := setupRepo(t)
	defer cleanup1()
	_, gb2, g2, r2, cleanup2 := setupRepo(t)
	var cleanup2Once sync.Once
	defer cleanup2Once.Do(cleanup2)
	m := Map(map[string]*Graph{
		gb1.RepoUrl(): g1,
		gb2.RepoUrl(): g2,
	})

	// 1. Verify that updating a Map actually updates each Graph.
	new1 := gb1.CommitGen(ctx, "f")
	new2 := gb2.CommitGen(ctx, "f")
	r1.Refresh()
	r2.Refresh()
	assert.NoError(t, m.Update(ctx))
	assert.Equal(t, new1, g1.Get("master").Hash)
	assert.Equal(t, new2, g2.Get("master").Hash)

	// 2. Verify that none of the changes are committed if a callback fails.
	new1 = gb1.CommitGen(ctx, "f")
	new2 = gb2.CommitGen(ctx, "f")
	r1.Refresh()
	r2.Refresh()
	failNext := false
	assert.EqualError(t, m.UpdateWithCallback(ctx, func(repoUrl string, g *Graph) error {
		if failNext {
			return errors.New("Fail")
		}
		failNext = true
		return nil
	}), "Fail")
	assert.NotEqual(t, new1, g1.Get("master").Hash)
	assert.NotEqual(t, new2, g2.Get("master").Hash)

	// 3. Verify that none of the changes are committed if an update fails.
	cleanup2Once.Do(cleanup2)
	r1.Refresh()
	r2.Refresh()
	assert.Error(t, m.Update(ctx))
	assert.NotEqual(t, new1, g1.Get("master").Hash)
	assert.NotEqual(t, new2, g2.Get("master").Hash)
}
