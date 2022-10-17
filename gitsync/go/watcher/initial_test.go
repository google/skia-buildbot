package watcher

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/gcs/mem_gcsclient"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/gitiles"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/mocks"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

func TestInitialIngestCommitBatch(t *testing.T) {

	ctx := cipd_git.UseGitFinder(context.Background())
	ri := repograph.NewMemCacheRepoImpl(nil, nil)
	graph, err := repograph.NewWithRepoImpl(ctx, ri)
	require.NoError(t, err)
	require.Equal(t, 0, graph.Len())
	require.Equal(t, 0, len(graph.Branches()))

	gb := git_testutils.GitInit(t, ctx)
	defer gb.Cleanup()
	gd := git.GitDir(gb.Dir())

	commit := func() *vcsinfo.LongCommit {
		h := gb.CommitGen(ctx, uuid.New().String())
		d, err := gd.Details(ctx, h)
		require.NoError(t, err)
		return d
	}

	test := func(cb *commitBatch, expectCommits, expectBranches int) {
		require.NoError(t, initialIngestCommitBatch(ctx, graph, ri, cb))
		require.Equal(t, expectCommits, graph.Len())
		require.Equal(t, expectBranches, len(graph.Branches()))
	}

	// Empty batch, nothing to do.
	test(&commitBatch{}, 0, 0)

	// Verify that we create a new branch.
	c0 := commit()
	test(&commitBatch{
		branch:  "mybranch", // Don't use the main branch, to make sure we didn't pick it up accidentally.
		commits: []*vcsinfo.LongCommit{c0},
	}, 1, 1)
	require.True(t, util.In("mybranch", graph.Branches()))

	// Verify that we walk the branch head forward for new commits.
	c1 := commit()
	test(&commitBatch{
		branch:  "mybranch",
		commits: []*vcsinfo.LongCommit{c1},
	}, 2, 1)
	require.Equal(t, c1.Hash, graph.BranchHeads()[0].Head)
	require.False(t, isFakeBranch(graph.BranchHeads()[0].Name))

	// Add two commits, both based at c1. Ensure that we create a fake
	// branch for the second one.
	c2 := commit()
	gb.CreateBranchTrackBranch(ctx, "mybranch2", git.MainBranch)
	gb.Reset(ctx, "--hard", c1.Hash)
	c3 := commit()
	test(&commitBatch{
		branch:  "mybranch",
		commits: []*vcsinfo.LongCommit{c2, c3},
	}, 4, 2)
	var fakeBranch string
	for _, b := range graph.BranchHeads() {
		if b.Name == "mybranch" {
			require.False(t, isFakeBranch(b.Name))
			require.Equal(t, c2.Hash, b.Head)
		} else {
			fakeBranch = b.Name
			require.True(t, isFakeBranch(b.Name))
			require.Equal(t, c3.Hash, b.Head)
		}
	}
	require.NotEqual(t, "", fakeBranch)

	// Add another commit on each branch. Ensure that we walk both branches
	// forward as expected.
	c4 := commit()
	gb.CheckoutBranch(ctx, git.MainBranch)
	c5 := commit()
	test(&commitBatch{
		branch:  "mybranch",
		commits: []*vcsinfo.LongCommit{c4, c5},
	}, 6, 2)
	for _, b := range graph.BranchHeads() {
		if b.Name == "mybranch" {
			require.False(t, isFakeBranch(b.Name))
			require.Equal(t, c5.Hash, b.Head)
		} else {
			require.Equal(t, fakeBranch, b.Name)
			require.True(t, isFakeBranch(b.Name))
			require.Equal(t, c4.Hash, b.Head)
		}
	}

	// Another commit on each, then merge. Ensure that we kept the real
	// branch, not the fake one.
	c6 := commit()
	gb.CheckoutBranch(ctx, "mybranch2")
	c7 := commit()
	c8Hash := gb.MergeBranch(ctx, git.MainBranch)
	c8, err := gd.Details(ctx, c8Hash)
	require.NoError(t, err)
	test(&commitBatch{
		branch:  "mybranch",
		commits: []*vcsinfo.LongCommit{c6, c7, c8},
	}, 9, 1)
	b := graph.BranchHeads()[0]
	require.False(t, isFakeBranch(b.Name))
	require.Equal(t, "mybranch", b.Name)
	require.Equal(t, c8.Hash, b.Head)
}

func setupTestInitial(t *testing.T) (context.Context, *git_testutils.GitBuilder, *gitiles_testutils.MockRepo, *repoImpl, func() *vcsinfo.LongCommit, func(int, int), func()) {
	ctx := context.Background()
	g := git_testutils.GitInit(t, ctx)
	gs := mocks.GitStore{}
	gs.On("RangeByTime", ctx, vcsinfo.MinTime, vcsinfo.MaxTime, gitstore.ALL_BRANCHES).Return(nil, nil)
	gs.On("GetBranches", ctx).Return(nil, nil)
	urlMock := mockhttpclient.NewURLMock()
	mockRepo := gitiles_testutils.NewMockRepo(t, g.RepoUrl(), git.GitDir(g.Dir()), urlMock)
	repo := gitiles.NewRepo(g.RepoUrl(), urlMock.Client())
	gcsClient := mem_gcsclient.New("fake-bucket")
	ri, err := newRepoImpl(ctx, &gs, repo, gcsClient, "repo-ingestion", nil, nil, nil)
	require.NoError(t, err)

	gd := git.GitDir(g.Dir())
	commit := func() *vcsinfo.LongCommit {
		h := g.CommitGen(ctx, uuid.New().String())
		c, err := gd.Details(ctx, h)
		require.NoError(t, err)
		return c
	}

	test := func(expectBranches, expectCommits int) {
		// Clear the cache between every attempt.
		ri.(*repoImpl).MemCacheRepoImpl = repograph.NewMemCacheRepoImpl(nil, nil)
		mockRepo.MockBranches(ctx)
		require.NoError(t, ri.(*repoImpl).initialIngestion(ctx))
		require.Equal(t, expectBranches, len(ri.(*repoImpl).BranchList))
		require.Equal(t, expectCommits, len(ri.(*repoImpl).Commits))
		require.True(t, mockRepo.Empty())
	}

	return ctx, g, mockRepo, ri.(*repoImpl), commit, test, g.Cleanup
}

func TestInitialIngestion(t *testing.T) {
	ctx, gb, mockRepo, ri, commit, test, cleanup := setupTestInitial(t)
	defer cleanup()

	// No commits, no branches; nothing to do.
	test(0, 0)

	// One commit.
	c0 := commit()
	mockRepo.MockLog(ctx, c0.Hash, gitiles.LogReverse(), gitiles.LogBatchSize(batchSize))
	test(1, 1)
	require.Equal(t, git.MainBranch, ri.BranchList[0].Name)
	require.Equal(t, c0.Hash, ri.BranchList[0].Head)
	assertdeep.Equal(t, ri.Commits[c0.Hash], c0)

	// No new commits. Clear out the cache and ensure that we don't request
	// the log of c0 again, because it's backed up in GCS.
	test(1, 1)

	// New commits on a non-main branch.
	gb.CreateBranchTrackBranch(ctx, "branch2", git.MainBranch)
	var newBranchCommits []*vcsinfo.LongCommit
	for i := 0; i < 10; i++ {
		newBranchCommits = append(newBranchCommits, commit())
	}
	last := newBranchCommits[len(newBranchCommits)-1]
	mockRepo.MockLog(ctx, last.Hash, gitiles.LogBatchSize(batchSize))
	test(2, 11)
	for _, b := range ri.BranchList {
		if b.Name == git.MainBranch {
			require.Equal(t, c0.Hash, b.Head)
		} else {
			require.Equal(t, "branch2", b.Name)
			require.Equal(t, last.Hash, b.Head)
		}
	}

	// New commits on several new branches.
	for i := 0; i < 10; i++ {
		gb.CreateBranchTrackBranch(ctx, fmt.Sprintf("b%d", i), git.MainBranch)
		commits := []*vcsinfo.LongCommit{}
		for j := 0; j < 10; j++ {
			commits = append(commits, commit())
		}
		last = commits[len(commits)-1]
		mockRepo.MockLog(ctx, last.Hash, gitiles.LogBatchSize(batchSize))
	}
	test(12, 111)

	// One new commit on one of the branches. Ensure that we only request
	// the new commit.
	mockRepo.MockLog(ctx, git.LogFromTo(last.Hash, commit().Hash), gitiles.LogBatchSize(batchSize))
	test(12, 112)
}

func TestInitialIngestionRespectsIncludeBranches(t *testing.T) {
	ctx, gb, mockRepo, ri, commit, test, cleanup := setupTestInitial(t)
	defer cleanup()

	// Set includeBranches.
	ri.includeBranches = []string{"branch2"}

	// "branch2" has two commits, while the main branch has three.
	commit()
	gb.CreateBranchTrackBranch(ctx, "branch2", git.MainBranch)
	branch2Head := commit()
	gb.CheckoutBranch(ctx, git.MainBranch)
	commit()
	commit()

	mockRepo.MockLog(ctx, branch2Head.Hash, gitiles.LogBatchSize(batchSize))
	test(1, 2)
}
