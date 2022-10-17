package repograph_test

import (
	"context"
	"errors"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/git/repograph/shared_tests"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

func TestFindCommit(t *testing.T) {
	ctx1, g1, repo1, rf1, cleanup1 := setupRepo(t)
	defer cleanup1()
	commits1 := shared_tests.GitSetup(t, ctx1, g1, repo1, rf1)
	ctx2, g2, repo2, rf2, cleanup2 := setupRepo(t)
	defer cleanup2()
	// Use a different random seed for the second repo, to ensure that we
	// end up with different commit hashes.
	g2.Seed(42)
	commits2 := shared_tests.GitSetup(t, ctx2, g2, repo2, rf2)

	m := repograph.Map{
		g1.Dir(): repo1,
		g2.Dir(): repo2,
	}

	tc := []struct {
		hash string
		url  string
		repo *repograph.Graph
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
			require.Error(t, err)
		} else {
			require.Nil(t, err)
			require.NotNil(t, commit)
			require.Equal(t, c.hash, commit.Hash)
			require.Equal(t, c.url, url)
			require.Equal(t, c.repo, repo)
		}
	}
}

// checkTopoSortCommits sorts the given commits topologically and asserts that
// the final ordering is correct.
func checkTopoSortCommits(t *testing.T, commits []*repograph.Commit) {
	sorted := repograph.TopologicalSort(commits)

	// Ensure that all of the commits are in the resulting slice.
	require.Equal(t, len(commits), len(sorted))
	found := make(map[*repograph.Commit]bool, len(commits))
	for _, c := range sorted {
		found[c] = true
	}
	for _, c := range commits {
		require.True(t, found[c])
	}
	shared_tests.AssertTopoSorted(t, sorted)
}

// checkTopoSortGraph takes every combination of commits from the given Graph
// and sorts them topologically, asserting that the ordering is correct.
func checkTopoSortGraph(t *testing.T, g *repograph.Graph) {
	commitsMap := g.GetAll()
	commitsList := make([]*repograph.Commit, 0, len(commitsMap))
	for _, c := range commitsMap {
		commitsList = append(commitsList, c)
	}
	sets := util.PowerSet(len(commitsList))
	for _, set := range sets {
		inp := make([]*repograph.Commit, 0, len(commitsList))
		for _, idx := range set {
			inp = append(inp, commitsList[idx])
		}
		checkTopoSortCommits(t, inp)
	}
}

// checkTopoSortGitBuilder creates a Graph from the given GitBuilder and
// performs topological sorting tests on it.
func checkTopoSortGitBuilder(t *testing.T, ctx context.Context, gb *git_testutils.GitBuilder, expect []string) {
	tmpDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)
	g, err := repograph.NewLocalGraph(ctx, gb.Dir(), tmpDir)
	require.NoError(t, err)
	require.NoError(t, g.Update(ctx))
	checkTopoSortGraph(t, g)

	// Test stability by verifying that we get the expected ordering
	// for the entire repo, multiple times.
	commitsMap := g.GetAll()
	commitsList := make([]*repograph.Commit, 0, len(commitsMap))
	for _, c := range commitsMap {
		commitsList = append(commitsList, c)
	}
	rng := rand.New(rand.NewSource(time.Now().Unix()))
	for i := 0; i < 10; i++ {
		// Randomly permute the slice.
		commits := make([]*repograph.Commit, 0, len(commitsList))
		for _, index := range rng.Perm(len(commitsList)) {
			commits = append(commits, commitsList[index])
		}
		sorted := repograph.CommitSlice(repograph.TopologicalSort(commitsList)).Hashes()
		assertdeep.Equal(t, expect, sorted)
	}
}

// TestTopoSort tests repograph.TopologicalSort using the default test repo.
func TestTopoSortDefault(t *testing.T) {
	ctx := cipd_git.UseGitFinder(context.Background())
	gb := git_testutils.GitInit(t, ctx)
	defer gb.Cleanup()
	commits := git_testutils.GitSetup(ctx, gb)

	// GitSetup doesn't wait between commits, which means that the
	// timestamps might be equal. Adjust the expectations
	// accordingly.
	expect := []string{commits[4], commits[3], commits[2], commits[1], commits[0]}
	tmpDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)
	g, err := repograph.NewLocalGraph(ctx, gb.Dir(), tmpDir)
	require.NoError(t, err)
	require.NoError(t, g.Update(ctx))
	c3 := g.Get(commits[3])
	c2 := g.Get(commits[2])
	if c3.Timestamp.Equal(c2.Timestamp) && c3.Hash < c2.Hash {
		expect[1], expect[2] = expect[2], expect[1]
	}
	checkTopoSortGitBuilder(t, ctx, gb, expect)
}

// TestTopoSortTimestamp verifies that we use the timestamp as a tie-breaker.
func TestTopoSortTimestamp(t *testing.T) {
	ctx := cipd_git.UseGitFinder(context.Background())
	gb := git_testutils.GitInit(t, ctx)
	defer gb.Cleanup()
	gb.Add(ctx, "file0", "contents")
	ts := time.Unix(1552403492, 0)
	c0 := gb.CommitMsgAt(ctx, "Initial commit", ts)
	require.Equal(t, c0, "c48b90c8ccc70b4d2bd146e4f708c398f78e2dd6") // Hashes are deterministic.
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
	checkTopoSortGitBuilder(t, ctx, gb, []string{c2, c1, c0})
}

// TestTopoSortCommitHash is similar to TestTopoSortTimestamp, but the child
// commits have the same timestamp. Verify that we use the commit hash as a
// secondary tie-breaker.
func TestTopoSortCommitHash(t *testing.T) {
	ctx := cipd_git.UseGitFinder(context.Background())
	gb := git_testutils.GitInit(t, ctx)
	defer gb.Cleanup()
	gb.Add(ctx, "file0", "contents")
	ts := time.Unix(1552403492, 0)
	c0 := gb.CommitMsgAt(ctx, "Initial commit", ts)
	require.Equal(t, c0, "c48b90c8ccc70b4d2bd146e4f708c398f78e2dd6") // Hashes are deterministic.
	ts = ts.Add(2 * time.Second)
	gb.Add(ctx, "file1", "contents")
	c1 := gb.CommitMsgAt(ctx, "Child 1", ts)
	require.Equal(t, c1, "dc24e5b042cdcf995a182815ef37f659e8ec20cc")
	gb.CreateBranchAtCommit(ctx, "otherbranch", c0)
	gb.Add(ctx, "file2", "contents")
	c2 := gb.CommitMsgAt(ctx, "Child 2", ts)
	require.Equal(t, c2, "06d29d7f828723c79c9eba25696111c628ab5a7e")
	// c1 and c2 both have c0 as a parent, and they both have the
	// same timestamp. The topological ordering is ambiguous even
	// with the timestamp as a tie-breaker, so we have to use the
	// commit hash.
	checkTopoSortGitBuilder(t, ctx, gb, []string{c1, c2, c0})
}

// TestTopoSortMergeTimestamp extends TestTopoSortCommitHash to ensure that, in
// the case of a merge, we follow the parent with the newer timestamp.
func TestTopoSortMergeTimestamp(t *testing.T) {
	ctx := cipd_git.UseGitFinder(context.Background())
	gb := git_testutils.GitInit(t, ctx)
	defer gb.Cleanup()
	gb.Add(ctx, "file0", "contents")
	ts := time.Unix(1552403492, 0)
	c0 := gb.CommitMsgAt(ctx, "Initial commit", ts)
	require.Equal(t, c0, "c48b90c8ccc70b4d2bd146e4f708c398f78e2dd6") // Hashes are deterministic.
	ts = ts.Add(2 * time.Second)
	gb.Add(ctx, "file1", "contents")
	c1 := gb.CommitMsgAt(ctx, "Child 1", ts)
	require.Equal(t, c1, "dc24e5b042cdcf995a182815ef37f659e8ec20cc")
	gb.CreateBranchAtCommit(ctx, "otherbranch", c0)
	gb.Add(ctx, "file2", "contents")
	c2 := gb.CommitMsgAt(ctx, "Child 2", ts)
	require.Equal(t, c2, "06d29d7f828723c79c9eba25696111c628ab5a7e")
	gb.CheckoutBranch(ctx, git.MainBranch)
	c3 := gb.CommitGen(ctx, "file1")
	ts = ts.Add(10 * time.Second)
	c4 := gb.CommitGenAt(ctx, "file1", ts)
	gb.CheckoutBranch(ctx, "otherbranch")
	c5 := gb.CommitGen(ctx, "file2")
	c6 := gb.CommitGenAt(ctx, "file2", ts.Add(-time.Second))
	gb.CheckoutBranch(ctx, git.MainBranch)
	c7 := gb.MergeBranch(ctx, "otherbranch")
	checkTopoSortGitBuilder(t, ctx, gb, []string{c7, c4, c3, c1, c6, c5, c2, c0})
}

func TestIsAncestor(t *testing.T) {
	ctx := cipd_git.UseGitFinder(context.Background())
	gb := git_testutils.GitInit(t, ctx)
	defer gb.Cleanup()
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

	checkIsAncestor := func(hashA, hashB string, expect bool) {
		// Compare against actual git.
		got, err := co.IsAncestor(ctx, hashA, hashB)
		require.NoError(t, err)
		require.Equal(t, expect, got)
		got, err = g.IsAncestor(hashA, hashB)
		require.NoError(t, err)
		require.Equal(t, expect, got)
	}
	checkIsAncestor(commits[0], commits[0], true)
	checkIsAncestor(commits[0], commits[1], true)
	checkIsAncestor(commits[0], commits[2], true)
	checkIsAncestor(commits[0], commits[3], true)
	checkIsAncestor(commits[0], commits[4], true)

	checkIsAncestor(commits[1], commits[0], false)
	checkIsAncestor(commits[1], commits[1], true)
	checkIsAncestor(commits[1], commits[2], true)
	checkIsAncestor(commits[1], commits[3], true)
	checkIsAncestor(commits[1], commits[4], true)

	checkIsAncestor(commits[2], commits[0], false)
	checkIsAncestor(commits[2], commits[1], false)
	checkIsAncestor(commits[2], commits[2], true)
	checkIsAncestor(commits[2], commits[3], false)
	checkIsAncestor(commits[2], commits[4], true)

	checkIsAncestor(commits[3], commits[0], false)
	checkIsAncestor(commits[3], commits[1], false)
	checkIsAncestor(commits[3], commits[2], false)
	checkIsAncestor(commits[3], commits[3], true)
	checkIsAncestor(commits[3], commits[4], true)

	checkIsAncestor(commits[4], commits[0], false)
	checkIsAncestor(commits[4], commits[1], false)
	checkIsAncestor(commits[4], commits[2], false)
	checkIsAncestor(commits[4], commits[3], false)
	checkIsAncestor(commits[4], commits[4], true)
}

func TestMapUpdate(t *testing.T) {
	ctx, gb1, g1, r1, cleanup1 := setupRepo(t)
	defer cleanup1()
	_, gb2, g2, r2, cleanup2 := setupRepo(t)
	var cleanup2Once sync.Once
	defer cleanup2Once.Do(cleanup2)
	m := repograph.Map(map[string]*repograph.Graph{
		gb1.RepoUrl(): g1,
		gb2.RepoUrl(): g2,
	})

	// 1. Verify that updating a Map actually updates each Graph.
	new1 := gb1.CommitGen(ctx, "f")
	new2 := gb2.CommitGen(ctx, "f")
	r1.Refresh()
	r2.Refresh()
	require.NoError(t, m.Update(ctx))
	require.Equal(t, new1, g1.Get(git.MainBranch).Hash)
	require.Equal(t, new2, g2.Get(git.MainBranch).Hash)

	// 2. Verify that none of the changes are committed if a callback fails.
	new1 = gb1.CommitGen(ctx, "f")
	new2 = gb2.CommitGen(ctx, "f")
	r1.Refresh()
	r2.Refresh()
	failNext := false
	require.EqualError(t, m.UpdateWithCallback(ctx, func(repoUrl string, g *repograph.Graph) error {
		if failNext {
			return errors.New("Fail")
		}
		failNext = true
		return nil
	}), "Fail")
	require.NotEqual(t, new1, g1.Get(git.MainBranch).Hash)
	require.NotEqual(t, new2, g2.Get(git.MainBranch).Hash)

	// 3. Verify that none of the changes are committed if an update fails.
	cleanup2Once.Do(cleanup2)
	r1.Refresh()
	r2.Refresh()
	require.Error(t, m.Update(ctx))
	require.NotEqual(t, new1, g1.Get(git.MainBranch).Hash)
	require.NotEqual(t, new2, g2.Get(git.MainBranch).Hash)
}
