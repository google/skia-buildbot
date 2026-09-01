package sse

import (
	"bufio"
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
	comments   []*types.RepoComments
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

	tc := &testContext{
		ctx:        ctx,
		repo:       repo,
		mockDB:     mockDB,
		mockCache:  nil, // set below
		mockWindow: nil, // set below
		memRepo:    ri,
		comments:   []*types.RepoComments{},
	}

	mockDB.On("GetCommentsForRepos", mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, repos []string, from time.Time) ([]*types.RepoComments, error) {
		return tc.comments, nil
	})

	mockCache := &cache_mocks.TaskCache{}
	mockWindow := &window_mocks.Window{}
	mockWindow.On("EarliestStart").Return(time.Time{})

	tc.mockCache = mockCache
	tc.mockWindow = mockWindow

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
	tc.server = s

	return tc
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
	mockDB.On("GetCommentsForRepos", mock.Anything, mock.Anything, mock.Anything).Return([]*types.RepoComments{}, nil)

	mockCache := &cache_mocks.TaskCache{}
	mockWindow := &window_mocks.Window{}
	mockWindow.On("EarliestStart").Return(time.Time{})

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
	clientCtx, clientCancel := context.WithCancel(tc.ctx)
	defer clientCancel()

	var buf bytes.Buffer
	bufWriter := bufio.NewWriter(&buf)
	client := &clientStream{
		db:               tc.server.cfg.TaskDb,
		tCache:           tc.server.cfg.TCache,
		window:           tc.server.cfg.Window,
		w:                bufWriter,
		ctx:              clientCtx,
		branchRegex:      nil,
		repoURL:          "https://repo.git",
		wantsNewCommits:  false,
		limit:            10,
		displayedCommits: []*rpc.LongCommit{{Hash: "commit1", IsAncestorOf: []string{"main"}}},
		taskFilter:       taskFilter_All,
		activeTaskSpecs:  map[string]bool{},
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

	t.Run("invalid taskFilter", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/sse?taskFilter=bbad", nil)
		w := httptest.NewRecorder()
		tc.server.Handler(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid taskSearch", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/sse?taskFilter=search&taskSearch=[[[[", nil)
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

	tc.mockWindow.On("TestCommitHash", "https://repo.git", mock.Anything).Return(true, nil)
	tc.mockCache.On("GetTasksForCommits", "https://repo.git", mock.Anything).Return(map[string]map[string]*types.Task{}, nil)

	// Now register a client stream that wants new commits
	var buf bytes.Buffer
	bufWriter := bufio.NewWriter(&buf)
	client := &clientStream{
		db:               tc.server.cfg.TaskDb,
		tCache:           tc.server.cfg.TCache,
		window:           tc.server.cfg.Window,
		w:                bufWriter,
		ctx:              tc.ctx,
		branchRegex:      nil,
		repoURL:          "https://repo.git",
		wantsNewCommits:  true,
		limit:            10,
		displayedCommits: []*rpc.LongCommit{{Hash: c0, IsAncestorOf: []string{"main"}}},
		taskFilter:       taskFilter_All,
		activeTaskSpecs:  map[string]bool{},
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

	// Read and verify if c1 was broadcasted
	resps := readSSEStream(t, &buf)
	require.Len(t, resps, 1)
	require.Len(t, resps[0].Update.Commits, 1)
	require.Equal(t, c1, resps[0].Update.Commits[0].Hash)
}

func TestSendUpdate_RemovesOldCommits(t *testing.T) {
	var buf bytes.Buffer
	bufWriter := bufio.NewWriter(&buf)
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
		w:               bufWriter,
		ctx:             ctx,
		repoURL:         "https://repo.git",
		wantsNewCommits: true,
		limit:           3,
		displayedCommits: []*rpc.LongCommit{
			{Hash: "c3", IsAncestorOf: []string{"main"}},
			{Hash: "c2", IsAncestorOf: []string{"main"}},
			{Hash: "c1", IsAncestorOf: []string{"main"}},
		},
		taskFilter:      taskFilter_All,
		activeTaskSpecs: map[string]bool{},
	}

	// Add 2 new commits: "c5", "c4" (sorted newest first)
	err := client.SendUpdate([]*rpc.LongCommit{
		{Hash: "c5", IsAncestorOf: []string{"main"}},
		{Hash: "c4", IsAncestorOf: []string{"main"}},
	}, nil, "https://repo.git", nil, newRequestCache(), nil)
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

	var buf bytes.Buffer
	bufWriter := bufio.NewWriter(&buf)
	client := &clientStream{
		db:              tc.server.cfg.TaskDb,
		tCache:          tc.server.cfg.TCache,
		window:          tc.server.cfg.Window,
		w:               bufWriter,
		ctx:             tc.ctx,
		repoURL:         "https://repo.git",
		limit:           10,
		wantsNewCommits: true,
		taskFilter:      taskFilter_All,
		activeTaskSpecs: map[string]bool{},
	}

	err := client.SendUpdate(rpcCommits, branchHeads, "https://repo.git", nil, newRequestCache(), nil)
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

func TestHandler_Cursor(t *testing.T) {
	tc := setupTestServer(t)
	c0 := "commit0"
	c1 := "commit1"

	tc.memRepo.Commits[c1] = makeMemCommit(c1, "me@google.com", "c1", time.Now().Add(-5*time.Minute), c0)
	tc.memRepo.BranchList = []*git.Branch{
		{
			Name: "main",
			Head: c1,
		},
	}
	err := tc.repo.Update(tc.ctx)
	require.NoError(t, err)

	tc.mockWindow.On("TestCommitHash", "https://repo.git", mock.Anything).Return(true, nil)
	tc.mockWindow.On("EarliestStart").Return(time.Time{})
	tc.mockCache.On("GetTasksForCommits", "https://repo.git", mock.Anything).Return(map[string]map[string]*types.Task{}, nil)

	// Create a context that is pre-cancelled. This prevents the Handler from blocking on <-ctx.Done()
	clientCtx, clientCancel := context.WithCancel(tc.ctx)
	clientCancel()

	// Connect to Handler with a cursor = c1. The initial commits payload must contain c0 (the older commit).
	r := httptest.NewRequest("GET", fmt.Sprintf("/sse?cursor=%s", c1), nil).WithContext(clientCtx)
	w := httptest.NewRecorder()

	// Call the Handler synchronously
	tc.server.Handler(w, r)

	// Ensure we got the initial commits response and it contains c0 (older than c1)
	gzipReader, err := gzip.NewReader(w.Body)
	require.NoError(t, err)
	defer gzipReader.Close()
	var buf bytes.Buffer
	_, err = io.Copy(&buf, gzipReader)
	require.NoError(t, err)

	resps := readSSEStream(t, &buf)
	require.Len(t, resps, 1, "Expected exactly 1 response representing the initial page of commits")
	require.Len(t, resps[0].Update.Commits, 1, "Expected exactly 1 commit in the update")
	require.Equal(t, c0, resps[0].Update.Commits[0].Hash)
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
	var buf bytes.Buffer
	bufWriter := bufio.NewWriter(&buf)
	client := &clientStream{
		db:               mockDB,
		tCache:           mockCache,
		window:           mockWindow,
		w:                bufWriter,
		ctx:              ctx,
		repoURL:          "https://repo.git",
		limit:            10,
		displayedCommits: []*rpc.LongCommit{{Hash: c0, IsAncestorOf: []string{"main"}}},
		taskFilter:       taskFilter_All,
		activeTaskSpecs:  map[string]bool{},
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

	err := client.SendUpdate(nil, nil, "https://repo.git", tasks, newRequestCache(), nil)
	require.NoError(t, err)

	// Read output and verify it is valid JSON and contains only the matching task
	resps := readSSEStream(t, &buf)
	require.Len(t, resps, 1)

	var taskIds []string
	for _, task := range resps[0].Update.Tasks {
		taskIds = append(taskIds, task.Id)
	}
	require.Contains(t, taskIds, "task-matching-commit")
	require.NotContains(t, taskIds, "task-not-matching-commit")
}

func TestSendUpdate_Filtering(t *testing.T) {
	tc := setupTestServer(t)

	c0 := "commit0"
	c1 := "c1"

	tc.mockWindow.On("TestCommitHash", "https://repo.git", c1).Return(true, nil)
	tc.mockWindow.On("TestCommitHash", "https://repo.git", c0).Return(true, nil)
	tc.mockWindow.On("EarliestStart").Return(time.Time{})

	// Setup tasks for c1 and c0
	tasks := map[string]map[string]*types.Task{
		c1: {
			"task-success-only": {
				Id:      "t1",
				Commits: []string{c1},
				Status:  types.TASK_STATUS_SUCCESS,
				TaskKey: types.TaskKey{Name: "task-success-only", RepoState: types.RepoState{Repo: "https://repo.git", Revision: c1}},
			},
			"task-interesting": {
				Id:      "t2",
				Commits: []string{c1},
				Status:  types.TASK_STATUS_SUCCESS,
				TaskKey: types.TaskKey{Name: "task-interesting", RepoState: types.RepoState{Repo: "https://repo.git", Revision: c1}},
			},
		},
		c0: {
			"task-interesting": {
				Id:      "t3",
				Commits: []string{c0},
				Status:  types.TASK_STATUS_FAILURE,
				TaskKey: types.TaskKey{Name: "task-interesting", RepoState: types.RepoState{Repo: "https://repo.git", Revision: c0}},
			},
		},
	}
	tc.mockCache.On("GetTasksForCommits", "https://repo.git", []string{c1, c0}).Return(tasks, nil)

	cfg := Config{
		Repos:                repograph.Map{"https://repo.git": tc.repo},
		TaskDb:               tc.mockDB,
		TCache:               tc.mockCache,
		Window:               tc.mockWindow,
		RepoUrls:             []string{"https://repo.git"},
		GetRepoTwirp:         func(name string) (string, string, error) { return "", "https://repo.git", nil },
		RepoUrlToName:        func(url string) string { return "foo" },
		DefaultCommitsToLoad: 10,
		MaxCommitsToLoad:     100,
	}
	s, err := New(tc.ctx, cfg)
	require.NoError(t, err)

	rpcCommits := []*rpc.LongCommit{
		{Hash: c1, Timestamp: timestamppb.New(time.Now().Add(-8 * time.Minute)), IsAncestorOf: []string{"main"}},
		{Hash: c0, Timestamp: timestamppb.New(time.Now().Add(-10 * time.Minute)), IsAncestorOf: []string{"main"}},
	}
	branchHeads := []*git.Branch{{Name: "main", Head: c1}}

	getTasksForFilter := func(filter taskFilter, search string) []string {
		var buf bytes.Buffer
		bufWriter := bufio.NewWriter(&buf)
		var taskSearch *regexp.Regexp
		if search != "" {
			var err error
			taskSearch, err = regexp.Compile(search)
			require.NoError(t, err)
		}
		client := &clientStream{
			db:              s.cfg.TaskDb,
			tCache:          s.cfg.TCache,
			window:          s.cfg.Window,
			w:               bufWriter,
			ctx:             tc.ctx,
			repoURL:         "https://repo.git",
			limit:           10,
			wantsNewCommits: true,
			taskFilter:      filter,
			taskSearch:      taskSearch,
			activeTaskSpecs: map[string]bool{},
		}

		err := client.SendUpdate(rpcCommits, branchHeads, "https://repo.git", nil, newRequestCache(), nil)
		require.NoError(t, err)

		output := buf.String()
		if !strings.HasPrefix(output, "data: ") {
			return nil
		}
		jsonData := strings.TrimSuffix(strings.TrimPrefix(output, "data: "), "\n\n")

		var resp rpc.GetIncrementalCommitsResponse
		err = protojson.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		var names []string
		for _, t := range resp.Update.Tasks {
			names = append(names, t.Name)
		}
		return names
	}

	t.Run("Interesting", func(t *testing.T) {
		taskNames := getTasksForFilter(taskFilter_Interesting, "")
		// Only "task-interesting" should be kept (it has success on c1 and failure on c0)
		// "task-success-only" has only success.
		require.Contains(t, taskNames, "task-interesting")
		require.NotContains(t, taskNames, "task-success-only")
	})

	t.Run("Failures", func(t *testing.T) {
		taskNames := getTasksForFilter(taskFilter_Failures, "")
		// Only "task-interesting" (with failure on c0) should be kept.
		require.Contains(t, taskNames, "task-interesting")
		require.NotContains(t, taskNames, "task-success-only")
	})

	t.Run("Search", func(t *testing.T) {
		taskNames := getTasksForFilter(taskFilter_Search, "success")
		// Only "task-success-only" should be kept.
		require.Contains(t, taskNames, "task-success-only")
		require.NotContains(t, taskNames, "task-interesting")
	})
}

func TestSendUpdate_SendsAllTasksForNewlyVisibleTaskSpecs(t *testing.T) {
	ctx := context.Background()
	c0 := "commit0"

	mockDB := &db_mocks.RemoteDB{}
	mockCache := &cache_mocks.TaskCache{}
	mockWindow := &window_mocks.Window{}

	mockWindow.On("TestCommitHash", "https://repo.git", mock.Anything).Return(true, nil)
	mockWindow.On("EarliestStart").Return(time.Time{})

	var buf bytes.Buffer
	bufWriter := bufio.NewWriter(&buf)
	client := &clientStream{
		db:               mockDB,
		tCache:           mockCache,
		window:           mockWindow,
		w:                bufWriter,
		ctx:              ctx,
		branchRegex:      nil,
		repoURL:          "https://repo.git",
		wantsNewCommits:  false,
		limit:            10,
		displayedCommits: []*rpc.LongCommit{{Hash: c0, IsAncestorOf: []string{"main"}}},
		taskFilter:       taskFilter_Interesting, // Wants success and failure
		activeTaskSpecs:  map[string]bool{"task-dynamic": false},
	}

	// Set up mockCache to return a successful task + the failing task to represent "Interesting" status
	mockCache.On("GetTasksForCommits", "https://repo.git", []string{c0}).Return(map[string]map[string]*types.Task{
		c0: {
			"t-success": {
				Id:      "t-success",
				Commits: []string{c0},
				Status:  types.TASK_STATUS_SUCCESS,
				TaskKey: types.TaskKey{Name: "task-dynamic", RepoState: types.RepoState{Repo: "https://repo.git", Revision: c0}},
			},
			"t-failure": {
				Id:      "t-failure",
				Commits: []string{c0},
				Status:  types.TASK_STATUS_FAILURE,
				TaskKey: types.TaskKey{Name: "task-dynamic", RepoState: types.RepoState{Repo: "https://repo.git", Revision: c0}},
			},
		},
	}, nil).Once()

	taskUpdate := &types.Task{
		Id:      "t-failure",
		Commits: []string{c0},
		Status:  types.TASK_STATUS_FAILURE,
		TaskKey: types.TaskKey{Name: "task-dynamic", RepoState: types.RepoState{Repo: "https://repo.git", Revision: c0}},
	}

	err := client.SendUpdate(nil, nil, "https://repo.git", []*types.Task{taskUpdate}, newRequestCache(), nil)
	require.NoError(t, err)

	resps := readSSEStream(t, &buf)
	require.Len(t, resps, 1)

	// Verify that task-dynamic now appears in client's activeTaskSpecs.
	require.True(t, client.activeTaskSpecs["task-dynamic"])

	// Verify both "t-success" and "t-failure" were sent in the response.
	require.Len(t, resps[0].Update.Tasks, 2)
	taskIds := []string{resps[0].Update.Tasks[0].Id, resps[0].Update.Tasks[1].Id}
	require.Contains(t, taskIds, "t-success")
	require.Contains(t, taskIds, "t-failure")
}

func TestSendUpdate_HideTaskSpecs(t *testing.T) {
	ctx := context.Background()
	c0 := "commit0"

	mockDB := &db_mocks.RemoteDB{}
	mockCache := &cache_mocks.TaskCache{}
	mockWindow := &window_mocks.Window{}

	mockWindow.On("TestCommitHash", "https://repo.git", mock.Anything).Return(true, nil)
	mockWindow.On("EarliestStart").Return(time.Time{})

	var buf bytes.Buffer
	bufWriter := bufio.NewWriter(&buf)
	client := &clientStream{
		db:               mockDB,
		tCache:           mockCache,
		window:           mockWindow,
		w:                bufWriter,
		ctx:              ctx,
		branchRegex:      nil,
		repoURL:          "https://repo.git",
		wantsNewCommits:  false,
		limit:            10,
		displayedCommits: []*rpc.LongCommit{{Hash: c0, IsAncestorOf: []string{"main"}}},
		taskFilter:       taskFilter_Interesting,
		activeTaskSpecs:  map[string]bool{"task-dynamic": true},
	}

	// Set up mockCache to return only a successful task (meaning it is no
	// longer Interesting since no failure exists).
	mockCache.On("GetTasksForCommits", "https://repo.git", []string{c0}).Return(map[string]map[string]*types.Task{
		c0: {
			"task-dynamic": {
				Id:      "t-success",
				Commits: []string{c0},
				Status:  types.TASK_STATUS_SUCCESS,
				TaskKey: types.TaskKey{Name: "task-dynamic", RepoState: types.RepoState{Repo: "https://repo.git", Revision: c0}},
			},
		},
	}, nil).Once()

	taskUpdate := &types.Task{
		Id:      "t-success",
		Commits: []string{c0},
		Status:  types.TASK_STATUS_SUCCESS,
		TaskKey: types.TaskKey{Name: "task-dynamic", RepoState: types.RepoState{Repo: "https://repo.git", Revision: c0}},
	}

	err := client.SendUpdate(nil, nil, "https://repo.git", []*types.Task{taskUpdate}, newRequestCache(), nil)
	require.NoError(t, err)

	resps := readSSEStream(t, &buf)
	require.Len(t, resps, 1)

	// Verify task-dynamic is no longer in client's activeTaskSpecs.
	require.False(t, client.activeTaskSpecs["task-dynamic"])

	// Verify hideTaskSpecs was sent containing "task-dynamic"
	require.Contains(t, resps[0].Update.HideTaskSpecs, "task-dynamic")
}

func TestRequestCache(t *testing.T) {
	rc := newRequestCache()

	commits1 := []*rpc.LongCommit{{Hash: "c1"}}
	commits2 := []*rpc.LongCommit{{Hash: "c2"}}

	calls1 := 0
	tasks1, _, err := rc.GetOrCache("repo", commits1, taskFilter_All, nil, func() ([]*rpc.Task, []string, error) {
		calls1++
		return []*rpc.Task{{Id: "t1"}}, nil, nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, calls1)
	require.Len(t, tasks1, 1)
	require.Equal(t, "t1", tasks1[0].Id)

	calls2 := 0
	tasks2, _, err := rc.GetOrCache("repo", commits2, taskFilter_All, nil, func() ([]*rpc.Task, []string, error) {
		calls2++
		return []*rpc.Task{{Id: "t2"}}, nil, nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, calls2, "Expected cache miss due to different displayed commits")
	require.Len(t, tasks2, 1)
	require.Equal(t, "t2", tasks2[0].Id)
}

func TestRequestCache_Concurrent(t *testing.T) {
	rc := newRequestCache()
	commits := []*rpc.LongCommit{{Hash: "c1"}}

	var wg sync.WaitGroup
	calls := 0
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := rc.GetOrCache("repo", commits, taskFilter_All, nil, func() ([]*rpc.Task, []string, error) {
				time.Sleep(10 * time.Millisecond) // Simulate DB query latency
				calls++
				return []*rpc.Task{{Id: "t1"}}, nil, nil
			})
			require.NoError(t, err)
		}()
	}

	wg.Wait()
	require.Equal(t, 1, calls, "Expected generator to be called exactly once")
}

func TestCheckForNewComments_BroadcastsToSubscribers(t *testing.T) {
	tc := setupTestServer(t)

	c0 := "commit0"

	tc.mockWindow.On("TestCommitHash", "https://repo.git", mock.Anything).Return(true, nil)
	tc.mockCache.On("GetTasksForCommits", "https://repo.git", mock.Anything).Return(map[string]map[string]*types.Task{}, nil)

	// Now register a client stream that wants updates
	var buf bytes.Buffer
	bufWriter := bufio.NewWriter(&buf)
	client := &clientStream{
		db:               tc.server.cfg.TaskDb,
		tCache:           tc.server.cfg.TCache,
		window:           tc.server.cfg.Window,
		w:                bufWriter,
		ctx:              tc.ctx,
		branchRegex:      nil,
		repoURL:          "https://repo.git",
		wantsNewCommits:  true,
		limit:            10,
		displayedCommits: []*rpc.LongCommit{{Hash: c0, IsAncestorOf: []string{"main"}}},
		taskFilter:       taskFilter_All,
		activeTaskSpecs:  map[string]bool{},
	}
	tc.server.register(client)

	// Set the comments to be returned by the mocked GetCommentsForRepos
	tc.comments = []*types.RepoComments{
		{
			Repo: "https://repo.git",
			CommitComments: map[string][]*types.CommitComment{
				c0: {
					{
						Repo:          "https://repo.git",
						Revision:      c0,
						Timestamp:     time.Now().UTC(),
						User:          "test-user@google.com",
						IgnoreFailure: true,
						Message:       "This is a test comment",
					},
				},
			},
		},
	}

	// Run checkForNewComments
	tc.server.checkForNewComments(tc.ctx)

	// Read and verify if the comment was broadcasted
	resps := readSSEStream(t, &buf)
	require.Len(t, resps, 1)
	require.Len(t, resps[0].Update.Comments, 1)
	require.Equal(t, "This is a test comment", resps[0].Update.Comments[0].Message)
}

func TestCheckForNewComments_CreateUpdateDelete(t *testing.T) {
	tc := setupTestServer(t)

	c0 := "commit0"

	tc.mockWindow.On("TestCommitHash", "https://repo.git", mock.Anything).Return(true, nil)
	tc.mockCache.On("GetTasksForCommits", "https://repo.git", mock.Anything).Return(map[string]map[string]*types.Task{}, nil)

	// Register a client stream.
	var buf bytes.Buffer
	bufWriter := bufio.NewWriter(&buf)
	client := &clientStream{
		db:               tc.server.cfg.TaskDb,
		tCache:           tc.server.cfg.TCache,
		window:           tc.server.cfg.Window,
		w:                bufWriter,
		ctx:              tc.ctx,
		branchRegex:      nil,
		repoURL:          "https://repo.git",
		wantsNewCommits:  true,
		limit:            10,
		displayedCommits: []*rpc.LongCommit{{Hash: c0, IsAncestorOf: []string{"main"}}},
		taskFilter:       taskFilter_All,
		activeTaskSpecs:  map[string]bool{},
	}
	tc.server.register(client)

	// Helper to extract all broadcasted comments so far.
	getComments := func() []*rpc.Comment {
		resps := readSSEStream(t, &buf)
		var comments []*rpc.Comment
		for _, r := range resps {
			if r.Update != nil {
				comments = append(comments, r.Update.Comments...)
			}
		}
		return comments
	}

	// Create a comment.
	comment1 := &types.CommitComment{
		Repo:          "https://repo.git",
		Revision:      c0,
		Timestamp:     time.Now().UTC().Truncate(time.Second),
		User:          "test-user@google.com",
		IgnoreFailure: false,
		Message:       "First comment",
	}
	tc.comments = []*types.RepoComments{
		{
			Repo: "https://repo.git",
			CommitComments: map[string][]*types.CommitComment{
				c0: {comment1},
			},
		},
	}

	tc.server.checkForNewComments(tc.ctx)

	commentsPh1 := getComments()
	require.Len(t, commentsPh1, 1)
	require.Equal(t, comment1.Id(), commentsPh1[0].Id)
	require.Equal(t, "First comment", commentsPh1[0].Message)
	require.False(t, commentsPh1[0].IgnoreFailure)
	require.False(t, commentsPh1[0].Deleted)

	// Update the comment.
	comment1Modified := &types.CommitComment{
		Repo:          comment1.Repo,
		Revision:      comment1.Revision,
		Timestamp:     comment1.Timestamp,
		User:          comment1.User,
		IgnoreFailure: true,
		Message:       "First comment - modified",
	}
	tc.comments = []*types.RepoComments{
		{
			Repo: "https://repo.git",
			CommitComments: map[string][]*types.CommitComment{
				c0: {comment1Modified},
			},
		},
	}

	tc.server.checkForNewComments(tc.ctx)

	commentsPh2 := getComments()
	require.Len(t, commentsPh2, 2)
	require.Equal(t, comment1.Id(), commentsPh2[1].Id)
	require.Equal(t, "First comment - modified", commentsPh2[1].Message)
	require.True(t, commentsPh2[1].IgnoreFailure)
	require.False(t, commentsPh2[1].Deleted)

	// Delete the comment.
	tc.comments = []*types.RepoComments{
		{
			Repo: "https://repo.git",
		},
	}

	tc.server.checkForNewComments(tc.ctx)

	commentsPh3 := getComments()
	require.Len(t, commentsPh3, 3)
	require.Equal(t, comment1.Id(), commentsPh3[2].Id)
	require.True(t, commentsPh3[2].Deleted)
}

func readSSEStream(t *testing.T, buf *bytes.Buffer) []*rpc.GetIncrementalCommitsResponse {
	output := buf.String()
	lines := strings.Split(output, "\n\n")

	var responses []*rpc.GetIncrementalCommitsResponse
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		jsonData := strings.TrimPrefix(line, "data: ")

		var resp rpc.GetIncrementalCommitsResponse
		err := protojson.Unmarshal([]byte(jsonData), &resp)
		require.NoError(t, err)

		responses = append(responses, &resp)
	}
	return responses
}
