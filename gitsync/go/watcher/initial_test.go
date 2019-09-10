package watcher

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/gitiles"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/mocks"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

func TestInitialIngestCommitBatch(t *testing.T) {
	unittest.MediumTest(t)

	ctx := context.Background()
	ri := repograph.NewMemCacheRepoImpl(nil, nil)
	graph, err := repograph.NewWithRepoImpl(ctx, ri)
	assert.NoError(t, err)
	assert.Equal(t, 0, graph.Len())
	assert.Equal(t, 0, len(graph.Branches()))

	gb := git_testutils.GitInit(t, ctx)
	defer gb.Cleanup()
	gd := git.GitDir(gb.Dir())

	commit := func() *vcsinfo.LongCommit {
		h := gb.CommitGen(ctx, uuid.New().String())
		d, err := gd.Details(ctx, h)
		assert.NoError(t, err)
		return d
	}

	test := func(cb *commitBatch, expectCommits, expectBranches int) {
		assert.NoError(t, initialIngestCommitBatch(ctx, graph, ri, cb))
		assert.Equal(t, expectCommits, graph.Len())
		assert.Equal(t, expectBranches, len(graph.Branches()))
	}

	// Empty batch, nothing to do.
	test(&commitBatch{}, 0, 0)

	// Verify that we create a new branch.
	c0 := commit()
	test(&commitBatch{
		branch:  "mybranch", // Don't use master, to make sure we didn't pick it up accidentally.
		commits: []*vcsinfo.LongCommit{c0},
	}, 1, 1)
	assert.True(t, util.In("mybranch", graph.Branches()))

	// Verify that we walk the branch head forward for new commits.
	c1 := commit()
	test(&commitBatch{
		branch:  "mybranch",
		commits: []*vcsinfo.LongCommit{c1},
	}, 2, 1)
	assert.Equal(t, c1.Hash, graph.BranchHeads()[0].Head)
	assert.False(t, isFakeBranch(graph.BranchHeads()[0].Name))

	// Add two commits, both based at c1. Ensure that we create a fake
	// branch for the second one.
	c2 := commit()
	gb.CreateBranchTrackBranch(ctx, "mybranch2", "master")
	gb.Reset(ctx, "--hard", c1.Hash)
	c3 := commit()
	test(&commitBatch{
		branch:  "mybranch",
		commits: []*vcsinfo.LongCommit{c2, c3},
	}, 4, 2)
	var fakeBranch string
	for _, b := range graph.BranchHeads() {
		if b.Name == "mybranch" {
			assert.False(t, isFakeBranch(b.Name))
			assert.Equal(t, c2.Hash, b.Head)
		} else {
			fakeBranch = b.Name
			assert.True(t, isFakeBranch(b.Name))
			assert.Equal(t, c3.Hash, b.Head)
		}
	}
	assert.NotEqual(t, "", fakeBranch)

	// Add another commit on each branch. Ensure that we walk both branches
	// forward as expected.
	c4 := commit()
	gb.CheckoutBranch(ctx, "master")
	c5 := commit()
	test(&commitBatch{
		branch:  "mybranch",
		commits: []*vcsinfo.LongCommit{c4, c5},
	}, 6, 2)
	for _, b := range graph.BranchHeads() {
		if b.Name == "mybranch" {
			assert.False(t, isFakeBranch(b.Name))
			assert.Equal(t, c5.Hash, b.Head)
		} else {
			assert.Equal(t, fakeBranch, b.Name)
			assert.True(t, isFakeBranch(b.Name))
			assert.Equal(t, c4.Hash, b.Head)
		}
	}

	// Another commit on each, then merge. Ensure that we kept the real
	// branch, not the fake one.
	c6 := commit()
	gb.CheckoutBranch(ctx, "mybranch2")
	c7 := commit()
	c8Hash := gb.MergeBranch(ctx, "master")
	c8, err := gd.Details(ctx, c8Hash)
	assert.NoError(t, err)
	test(&commitBatch{
		branch:  "mybranch",
		commits: []*vcsinfo.LongCommit{c6, c7, c8},
	}, 9, 1)
	b := graph.BranchHeads()[0]
	assert.False(t, isFakeBranch(b.Name))
	assert.Equal(t, "mybranch", b.Name)
	assert.Equal(t, c8.Hash, b.Head)
}

func setupTestInitial(t *testing.T) (context.Context, *git_testutils.GitBuilder, *gitiles_testutils.MockRepo, *repoImpl, func()) {
	ctx := context.Background()
	g := git_testutils.GitInit(t, ctx)
	gs := mocks.GitStore{}
	gs.On("RangeByTime", ctx, vcsinfo.MinTime, vcsinfo.MaxTime, gitstore.ALL_BRANCHES).Return(nil, nil)
	gs.On("GetBranches", ctx).Return(nil, nil)
	urlMock := mockhttpclient.NewURLMock()
	mockRepo := gitiles_testutils.NewMockRepo(t, g.RepoUrl(), git.GitDir(g.Dir()), urlMock)
	repo := gitiles.NewRepo(g.RepoUrl(), urlMock.Client())
	gcsClient := test_gcsclient.NewMemoryClient("fake-bucket")
	ri, err := newRepoImpl(ctx, &gs, repo, gcsClient, "repo-ingestion", nil)
	assert.NoError(t, err)
	return ctx, g, mockRepo, ri.(*repoImpl), g.Cleanup
}

func TestInitialIngestion(t *testing.T) {
	unittest.LargeTest(t)
	ctx, gb, mockRepo, ri, cleanup := setupTestInitial(t)
	defer cleanup()

	gd := git.GitDir(gb.Dir())

	commit := func() *vcsinfo.LongCommit {
		h := gb.CommitGen(ctx, uuid.New().String())
		c, err := gd.Details(ctx, h)
		assert.NoError(t, err)
		return c
	}

	test := func(expectBranches, expectCommits int) {
		// Clear the cache between every attempt.
		ri.MemCacheRepoImpl = repograph.NewMemCacheRepoImpl(nil, nil)
		mockRepo.MockBranches(ctx)
		assert.NoError(t, ri.initialIngestion(ctx))
		assert.Equal(t, expectBranches, len(ri.BranchList))
		assert.Equal(t, expectCommits, len(ri.Commits))
		assert.True(t, mockRepo.Empty())
	}

	// No commits, no branches; nothing to do.
	test(0, 0)

	// One commit.
	c0 := commit()
	mockRepo.MockLog(ctx, c0.Hash, gitiles.LogReverse(), gitiles.LogBatchSize(batchSize))
	test(1, 1)
	assert.Equal(t, "master", ri.BranchList[0].Name)
	assert.Equal(t, c0.Hash, ri.BranchList[0].Head)
	deepequal.AssertDeepEqual(t, ri.Commits[c0.Hash], c0)

	// No new commits. Clear out the cache and ensure that we don't request
	// the log of c0 again, because it's backed up in GCS.
	test(1, 1)

	// New commits on a non-master branch.
	gb.CreateBranchTrackBranch(ctx, "branch2", "master")
	var newBranchCommits []*vcsinfo.LongCommit
	for i := 0; i < 10; i++ {
		newBranchCommits = append(newBranchCommits, commit())
	}
	last := newBranchCommits[len(newBranchCommits)-1]
	mockRepo.MockLog(ctx, last.Hash, gitiles.LogBatchSize(batchSize))
	test(2, 11)
	for _, b := range ri.BranchList {
		if b.Name == "master" {
			assert.Equal(t, c0.Hash, b.Head)
		} else {
			assert.Equal(t, "branch2", b.Name)
			assert.Equal(t, last.Hash, b.Head)
		}
	}

	// New commits on several new branches.
	for i := 0; i < 10; i++ {
		gb.CreateBranchTrackBranch(ctx, fmt.Sprintf("b%d", i), "master")
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
