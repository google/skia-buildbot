package sse

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/status/go/rpc"
	"go.skia.org/infra/task_scheduler/go/db"
	cache_mocks "go.skia.org/infra/task_scheduler/go/db/cache/mocks"
	db_mocks "go.skia.org/infra/task_scheduler/go/mocks"
	"go.skia.org/infra/task_scheduler/go/types"
	window_mocks "go.skia.org/infra/task_scheduler/go/window/mocks"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type testContext struct {
	ctx        context.Context
	repo       *repograph.Graph
	mockDB     *db_mocks.RemoteDB
	mockCache  *cache_mocks.TaskCache
	mockWindow *window_mocks.Window
	server     *SSEServer
	memRepo    *repograph.MemCacheRepoImpl
}

func makeMemCommit(hash, author, subject string, timestamp time.Time, parents ...string) *vcsinfo.LongCommit {
	return &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    hash,
			Author:  author,
			Subject: subject,
		},
		Parents:   parents,
		Timestamp: timestamp,
	}
}

func setupTestServer(t *testing.T) *testContext {
	ctx := context.Background()

	c0 := "commit0"
	commits := map[string]*vcsinfo.LongCommit{
		c0: makeMemCommit(c0, "me@google.com", "file1", time.Now().Add(-10*time.Minute)),
	}
	branches := []*git.Branch{
		{
			Name: "main",
			Head: c0,
		},
	}

	ri := repograph.NewMemCacheRepoImpl(commits, branches)
	repo, err := repograph.NewWithRepoImpl(ctx, ri)
	require.NoError(t, err)

	mockDB := &db_mocks.RemoteDB{}
	ch := make(chan []*types.Task)
	mockDB.On("ModifiedTasksCh", mock.Anything).Return((<-chan []*types.Task)(ch))

	mockCache := &cache_mocks.TaskCache{}
	mockWindow := &window_mocks.Window{}

	cfg := Config{
		Repos:    repograph.Map{"https://repo.git": repo},
		TaskDb:   mockDB,
		TCache:   mockCache,
		Window:   mockWindow,
		RepoUrls: []string{"https://repo.git"},
		GetRepoTwirp: func(name string) (string, string, error) {
			if name == "unknown" {
				return "", "", fmt.Errorf("unknown repo")
			}
			return "", "https://repo.git", nil
		},
		RepoUrlToName:        func(url string) string { return "foo" },
		DefaultCommitsToLoad: 10,
		MaxCommitsToLoad:     100,
	}

	s, err := New(ctx, cfg)
	require.NoError(t, err)

	return &testContext{
		ctx:        ctx,
		repo:       repo,
		mockDB:     mockDB,
		mockCache:  mockCache,
		mockWindow: mockWindow,
		server:     s,
		memRepo:    ri,
	}
}

func TestQueryCommitsAndReachability(t *testing.T) {
	ctx := context.Background()

	// Create commit tree in memory:
	// c0 -> c1 -> c2 [main]
	//         \
	//          -> f1 -> f2 [feature]
	c0 := "c0"
	c1 := "c1"
	c2 := "c2"
	f1 := "f1"
	f2 := "f2"

	t0 := time.Now().Add(-10 * time.Minute)
	t1 := time.Now().Add(-8 * time.Minute)
	t2 := time.Now().Add(-6 * time.Minute)
	t3 := time.Now().Add(-4 * time.Minute)
	t4 := time.Now().Add(-2 * time.Minute)

	commits := map[string]*vcsinfo.LongCommit{
		c0: makeMemCommit(c0, "me@google.com", "c0", t0),
		c1: makeMemCommit(c1, "me@google.com", "c1", t1, c0),
		c2: makeMemCommit(c2, "me@google.com", "c2", t2, c1),
		f1: makeMemCommit(f1, "me@google.com", "f1", t3, c1),
		f2: makeMemCommit(f2, "me@google.com", "f2", t4, f1),
	}

	branches := []*git.Branch{
		{Name: "main", Head: c2},
		{Name: "feature", Head: f2},
	}

	ri := repograph.NewMemCacheRepoImpl(commits, branches)
	repo, err := repograph.NewWithRepoImpl(ctx, ri)
	require.NoError(t, err)

	repos := repograph.Map{
		"https://repo.git": repo,
	}

	mockDB := &db_mocks.RemoteDB{}
	ch := make(chan []*types.Task)
	mockDB.On("ModifiedTasksCh", mock.Anything).Return((<-chan []*types.Task)(ch))

	mockCache := &cache_mocks.TaskCache{}
	mockWindow := &window_mocks.Window{}

	cfg := Config{
		Repos:                repos,
		TaskDb:               mockDB,
		TCache:               mockCache,
		Window:               mockWindow,
		RepoUrls:             []string{"https://repo.git"},
		GetRepoTwirp:         func(name string) (string, string, error) { return "", "https://repo.git", nil },
		RepoUrlToName:        func(url string) string { return "foo" },
		DefaultCommitsToLoad: 10,
		MaxCommitsToLoad:     100,
	}
	s, err := New(ctx, cfg)
	require.NoError(t, err)

	// No regex or cursor.
	t.Run("defaults", func(t *testing.T) {
		resCommits, branchHeads, err := s.queryCommits(repo, nil, "", 10)
		require.NoError(t, err)
		require.Len(t, resCommits, 5)
		require.Equal(t, f2, resCommits[0].Hash)
		require.Equal(t, f1, resCommits[1].Hash)
		require.Equal(t, c2, resCommits[2].Hash)
		require.Equal(t, c1, resCommits[3].Hash)
		require.Equal(t, c0, resCommits[4].Hash)
		require.Equal(t, []*git.Branch{
			{
				Name: "feature",
				Head: f2,
			},
			{
				Name: "main",
				Head: c2,
			},
		}, branchHeads)
	})

	// Regex matches "main" branch.
	t.Run("branch regex: main", func(t *testing.T) {
		mainRegex := regexp.MustCompile("main")
		resCommits, branchHeads, err := s.queryCommits(repo, mainRegex, "", 10)
		require.NoError(t, err)
		require.Len(t, resCommits, 3)
		require.Equal(t, c2, resCommits[0].Hash)
		require.Equal(t, c1, resCommits[1].Hash)
		require.Equal(t, c0, resCommits[2].Hash)
		require.Equal(t, []*git.Branch{
			{
				Name: "main",
				Head: c2,
			},
		}, branchHeads)
	})

	// Regex matches "feature" branch.
	t.Run("branch regex: feature", func(t *testing.T) {
		featRegex := regexp.MustCompile("feature")
		resCommits, branchHeads, err := s.queryCommits(repo, featRegex, "", 10)
		require.NoError(t, err)
		require.Len(t, resCommits, 4)
		require.Equal(t, f2, resCommits[0].Hash)
		require.Equal(t, f1, resCommits[1].Hash)
		require.Equal(t, c1, resCommits[2].Hash)
		require.Equal(t, c0, resCommits[3].Hash)
		require.Equal(t, []*git.Branch{
			{
				Name: "feature",
				Head: f2,
			},
		}, branchHeads)
	})

	// Regex matches both branches.
	t.Run("branch regex: both", func(t *testing.T) {
		allRegex := regexp.MustCompile("(main|feature)")
		resCommits, branchHeads, err := s.queryCommits(repo, allRegex, "", 10)
		require.NoError(t, err)
		require.Len(t, resCommits, 5)
		// Order should be strictly reverse chronological: f2, f1, c2, c1, c0
		require.Equal(t, f2, resCommits[0].Hash)
		require.Equal(t, f1, resCommits[1].Hash)
		require.Equal(t, c2, resCommits[2].Hash)
		require.Equal(t, c1, resCommits[3].Hash)
		require.Equal(t, c0, resCommits[4].Hash)
		require.ElementsMatch(t, []*git.Branch{
			{
				Name: "main",
				Head: c2,
			},
			{
				Name: "feature",
				Head: f2,
			},
		}, branchHeads)
	})

	// Regex doesn't match either branch.
	t.Run("branch regex: no match", func(t *testing.T) {
		noneRegex := regexp.MustCompile("totallybogus")
		resCommits, branchHeads, err := s.queryCommits(repo, noneRegex, "", 10)
		require.NoError(t, err)
		require.Len(t, resCommits, 0)
		require.ElementsMatch(t, []*git.Branch{}, branchHeads)
	})

	// Query with cursor commit c2.
	// We want all commits older than c2 (which has time -6 mins).
	// Commits: f2 (-2 mins, skip), f1 (-4 mins, skip), c2 (-6 mins, skip/cursor), c1 (-8 mins, keep), c0 (-10 mins, keep).
	t.Run("cursor commit", func(t *testing.T) {
		resCommits, branchHeads, err := s.queryCommits(repo, nil, c2, 10)
		require.NoError(t, err)
		require.Len(t, resCommits, 2)
		require.Equal(t, c1, resCommits[0].Hash)
		require.Equal(t, c0, resCommits[1].Hash)
		require.ElementsMatch(t, []*git.Branch{
			{
				Name: "main",
				Head: c2,
			},
			{
				Name: "feature",
				Head: f2,
			},
		}, branchHeads)
	})
}

func TestBroadcastTasks_Cleanup(t *testing.T) {
	tc := setupTestServer(t)

	// Create mock client
	w := httptest.NewRecorder()
	clientCtx, clientCancel := context.WithCancel(tc.ctx)
	defer clientCancel()

	gzipWriter := gzip.NewWriter(w)
	client := &clientStream{
		db:               tc.server.cfg.TaskDb,
		tCache:           tc.server.cfg.TCache,
		window:           tc.server.cfg.Window,
		w:                gzipWriter,
		flusher:          nil,
		ctx:              clientCtx,
		branchRegex:      nil,
		repoURL:          "https://repo.git",
		wantsNewCommits:  false,
		limit:            10,
		displayedCommits: []*rpc.LongCommit{{Hash: "commit1", IsAncestorOf: []string{"main"}}},
	}

	tc.server.register(client)
	require.Len(t, tc.server.clients, 1)

	// Simulate unregister by cancelling the client's context
	clientCancel()
	// Run broadcastTasks with a dummy task, which will clean up the unregistered client
	tc.server.broadcastTasks([]*types.Task{{Id: "dummy-task"}})
	require.Len(t, tc.server.clients, 0)
}

func TestHandler_Validation(t *testing.T) {
	tc := setupTestServer(t)

	t.Run("unknown repo", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/sse?repo=unknown", nil)
		w := httptest.NewRecorder()
		tc.server.Handler(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid branch regex", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/sse?branch=[(", nil)
		w := httptest.NewRecorder()
		tc.server.Handler(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid limit", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/sse?n=not-a-number", nil)
		w := httptest.NewRecorder()
		tc.server.Handler(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandler_ClientLifecycle(t *testing.T) {
	tc := setupTestServer(t)
	c0 := "commit0"

	tc.mockWindow.On("TestCommitHash", "https://repo.git", mock.Anything).Return(true, nil)
	tc.mockWindow.On("EarliestStart").Return(time.Time{})
	tc.mockCache.On("GetTasksForCommits", "https://repo.git", mock.Anything).Return(map[string]map[string]*types.Task{}, nil)

	clientCtx, clientCancel := context.WithCancel(tc.ctx)
	r := httptest.NewRequest("GET", "/sse", nil).WithContext(clientCtx)
	w := httptest.NewRecorder()

	// Run Handler in a goroutine since it blocks on <-ctx.Done()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tc.server.Handler(w, r)
	}()

	// Wait briefly for Handler to write headers, register client, and block
	time.Sleep(100 * time.Millisecond)

	tc.server.clientsMtx.Lock()
	numClients := len(tc.server.clients)
	tc.server.clientsMtx.Unlock()
	require.Equal(t, 1, numClients)

	// Cancel client context to simulate client disconnecting
	clientCancel()
	wg.Wait()

	tc.server.clientsMtx.Lock()
	numClients = len(tc.server.clients)
	tc.server.clientsMtx.Unlock()
	require.Equal(t, 0, numClients)

	// Check response headers
	require.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	require.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	require.Equal(t, "keep-alive", w.Header().Get("Connection"))
	require.Equal(t, "gzip", w.Header().Get("Content-Encoding"))

	// Ensure we sent initial page of commits
	gzipReader, err := gzip.NewReader(w.Body)
	require.NoError(t, err)
	defer gzipReader.Close()
	var buf bytes.Buffer
	_, err = io.Copy(&buf, gzipReader)
	require.NoError(t, err)
	require.Contains(t, buf.String(), c0)
}

func TestCheckForNewCommits_BroadcastsToSubscribers(t *testing.T) {
	tc := setupTestServer(t)

	c0 := "commit0"

	tc.mockWindow.On("TestCommitHash", "https://repo.git", mock.Anything).Return(true, nil).Maybe()
	tc.mockWindow.On("EarliestStart").Return(time.Time{}).Maybe()
	tc.mockCache.On("GetTasksForCommits", "https://repo.git", mock.Anything).Return(map[string]map[string]*types.Task{}, nil).Maybe()

	// Now register a client stream that wants new commits
	w := httptest.NewRecorder()
	gzipWriter := gzip.NewWriter(w)
	client := &clientStream{
		db:               tc.server.cfg.TaskDb,
		tCache:           tc.server.cfg.TCache,
		window:           tc.server.cfg.Window,
		w:                gzipWriter,
		flusher:          nil,
		ctx:              tc.ctx,
		branchRegex:      nil,
		repoURL:          "https://repo.git",
		wantsNewCommits:  true,
		limit:            10,
		displayedCommits: []*rpc.LongCommit{{Hash: c0, IsAncestorOf: []string{"main"}}},
	}
	tc.server.register(client)

	// Let's make a new commit in the in-memory repository
	c1 := "commit1"
	tc.memRepo.Commits[c1] = makeMemCommit(c1, "me@google.com", "file1", time.Now().Add(-5*time.Minute), c0)
	tc.memRepo.BranchList = []*git.Branch{
		{
			Name: "main",
			Head: c1,
		},
	}

	err := tc.repo.Update(tc.ctx)
	require.NoError(t, err)

	// Run checkForNewCommits
	tc.server.checkForNewCommits()

	// Close and flush client stream to verify output
	err = gzipWriter.Close()
	require.NoError(t, err)

	// Read and verify if c1 was broadcasted
	gzipReader, err := gzip.NewReader(w.Body)
	require.NoError(t, err)
	defer gzipReader.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gzipReader)
	require.NoError(t, err)

	output := buf.String()
	// Output should contain data and hash c1.
	require.Contains(t, output, c1)
}

func TestSendUpdate_RemovesOldCommits(t *testing.T) {
	w := httptest.NewRecorder()
	gzipWriter := gzip.NewWriter(w)
	ctx := context.Background()

	mockDB := &db_mocks.RemoteDB{}
	mockCache := &cache_mocks.TaskCache{}
	mockWindow := &window_mocks.Window{}

	mockWindow.On("TestCommitHash", mock.Anything, mock.Anything).Return(true, nil)
	mockCache.On("GetTasksForCommits", mock.Anything, mock.Anything).Return(map[string]map[string]*types.Task{}, nil)

	s := &SSEServer{
		cfg: Config{
			PodId:  "test-pod",
			TCache: mockCache,
			TaskDb: mockDB,
			Window: mockWindow,
		},
	}

	client := &clientStream{
		db:              s.cfg.TaskDb,
		tCache:          s.cfg.TCache,
		window:          s.cfg.Window,
		w:               gzipWriter,
		ctx:             ctx,
		repoURL:         "https://repo.git",
		wantsNewCommits: true,
		limit:           3,
		displayedCommits: []*rpc.LongCommit{
			{Hash: "c3", IsAncestorOf: []string{"main"}},
			{Hash: "c2", IsAncestorOf: []string{"main"}},
			{Hash: "c1", IsAncestorOf: []string{"main"}},
		},
	}

	// Add 2 new commits: "c5", "c4" (sorted newest first)
	err := client.SendUpdate([]*rpc.LongCommit{
		{Hash: "c5", IsAncestorOf: []string{"main"}},
		{Hash: "c4", IsAncestorOf: []string{"main"}},
	}, nil, "https://repo.git", nil)
	require.NoError(t, err)

	// Since limit is 3, c.displayedCommits should be exactly ["c5", "c4", "c3"]
	// and "c2", "c1" should have scrolled off (removed)
	require.Len(t, client.displayedCommits, 3)
	require.Equal(t, "c5", client.displayedCommits[0].Hash)
	require.Equal(t, "c4", client.displayedCommits[1].Hash)
	require.Equal(t, "c3", client.displayedCommits[2].Hash)
}

func TestSendUpdate_CombinesCachedAndUncachedTasks(t *testing.T) {
	tc := setupTestServer(t)

	c0 := "c0"
	c1 := "c1"

	// Let c0 be uncached (out of window), c1 be cached (in window)
	tc.mockWindow.On("TestCommitHash", "https://repo.git", c1).Return(true, nil)
	tc.mockWindow.On("TestCommitHash", "https://repo.git", c0).Return(false, nil)

	// Setup task for c1 (cached) via TaskCache
	cachedTasks := map[string]map[string]*types.Task{
		c1: {
			"cached-task-name": {
				Id:      "task-cached",
				Commits: []string{c1},
				Status:  types.TASK_STATUS_SUCCESS,
				TaskKey: types.TaskKey{
					Name: "cached-task-name",
					RepoState: types.RepoState{
						Repo:     "https://repo.git",
						Revision: c1,
					},
				},
			},
		},
	}
	tc.mockCache.On("GetTasksForCommits", "https://repo.git", []string{c1}).Return(cachedTasks, nil)

	// Setup task for c0 (uncached) via DB SearchTasks
	tc.mockDB.On("SearchTasks", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		params := args.Get(1).(*db.TaskSearchParams)
		require.Equal(t, "https://repo.git", *params.Repo)
		require.Equal(t, c0, *params.BlamelistContains)
	}).Return([]*types.Task{
		{
			Id:      "task-uncached",
			Commits: []string{c0},
			Status:  types.TASK_STATUS_FAILURE,
			TaskKey: types.TaskKey{
				Name: "uncached-task-name",
				RepoState: types.RepoState{
					Repo:     "https://repo.git",
					Revision: c0,
				},
			},
		},
	}, nil)

	rpcCommits := []*rpc.LongCommit{
		{Hash: c1, Timestamp: timestamppb.New(time.Now().Add(-8 * time.Minute)), IsAncestorOf: []string{"main"}},
		{Hash: c0, Timestamp: timestamppb.New(time.Now().Add(-10 * time.Minute)), IsAncestorOf: []string{"main"}},
	}
	branchHeads := []*git.Branch{{Name: "main", Head: c1}}

	w := httptest.NewRecorder()
	gzipWriter := gzip.NewWriter(w)
	client := &clientStream{
		db:              tc.server.cfg.TaskDb,
		tCache:          tc.server.cfg.TCache,
		window:          tc.server.cfg.Window,
		w:               gzipWriter,
		ctx:             tc.ctx,
		repoURL:         "https://repo.git",
		limit:           10,
		wantsNewCommits: true,
	}

	err := client.SendUpdate(rpcCommits, branchHeads, "https://repo.git", nil)
	require.NoError(t, err)

	err = gzipWriter.Close()
	require.NoError(t, err)

	gzipReader, err := gzip.NewReader(w.Body)
	require.NoError(t, err)
	defer gzipReader.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gzipReader)
	require.NoError(t, err)

	output := buf.String()
	require.True(t, strings.HasPrefix(output, "data: "))
	jsonData := strings.TrimSuffix(strings.TrimPrefix(output, "data: "), "\n\n")

	var resp rpc.GetIncrementalCommitsResponse
	err = protojson.Unmarshal([]byte(jsonData), &resp)
	require.NoError(t, err)

	// Verify tasks are correctly retrieved and combined
	require.Len(t, resp.Update.Tasks, 2)
	taskMap := make(map[string]*rpc.Task)
	for _, t := range resp.Update.Tasks {
		taskMap[t.Id] = t
	}

	require.Contains(t, taskMap, "task-cached")
	require.Equal(t, "cached-task-name", taskMap["task-cached"].Name)
	require.Equal(t, string(types.TASK_STATUS_SUCCESS), taskMap["task-cached"].Status)

	require.Contains(t, taskMap, "task-uncached")
	require.Equal(t, "uncached-task-name", taskMap["task-uncached"].Name)
	require.Equal(t, string(types.TASK_STATUS_FAILURE), taskMap["task-uncached"].Status)
}

func TestSendUpdate_OnlyTasksForDisplayedCommits(t *testing.T) {
	ctx := context.Background()

	c0 := "commit0"

	mockDB := &db_mocks.RemoteDB{}
	mockCache := &cache_mocks.TaskCache{}
	mockWindow := &window_mocks.Window{}

	mockWindow.On("TestCommitHash", "https://repo.git", mock.Anything).Return(true, nil)
	mockCache.On("GetTasksForCommits", "https://repo.git", mock.Anything).Return(map[string]map[string]*types.Task{
		c0: {
			"matching-task": {
				Id:     "task-matching",
				Status: types.TASK_STATUS_SUCCESS,
				TaskKey: types.TaskKey{
					Name: "matching-task",
					RepoState: types.RepoState{
						Repo:     "https://repo.git",
						Revision: c0,
					},
				},
			},
		},
	}, nil)

	// Setup client stream
	w := httptest.NewRecorder()
	gzipWriter := gzip.NewWriter(w)
	client := &clientStream{
		db:               mockDB,
		tCache:           mockCache,
		window:           mockWindow,
		w:                gzipWriter,
		ctx:              ctx,
		repoURL:          "https://repo.git",
		limit:            10,
		displayedCommits: []*rpc.LongCommit{{Hash: c0, IsAncestorOf: []string{"main"}}},
	}

	// Call SendUpdate. One matches client, one does not.
	tasks := []*types.Task{
		{
			Id:     "task-matching-commit",
			Status: types.TASK_STATUS_SUCCESS,
			TaskKey: types.TaskKey{
				Name: "matching-task",
				RepoState: types.RepoState{
					Repo:     "https://repo.git",
					Revision: c0,
				},
			},
		},
		{
			Id:     "task-not-matching-commit",
			Status: types.TASK_STATUS_FAILURE,
			TaskKey: types.TaskKey{
				Name: "non-matching-task",
				RepoState: types.RepoState{
					Repo:     "https://repo.git",
					Revision: "some-other-commit",
				},
			},
		},
	}

	err := client.SendUpdate(nil, nil, "https://repo.git", tasks)
	require.NoError(t, err)

	// Close client stream to flush everything
	err = gzipWriter.Close()
	require.NoError(t, err)

	// Read output and verify it is valid JSON and contains the matching task
	gzipReader, err := gzip.NewReader(w.Body)
	require.NoError(t, err)
	defer gzipReader.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gzipReader)
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, "data:")
	require.Contains(t, output, "task-matching-commit")
	require.NotContains(t, output, "task-not-matching-commit")
}
