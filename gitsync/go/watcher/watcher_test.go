package watcher

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	repograph_shared_tests "go.skia.org/infra/go/git/repograph/shared_tests"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/gitiles"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/mocks"
	gitstore_testutils "go.skia.org/infra/go/gitstore/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

func TestIngestCommits(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()
	gs := &mocks.GitStore{}
	ri := &repoImpl{
		MemCacheRepoImpl: repograph.NewMemCacheRepoImpl(nil, nil),
		gitstore:         gs,
	}
	idx := 0
	makeCommits := func(hashes ...string) []*vcsinfo.LongCommit {
		rv := make([]*vcsinfo.LongCommit, 0, len(hashes))
		for _, h := range hashes {
			rv = append(rv, &vcsinfo.LongCommit{
				ShortCommit: &vcsinfo.ShortCommit{
					Hash: h,
				},
				Index:    idx,
				Branches: map[string]bool{"master": true},
			})
			idx++
		}
		return rv
	}

	totalIngested := 0
	assertNew := func(numNew int) {
		totalIngested += numNew
		assert.Equal(t, totalIngested, len(ri.Commits))
	}
	process := func(ctx context.Context, cb *commitBatch) error {
		if err := ri.gitstore.Put(ctx, cb.commits); err != nil {
			return err
		}
		for _, c := range cb.commits {
			ri.Commits[c.Hash] = c
		}
		return nil
	}
	// Ingest a single commit.
	assert.NoError(t, ri.processCommits(ctx, process, func(ctx context.Context, ch chan<- *commitBatch) error {
		commits := makeCommits("abc123")
		gs.On("Put", ctx, commits).Return(nil)
		gs.On("PutBranches", ctx, map[string]string{"master": commits[len(commits)-1].Hash}).Return(nil)
		ch <- &commitBatch{
			commits: commits,
		}
		return nil
	}))
	assertNew(1)

	// Ingest a series of commits.
	assert.NoError(t, ri.processCommits(ctx, process, func(ctx context.Context, ch chan<- *commitBatch) error {
		for i := 1; i < 5; i++ {
			hashes := make([]string, 0, i)
			for j := 0; j < i; j++ {
				hashes = append(hashes, fmt.Sprintf("%dabc%d", i, j))
			}
			commits := makeCommits(hashes...)
			gs.On("Put", ctx, commits).Return(nil)
			gs.On("PutBranches", ctx, map[string]string{"master": commits[len(commits)-1].Hash}).Return(nil)
			ch <- &commitBatch{
				commits: commits,
			}
		}
		return nil
	}))
	assertNew(1 + 2 + 3 + 4)

	// If the passed-in func returns an error, it should propagate, and the
	// previously-queued commits should still get ingested.
	err := errors.New("commit retrieval failed.")
	assert.Equal(t, err, ri.processCommits(ctx, process, func(ctx context.Context, ch chan<- *commitBatch) error {
		commits := makeCommits("def456")
		gs.On("Put", ctx, commits).Return(nil)
		gs.On("PutBranches", ctx, map[string]string{"master": commits[len(commits)-1].Hash}).Return(nil)
		ch <- &commitBatch{
			commits: commits,
		}
		return err
	}))
	assertNew(1)

	// Ensure that the context gets canceled if ingestion fails.
	err = ri.processCommits(ctx, process, func(ctx context.Context, ch chan<- *commitBatch) error {
		for i := 5; i < 10; i++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			hashes := make([]string, 0, i)
			for j := 0; j < i; j++ {
				hashes = append(hashes, fmt.Sprintf("%dabc%d", i, j))
			}
			commits := makeCommits(hashes...)
			if i == 7 {
				gs.On("Put", ctx, commits).Return(errors.New("commit ingestion failed."))
			} else {
				gs.On("Put", ctx, commits).Return(nil)
				gs.On("PutBranches", ctx, map[string]string{"master": commits[len(commits)-1].Hash}).Return(nil)
			}
			ch <- &commitBatch{
				commits: commits,
			}
		}
		return nil
	})
	sklog.Errorf("Error: %s", err.Error())
	assert.True(t, strings.Contains(err.Error(), "commit ingestion failed"))
	assert.True(t, strings.Contains(err.Error(), "and commit-loading func failed with: context canceled"))
	assertNew(5 + 6)
}

// gitsyncRefresher is an implementation of repograph_shared_tests.RepoImplRefresher
// used for testing gitsync.
type gitsyncRefresher struct {
	gitiles     *gitiles_testutils.MockRepo
	graph       *repograph.Graph
	gs          gitstore.GitStore
	initialSync bool
	oldBranches map[string]string
	repo        *git.Repo
	t           *testing.T
}

func newGitsyncRefresher(t *testing.T, ctx context.Context, gs gitstore.GitStore, gb *git_testutils.GitBuilder, mr *gitiles_testutils.MockRepo) repograph_shared_tests.RepoImplRefresher {
	repo := &git.Repo{GitDir: git.GitDir(gb.Dir())}
	branches, err := repo.Branches(ctx)
	assert.NoError(t, err)
	oldBranches := make(map[string]string, len(branches))
	for _, b := range branches {
		oldBranches[b.Name] = b.Head
	}
	return &gitsyncRefresher{
		gitiles:     mr,
		graph:       nil, // Set later in setupGitsync, after the graph is created.
		gs:          gs,
		initialSync: true,
		oldBranches: oldBranches,
		repo:        repo,
		t:           t,
	}
}

func (u *gitsyncRefresher) Refresh(commits ...*vcsinfo.LongCommit) {
	ctx := context.Background()

	assert.True(u.t, u.gitiles.Empty())

	// Check the GitStore contents before updating the underlying repo.
	u.checkIngestion(ctx)

	// Update the backing repo.
	assert.NoError(u.t, u.repo.Update(ctx))

	// Mock calls to gitiles.
	branches, err := u.repo.Branches(ctx)
	assert.NoError(u.t, err)
	branchMap := make(map[string]string, len(branches))
	for _, b := range branches {
		oldHead := u.oldBranches[b.Name]
		if b.Head != oldHead {
			logExpr := b.Head
			if oldHead != "" {
				logExpr = fmt.Sprintf("%s..%s", oldHead, b.Head)
			}
			var opts []gitiles.LogOption
			if u.initialSync && b.Name == "master" {
				opts = append(opts, gitiles.LogReverse(), gitiles.LogBatchSize(batchSize))
			}
			u.gitiles.MockLog(ctx, logExpr, opts...)
		}
		branchMap[b.Name] = b.Head
	}
	u.oldBranches = branchMap
	u.gitiles.MockBranches(ctx)
	if u.initialSync {
		u.gitiles.MockBranches(ctx)
		u.initialSync = false
	}
}

// checkIngestion asserts that the contents of the GitStore match those of the
// repograph.Graph.
func (u *gitsyncRefresher) checkIngestion(ctx context.Context) {
	if u.graph == nil {
		return
	}

	// Wait for GitStore to be up to date.
	branchHeads := u.graph.BranchHeads()
	expectBranches := make(map[string]string, len(branchHeads))
	for _, b := range branchHeads {
		expectBranches[b.Name] = b.Head
	}
	assert.NoError(u.t, testutils.EventuallyConsistent(time.Second, func() error {
		actual, err := u.gs.GetBranches(ctx)
		assert.NoError(u.t, err)
		for name, expect := range expectBranches {
			actualBranch, ok := actual[name]
			if !ok || actualBranch.Head != expect {
				sklog.Errorf("%s is %+v, expect %s", name, actualBranch, expect)
				time.Sleep(10 * time.Millisecond)
				return testutils.TryAgainErr
			}
		}
		for name := range actual {
			if _, ok := expectBranches[name]; name != gitstore.ALL_BRANCHES && !ok {
				sklog.Errorf("Expected %s not to be present", name)
				time.Sleep(10 * time.Millisecond)
				return testutils.TryAgainErr
			}
		}
		return nil
	}))

	// Assert that the branch heads are the same.
	gotBranches, err := u.gs.GetBranches(ctx)
	assert.NoError(u.t, err)
	delete(gotBranches, gitstore.ALL_BRANCHES)
	assert.Equal(u.t, len(expectBranches), len(gotBranches))
	for name, head := range expectBranches {
		assert.Equal(u.t, head, gotBranches[name].Head)
	}

	// Assert that all LongCommits are present and correct.
	iCommits, err := u.gs.RangeByTime(ctx, vcsinfo.MinTime, vcsinfo.MaxTime, gitstore.ALL_BRANCHES)
	assert.NoError(u.t, err)
	hashes := make([]string, 0, len(iCommits))
	for _, c := range iCommits {
		hashes = append(hashes, c.Hash)
	}
	longCommits, err := u.gs.Get(ctx, hashes)
	assert.NoError(u.t, err)
	commits := make(map[string]*vcsinfo.LongCommit, len(hashes))
	for _, c := range longCommits {
		assert.NotNil(u.t, c)
		commits[c.Hash] = c
	}
	for _, c := range u.graph.GetAll() {
		deepequal.AssertDeepEqual(u.t, c.LongCommit, commits[c.Hash])
	}

	// Assert that the IndexCommits are correct for each branch.
	for name := range expectBranches {
		branchPtr := gotBranches[name]
		branchCommits, err := u.graph.LogLinear("", name)
		assert.NoError(u.t, err)
		expectIndexCommits := make([]*vcsinfo.IndexCommit, 0, len(branchCommits))
		for i := len(branchCommits) - 1; i >= 0; i-- {
			c := branchCommits[i]
			expectIndexCommits = append(expectIndexCommits, &vcsinfo.IndexCommit{
				Hash:      c.Hash,
				Index:     len(expectIndexCommits),
				Timestamp: c.Timestamp.UTC(),
			})
		}

		// RangeN.
		gotIndexCommits, err := u.gs.RangeN(ctx, 0, branchPtr.Index+1, name)
		assert.NoError(u.t, err)
		deepequal.AssertDeepEqual(u.t, expectIndexCommits, gotIndexCommits)

		// RangeByTime.
		gotIndexCommits, err = u.gs.RangeByTime(ctx, vcsinfo.MinTime, vcsinfo.MaxTime, name)
		assert.NoError(u.t, err)
		deepequal.AssertDeepEqual(u.t, expectIndexCommits, gotIndexCommits)
	}
}

// setupGitsync performs common setup for GitStore based Graphs.
func setupGitsync(t *testing.T) (context.Context, *git_testutils.GitBuilder, *repograph.Graph, repograph_shared_tests.RepoImplRefresher, func()) {
	ctx, g, cleanup := repograph_shared_tests.CommonSetup(t)
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer util.RemoveAll(wd)
	_, _, gs := gitstore_testutils.SetupAndLoadBTGitStore(t, ctx, wd, g.RepoUrl(), true)
	urlMock := mockhttpclient.NewURLMock()
	mockRepo := gitiles_testutils.NewMockRepo(t, g.RepoUrl(), git.GitDir(g.Dir()), urlMock)
	repo := gitiles.NewRepo(g.RepoUrl(), "", urlMock.Client())
	gcsClient := test_gcsclient.NewMemoryClient("fake-bucket")
	ri, err := newRepoImpl(ctx, gs, repo, gcsClient, "repo-ingestion")
	assert.NoError(t, err)
	ud := newGitsyncRefresher(t, ctx, gs, g, mockRepo)
	graph, err := repograph.NewWithRepoImpl(ctx, ri)
	assert.NoError(t, err)
	ud.(*gitsyncRefresher).graph = graph
	return ctx, g, graph, ud, cleanup
}

func TestGraphWellFormedGitSync(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitsync(t)
	defer cleanup()
	repograph_shared_tests.TestGraphWellFormed(t, ctx, g, repo, ud)
}

func TestRecurseGitSync(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitsync(t)
	defer cleanup()
	repograph_shared_tests.TestRecurse(t, ctx, g, repo, ud)
}

func TestRecurseAllBranchesGitSync(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitsync(t)
	defer cleanup()
	repograph_shared_tests.TestRecurseAllBranches(t, ctx, g, repo, ud)
}

func TestUpdateHistoryChangedGitSync(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitsync(t)
	defer cleanup()
	repograph_shared_tests.TestUpdateHistoryChanged(t, ctx, g, repo, ud)
}

func TestUpdateAndReturnCommitDiffsGitSync(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitsync(t)
	defer cleanup()
	repograph_shared_tests.TestUpdateAndReturnCommitDiffs(t, ctx, g, repo, ud)
}

func TestRevListGitSync(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitsync(t)
	defer cleanup()
	repograph_shared_tests.TestRevList(t, ctx, g, repo, ud)
}

func TestBranchMembershipGitSync(t *testing.T) {
	unittest.LargeTest(t)
	ctx, g, repo, ud, cleanup := setupGitsync(t)
	defer cleanup()
	repograph_shared_tests.TestBranchMembership(t, ctx, g, repo, ud)
}
