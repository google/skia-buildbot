package watcher

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/gcs/mem_gcsclient"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	repograph_shared_tests "go.skia.org/infra/go/git/repograph/shared_tests"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/gitiles"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/gitstore"
	gitstore_testutils "go.skia.org/infra/go/gitstore/bt_gitstore/testutils"
	"go.skia.org/infra/go/gitstore/mocks"
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
				Branches: map[string]bool{git.DefaultBranch: true},
			})
			idx++
		}
		return rv
	}

	totalIngested := 0
	assertNew := func(numNew int) {
		totalIngested += numNew
		require.Equal(t, totalIngested, len(ri.Commits))
	}
	// There's a data race between the mocked GitStore's use of reflection
	// and the lock/unlock of a mutex in Context.Err(). We add a mutex here
	// to avoid that problem.
	var ctxMtx sync.Mutex
	process := func(ctx context.Context, cb *commitBatch) error {
		ctxMtx.Lock()
		err := ri.gitstore.Put(ctx, cb.commits)
		ctxMtx.Unlock()
		if err != nil {
			return err
		}
		for _, c := range cb.commits {
			ri.Commits[c.Hash] = c
		}
		return nil
	}
	// Ingest a single commit.
	require.NoError(t, ri.processCommits(ctx, process, func(ctx context.Context, ch chan<- *commitBatch) error {
		commits := makeCommits("abc123")
		gs.On("Put", ctx, commits).Return(nil)
		gs.On("PutBranches", ctx, map[string]string{git.DefaultBranch: commits[len(commits)-1].Hash}).Return(nil)
		ch <- &commitBatch{
			commits: commits,
		}
		return nil
	}))
	assertNew(1)

	// Ingest a series of commits.
	require.NoError(t, ri.processCommits(ctx, process, func(ctx context.Context, ch chan<- *commitBatch) error {
		for i := 1; i < 5; i++ {
			hashes := make([]string, 0, i)
			for j := 0; j < i; j++ {
				hashes = append(hashes, fmt.Sprintf("%dabc%d", i, j))
			}
			commits := makeCommits(hashes...)
			gs.On("Put", ctx, commits).Return(nil)
			gs.On("PutBranches", ctx, map[string]string{git.DefaultBranch: commits[len(commits)-1].Hash}).Return(nil)
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
	require.Equal(t, err, ri.processCommits(ctx, process, func(ctx context.Context, ch chan<- *commitBatch) error {
		commits := makeCommits("def456")
		gs.On("Put", ctx, commits).Return(nil)
		gs.On("PutBranches", ctx, map[string]string{git.DefaultBranch: commits[len(commits)-1].Hash}).Return(nil)
		ch <- &commitBatch{
			commits: commits,
		}
		return err
	}))
	assertNew(1)

	// Ensure that the context gets canceled if ingestion fails.
	err = ri.processCommits(ctx, process, func(ctx context.Context, ch chan<- *commitBatch) error {
		for i := 5; i < 10; i++ {
			// See the above comment about mocks, reflect, and race.
			ctxMtx.Lock()
			err := ctx.Err()
			ctxMtx.Unlock()
			if err != nil {
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
				gs.On("PutBranches", ctx, map[string]string{git.DefaultBranch: commits[len(commits)-1].Hash}).Return(nil)
			}
			ch <- &commitBatch{
				commits: commits,
			}
		}
		return nil
	})
	require.True(t, strings.Contains(err.Error(), "commit ingestion failed"))
	require.True(t, strings.Contains(err.Error(), "and commit-loading func failed with: context canceled"))
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

func newGitsyncRefresher(t *testing.T, ctx context.Context, gs gitstore.GitStore, gb *git_testutils.GitBuilder, mr *gitiles_testutils.MockRepo) *gitsyncRefresher {
	repo := &git.Repo{GitDir: git.GitDir(gb.Dir())}
	branches, err := repo.Branches(ctx)
	require.NoError(t, err)
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

	require.True(u.t, u.gitiles.Empty())

	// Check the GitStore contents before updating the underlying repo.
	u.checkIngestion(ctx)

	// Update the backing repo.
	require.NoError(u.t, u.repo.Update(ctx))

	// Mock calls to gitiles.
	branches, err := u.repo.Branches(ctx)
	require.NoError(u.t, err)
	branchMap := make(map[string]string, len(branches))
	for _, b := range branches {
		oldHead := u.oldBranches[b.Name]
		if b.Head != oldHead {
			logExpr := b.Head
			if oldHead != "" {
				logExpr = fmt.Sprintf("%s..%s", oldHead, b.Head)
			}
			var opts []gitiles.LogOption
			if u.initialSync && b.Name == git.DefaultBranch {
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
	require.NoError(u.t, testutils.EventuallyConsistent(time.Second, func() error {
		actual, err := u.gs.GetBranches(ctx)
		require.NoError(u.t, err)
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
	require.NoError(u.t, err)
	delete(gotBranches, gitstore.ALL_BRANCHES)
	require.Equal(u.t, len(expectBranches), len(gotBranches))
	for name, head := range expectBranches {
		require.Equal(u.t, head, gotBranches[name].Head)
	}

	// Assert that all LongCommits are present and correct.
	iCommits, err := u.gs.RangeByTime(ctx, vcsinfo.MinTime, vcsinfo.MaxTime, gitstore.ALL_BRANCHES)
	require.NoError(u.t, err)
	hashes := make([]string, 0, len(iCommits))
	for _, c := range iCommits {
		hashes = append(hashes, c.Hash)
	}
	longCommits, err := u.gs.Get(ctx, hashes)
	require.NoError(u.t, err)
	commits := make(map[string]*vcsinfo.LongCommit, len(hashes))
	for _, c := range longCommits {
		require.NotNil(u.t, c)
		commits[c.Hash] = c
	}
	for _, c := range u.graph.GetAll() {
		assertdeep.Equal(u.t, c.LongCommit, commits[c.Hash])
	}

	// Assert that the IndexCommits are correct for each branch.
	for name := range expectBranches {
		branchPtr := gotBranches[name]
		branchCommits, err := u.graph.LogLinear("", name)
		require.NoError(u.t, err)
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
		require.NoError(u.t, err)
		assertdeep.Equal(u.t, expectIndexCommits, gotIndexCommits)

		// RangeByTime.
		gotIndexCommits, err = u.gs.RangeByTime(ctx, vcsinfo.MinTime, vcsinfo.MaxTime, name)
		require.NoError(u.t, err)
		assertdeep.Equal(u.t, expectIndexCommits, gotIndexCommits)
	}
}

// setupGitsync performs common setup for GitStore based Graphs.
func setupGitsync(t *testing.T) (context.Context, *git_testutils.GitBuilder, *repograph.Graph, *gitsyncRefresher, func()) {
	ctx, g, cleanup := repograph_shared_tests.CommonSetup(t)
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer util.RemoveAll(wd)
	_, _, gs := gitstore_testutils.SetupAndLoadBTGitStore(t, ctx, wd, g.RepoUrl(), true)
	urlMock := mockhttpclient.NewURLMock()
	mockRepo := gitiles_testutils.NewMockRepo(t, g.RepoUrl(), git.GitDir(g.Dir()), urlMock)
	repo := gitiles.NewRepo(g.RepoUrl(), urlMock.Client())
	gcsClient := mem_gcsclient.New("fake-bucket")
	ri, err := newRepoImpl(ctx, gs, repo, gcsClient, "repo-ingestion", nil)
	require.NoError(t, err)
	ud := newGitsyncRefresher(t, ctx, gs, g, mockRepo)
	graph, err := repograph.NewWithRepoImpl(ctx, ri)
	require.NoError(t, err)
	ud.graph = graph
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

func TestMissingOldBranchHeadFallback(t *testing.T) {
	unittest.LargeTest(t)

	ctx, g, repo, ud, cleanup := setupGitsync(t)
	defer cleanup()

	// Initial update.
	orig := g.CommitGen(ctx, "fake")
	deleted := g.CommitGen(ctx, "fake")
	ud.gitiles.MockBranches(ctx)
	ud.gitiles.MockBranches(ctx) // Initial Update() loads branches twice.
	ud.gitiles.MockLog(ctx, deleted, gitiles.LogReverse(), gitiles.LogBatchSize(batchSize))
	require.NoError(t, repo.Update(ctx))
	branches, err := ud.gs.GetBranches(ctx)
	require.NoError(t, err)
	assertdeep.Equal(t, map[string]*gitstore.BranchPointer{
		git.DefaultBranch: {
			Head:  deleted,
			Index: 1,
		},
	}, branches)

	// Change history.
	g.Git(ctx, "reset", "--hard", "HEAD^")
	require.Equal(t, orig, strings.TrimSpace(g.Git(ctx, "rev-parse", "HEAD")))
	next := g.CommitGen(ctx, "fake")
	ud.gitiles.MockBranches(ctx)
	ud.gitiles.URLMock.MockOnce(fmt.Sprintf(gitiles.LogURL, g.RepoUrl(), git.LogFromTo(deleted, next)), mockhttpclient.MockGetError("404 Not Found", http.StatusNotFound))
	ud.gitiles.MockLog(ctx, next)
	require.NoError(t, repo.Update(ctx))
	require.True(t, ud.gitiles.Empty())
}
