package sse

import (
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/status/go/rpc"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Config struct {
	Repos                repograph.Map
	TaskDb               db.RemoteDB
	TCache               cache.TaskCache
	Window               window.Window
	PodId                string
	DefaultCommitsToLoad int
	MaxCommitsToLoad     int
	RepoUrls             []string
	GetRepoTwirp         func(name string) (string, string, error)
	RepoUrlToName        func(url string) string
}

type SSEServer struct {
	clients          map[*clientStream]bool
	clientsMtx       sync.Mutex
	lastBranchHeads  map[string][]*git.Branch
	modifiedTasksCh  <-chan []*types.Task
	cfg              Config
	ancestryCache    map[string]map[string][]string
	ancestryCacheMtx sync.RWMutex
}

func New(ctx context.Context, cfg Config) (*SSEServer, error) {
	if cfg.Repos == nil {
		return nil, skerr.Fmt("Repos is a required configuration field")
	}
	if cfg.TaskDb == nil {
		return nil, skerr.Fmt("TaskDb is a required configuration field")
	}
	if cfg.TCache == nil {
		return nil, skerr.Fmt("TCache is a required configuration field")
	}
	if cfg.Window == nil {
		return nil, skerr.Fmt("Window is a required configuration field")
	}
	if len(cfg.RepoUrls) == 0 {
		return nil, skerr.Fmt("RepoUrls is a required configuration field")
	}
	if cfg.GetRepoTwirp == nil {
		return nil, skerr.Fmt("GetRepoTwirp is a required configuration field")
	}
	if cfg.RepoUrlToName == nil {
		return nil, skerr.Fmt("RepoUrlToName is a required configuration field")
	}

	s := &SSEServer{
		ancestryCache:   make(map[string]map[string][]string, len(cfg.Repos)),
		clients:         make(map[*clientStream]bool),
		lastBranchHeads: make(map[string][]*git.Branch),
		modifiedTasksCh: cfg.TaskDb.ModifiedTasksCh(ctx),
		cfg:             cfg,
	}
	// Warm up branch heads
	for _, repoURL := range cfg.RepoUrls {
		repo := cfg.Repos[repoURL]
		s.lastBranchHeads[repoURL] = repo.BranchHeads()
		s.ancestryCache[repoURL] = make(map[string][]string, repo.Len())
		updateAncestryCache(s.ancestryCache[repoURL], repo, nil)
	}
	go s.broadcastLoop(ctx)
	return s, nil
}

func (s *SSEServer) Handler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	repoName := r.URL.Query().Get("repo")
	if repoName == "" {
		repoName = s.cfg.RepoUrlToName(s.cfg.RepoUrls[0])
	}
	_, repoURL, err := s.cfg.GetRepoTwirp(repoName)
	if err != nil {
		httputils.ReportError(w, err, "unknown repo", http.StatusBadRequest)
		return
	}

	branchQuery := r.URL.Query().Get("branch")
	var branchRegex *regexp.Regexp
	if branchQuery != "" {
		var err error
		branchRegex, err = regexp.Compile(branchQuery)
		if err != nil {
			httputils.ReportError(w, err, "invalid branch regex", http.StatusBadRequest)
			return
		}
	}

	n := s.cfg.DefaultCommitsToLoad
	if limitStr := r.URL.Query().Get("n"); limitStr != "" {
		var err error
		n, err = strconv.Atoi(limitStr)
		if err != nil {
			httputils.ReportError(w, err, "invalid value for 'n'", http.StatusBadRequest)
			return
		}
	}
	if n > s.cfg.MaxCommitsToLoad {
		n = s.cfg.MaxCommitsToLoad
	}

	cursor := r.URL.Query().Get("cursor")

	// Retrieve the graph for this repo.
	repo, ok := s.cfg.Repos[repoURL]
	if !ok {
		http.Error(w, "repository not found", http.StatusInternalServerError)
		return
	}

	// Set headers for Server-Sent Events (SSE) with Gzip compression
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, flusherOk := w.(http.Flusher)
	if flusherOk {
		flusher.Flush()
	}

	gzipWriter := gzip.NewWriter(w)
	// Flush the headers and gzip metadata immediately so the connection is established
	_ = gzipWriter.Flush()
	if flusherOk {
		flusher.Flush()
	}
	defer func() {
		_ = gzipWriter.Close()
	}()

	// There's a race condition in which the server might produce an update
	// (eg. new tasks or commits) after our initial queries run but before the
	// client is registered, causing the client to never receive the update and
	// thus miss tasks or commits. To circumvent this, we register the client
	// with the lock already held, so that the server is aware of this client
	// but cannot send updates until after the initial load.
	// Set up the client stream.
	clientStream := &clientStream{
		branchRegex:     branchRegex,
		ctx:             ctx,
		db:              s.cfg.TaskDb,
		flusher:         flusher,
		limit:           n,
		repoURL:         repoURL,
		tCache:          s.cfg.TCache,
		w:               gzipWriter,
		wantsNewCommits: cursor == "",
		window:          s.cfg.Window,
	}
	clientStream.mtx.Lock()
	s.register(clientStream)
	defer s.unregister(clientStream)

	// Get the initial full page of commits.
	resultCommits, branchHeads, err := s.queryCommits(repo, branchRegex, cursor, n)
	if err != nil {
		clientStream.mtx.Unlock()
		httputils.ReportError(w, err, "failed to retrieve commits", http.StatusInternalServerError)
		return
	}
	s.ancestryCacheMtx.RLock()
	rpcCommits := convertCommits(resultCommits, s.ancestryCache[repoURL])
	s.ancestryCacheMtx.RUnlock()

	// Build the initial response.
	if err := clientStream.sendUpdateLocked(rpcCommits, branchHeads, repoURL, nil); err != nil {
		clientStream.mtx.Unlock()
		httputils.ReportError(w, err, "failed to build send response", http.StatusInternalServerError)
		return
	}
	clientStream.mtx.Unlock()

	// Keep socket open
	<-ctx.Done()
}

func (s *SSEServer) register(client *clientStream) {
	s.clientsMtx.Lock()
	defer s.clientsMtx.Unlock()
	s.clients[client] = true
}

func (s *SSEServer) unregister(client *clientStream) {
	s.clientsMtx.Lock()
	defer s.clientsMtx.Unlock()
	delete(s.clients, client)
}

func (s *SSEServer) broadcastLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case tasks := <-s.modifiedTasksCh:
			s.broadcastTasks(tasks)
		case <-ticker.C:
			s.checkForNewCommits()
		}
	}
}

func (s *SSEServer) broadcastTasks(tasks []*types.Task) {
	tasksByRepo := map[string][]*types.Task{}
	for _, t := range tasks {
		if !t.IsTryJob() {
			tasksByRepo[t.Repo] = append(tasksByRepo[t.Repo], t)
		}
	}
	for repoURL, repoTasks := range tasksByRepo {
		s.broadcastUpdate(nil, nil, repoURL, repoTasks)
	}
}

func (s *SSEServer) checkForNewCommits() {
	s.ancestryCacheMtx.Lock()
	defer s.ancestryCacheMtx.Unlock()
	for repoURL, repo := range s.cfg.Repos {
		var newCommits []*repograph.Commit

		_ = repo.RecurseAllBranches(func(c *repograph.Commit) error {
			// Presence in ancestryCache indicates that we've seen this commit before.
			if _, ok := s.ancestryCache[repoURL][c.Hash]; ok {
				return repograph.ErrStopRecursing
			}
			newCommits = append(newCommits, c)
			return nil
		})

		if len(newCommits) > 0 {
			updateAncestryCache(s.ancestryCache[repoURL], repo, s.lastBranchHeads[repoURL])
			rpcCommits := convertCommits(newCommits, s.ancestryCache[repoURL])

			newBranchHeads := repo.BranchHeads()
			s.lastBranchHeads[repoURL] = newBranchHeads
			s.broadcastUpdate(rpcCommits, newBranchHeads, repoURL, nil)
		}
	}
}

func (s *SSEServer) broadcastUpdate(commits []*rpc.LongCommit, branchHeads []*git.Branch, repoURL string, tasks []*types.Task) {
	s.clientsMtx.Lock()
	var eg errgroup.Group
	for client := range s.clients {
		eg.Go(func() error {
			select {
			case <-client.Done():
				s.unregister(client)
				return nil
			default:
			}
			if err := client.SendUpdate(commits, branchHeads, repoURL, tasks); err != nil {
				s.unregister(client)
				return skerr.Wrapf(err, "failed to send response; unregistered the client")
			}
			return nil
		})
	}
	s.clientsMtx.Unlock()
	if err := eg.Wait(); err != nil {
		sklog.Error(err)
	}
}

func (s *SSEServer) queryCommits(repo *repograph.Graph, branchRegex *regexp.Regexp, cursor string, n int) ([]*repograph.Commit, []*git.Branch, error) {
	var candidates []*repograph.Commit
	var branchHeads []*git.Branch
	for _, bh := range repo.BranchHeads() {
		if branchRegex == nil || branchRegex.MatchString(bh.Name) {
			c := repo.Get(bh.Head)
			if c == nil {
				return nil, nil, skerr.Fmt("unknown commit %s", bh.Head)
			}
			candidates = append(candidates, c)
			branchHeads = append(branchHeads, bh)
		}
	}

	if len(candidates) == 0 {
		return nil, nil, nil
	}

	var cursorCommit *repograph.Commit
	if cursor != "" {
		cursorCommit = repo.Get(cursor)
		if cursorCommit == nil {
			return nil, nil, skerr.Fmt("cursor commit %s not found", cursor)
		}
	}

	// Perform a breadth-first search of the commit graph.
	visited := make(map[string]bool)
	s.sortCandidates(candidates)
	var result []*repograph.Commit
	for len(result) < n && len(candidates) > 0 {
		curr := candidates[0]
		candidates = candidates[1:]
		if visited[curr.Hash] {
			continue
		}
		visited[curr.Hash] = true

		isOlder := true
		if cursorCommit != nil {
			if curr.Hash == cursorCommit.Hash || s.isCommitNewer(curr, cursorCommit) {
				isOlder = false
			}
		}
		if isOlder {
			result = append(result, curr)
		}

		for _, p := range curr.GetParents() {
			if !visited[p.Hash] {
				candidates = append(candidates, p)
			}
		}

		s.sortCandidates(candidates)
	}

	return result, branchHeads, nil
}

func (s *SSEServer) isCommitNewer(a, b *repograph.Commit) bool {
	if a.Timestamp.Equal(b.Timestamp) {
		return a.Hash > b.Hash
	}
	return a.Timestamp.After(b.Timestamp)
}

func (s *SSEServer) sortCandidates(candidates []*repograph.Commit) {
	sort.Slice(candidates, func(i, j int) bool {
		return s.isCommitNewer(candidates[i], candidates[j])
	})
}

type clientStream struct {
	branchRegex      *regexp.Regexp
	ctx              context.Context
	db               db.TaskReader
	displayedCommits []*rpc.LongCommit
	flusher          http.Flusher
	limit            int
	mtx              sync.Mutex
	repoURL          string
	tCache           cache.TaskCache
	w                *gzip.Writer
	wantsNewCommits  bool
	window           window.Window
}

func (c *clientStream) Done() <-chan struct{} {
	return c.ctx.Done()
}

// SendUpdate builds and sends an update to the clientStream.
//
// repoURL must always be set. Either newTasks or newCommits and branchHeads,
// or all of the above must be set.
func (s *clientStream) SendUpdate(newCommits []*rpc.LongCommit, branchHeads []*git.Branch, repoURL string, newTasks []*types.Task) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.sendUpdateLocked(newCommits, branchHeads, repoURL, newTasks)
}

// sendUpdateLocked is a helper function for SendUpdate which assumes that the
// caller holds clientStream.mtx.
func (s *clientStream) sendUpdateLocked(newCommits []*rpc.LongCommit, branchHeads []*git.Branch, repoURL string, newTasks []*types.Task) error {
	if repoURL != s.repoURL {
		return nil
	}

	resp := &rpc.GetIncrementalCommitsResponse{
		Metadata: &rpc.ResponseMetadata{
			Timestamp: timestamppb.New(time.Now()),
		},
		Update: &rpc.IncrementalUpdate{},
	}

	// Commits and branch heads.
	if len(newCommits) > 0 && s.wantsNewCommits {
		// Filter commits.
		var filteredCommits []*rpc.LongCommit
		for _, c := range newCommits {
			for _, branch := range c.IsAncestorOf {
				if s.matchBranch(branch) {
					filteredCommits = append(filteredCommits, c)
					break
				}
			}
		}
		resp.Update.Commits = filteredCommits
		// Prepend new commits to keep them sorted newest first
		updatedCommits := append(filteredCommits, s.displayedCommits...)
		if len(updatedCommits) > s.limit {
			updatedCommits = updatedCommits[:s.limit]
		}
		s.displayedCommits = updatedCommits

		// Filter branch heads.
		for _, bh := range branchHeads {
			if s.matchBranch(bh.Name) {
				resp.Update.BranchHeads = append(resp.Update.BranchHeads, &rpc.Branch{
					Name: bh.Name,
					Head: bh.Head,
				})
			}
		}

		// Since we have new commits, retrieve their tasks.
		if len(filteredCommits) > 0 {
			tasks, err := getTasksForCommits(s.ctx, s.window, s.tCache, s.db, s.repoURL, filteredCommits)
			if err != nil {
				return skerr.Wrap(err)
			}
			for _, t := range tasks {
				resp.Update.Tasks = append(resp.Update.Tasks, &rpc.Task{
					Commits:        t.Commits,
					Name:           t.Name,
					Id:             t.Id,
					Revision:       t.Revision,
					Status:         string(t.Status),
					SwarmingTaskId: t.SwarmingTaskId,
					TaskExecutor:   t.TaskExecutor,
				})
			}
		}
	}

	// Filter modified tasks by displayed commits.
	displayedCommits := make(map[string]bool, len(s.displayedCommits))
	for _, c := range s.displayedCommits {
		displayedCommits[c.Hash] = true
	}
	var updatedTasks []*rpc.Task
	for _, t := range newTasks {
		if t.Repo == s.repoURL && displayedCommits[t.Revision] {
			updatedTasks = append(updatedTasks, &rpc.Task{
				Commits:        t.Commits,
				Name:           t.Name,
				Id:             t.Id,
				Revision:       t.Revision,
				Status:         string(t.Status),
				SwarmingTaskId: t.SwarmingTaskId,
				TaskExecutor:   t.TaskExecutor,
			})
		}
	}
	if len(updatedTasks) > 0 {
		resp.Update.Tasks = append(resp.Update.Tasks, updatedTasks...)
	}

	// Send the update, if non-empty.
	if !updateIsEmpty(resp) {
		jsonData, err := protojson.Marshal(resp)
		if err != nil {
			return skerr.Wrapf(err, "failed to encode SSE event")
		}
		_, err = fmt.Fprintf(s.w, "data: %s\n\n", jsonData)
		if err != nil {
			return skerr.Wrapf(err, "failed to send SSE event")
		}
		if err := s.w.Flush(); err != nil {
			return skerr.Wrapf(err, "failed to flush SSE stream")
		}
		if s.flusher != nil {
			s.flusher.Flush()
		}
	}
	return nil
}

func (c *clientStream) matchBranch(branchName string) bool {
	return c.branchRegex == nil || c.branchRegex.MatchString(branchName)
}

// updateAncestryCache updates the given sub-map of the ancestryCache. The
// caller MUST hold a lock on ancestryCache.
func updateAncestryCache(cache map[string][]string, repo *repograph.Graph, oldBranchHeads []*git.Branch) {
	// Ensure that the cache has each branch listed for every reachable commit.
	for _, b := range repo.BranchHeads() {
		_ = repo.Get(b.Head).Recurse(func(c *repograph.Commit) error {
			if util.In(b.Name, cache[c.Hash]) {
				return repograph.ErrStopRecursing
			}
			cache[c.Hash] = append(cache[c.Hash], b.Name)
			return nil
		})
	}

	// Handle any branches which were removed.
	newBranches := make(map[string]bool, len(repo.BranchHeads()))
	for _, bh := range repo.BranchHeads() {
		newBranches[bh.Name] = true
	}
	var removedBranches []*git.Branch
	for _, bh := range oldBranchHeads {
		if !newBranches[bh.Name] {
			removedBranches = append(removedBranches, bh)
		}
	}
	for _, bh := range removedBranches {
		_ = repo.RecurseAllBranches(func(c *repograph.Commit) error {
			// Shortcut: if this commit was never part of this branch, its
			// ancestors weren't either, so we can stop recursing.
			foundBranch := false
			cache[c.Hash] = slices.DeleteFunc(cache[c.Hash], func(branch string) bool {
				if branch == bh.Name {
					foundBranch = true
					return true
				}
				return false
			})
			if !foundBranch {
				return repograph.ErrStopRecursing
			}
			return nil
		})
	}
}

// convertCommits converts the repograph.Commits to rpc.LongCommits. The caller
// MUST hold a read lock on ancestryCache.
func convertCommits(commits []*repograph.Commit, ancestryCache map[string][]string) []*rpc.LongCommit {
	rv := make([]*rpc.LongCommit, 0, len(commits))
	for _, c := range commits {
		isAncestorOf := ancestryCache[c.Hash]
		rv = append(rv, &rpc.LongCommit{
			Hash:         c.Hash,
			Author:       c.Author,
			Subject:      c.Subject,
			Parents:      c.Parents,
			Body:         c.Body,
			Timestamp:    timestamppb.New(c.Timestamp),
			IsAncestorOf: isAncestorOf,
		})
	}
	return rv
}

func getTasksForCommits(ctx context.Context, w window.Window, tCache cache.TaskCache, tsDB db.TaskReader, repoURL string, commits []*rpc.LongCommit) ([]*types.Task, error) {
	// Separate commits into cached (in-window) and uncached (out-of-window).
	var cachedCommits []string
	var uncachedCommits []*rpc.LongCommit

	for _, c := range commits {
		cached, err := w.TestCommitHash(repoURL, c.Hash)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if cached {
			cachedCommits = append(cachedCommits, c.Hash)
		} else {
			uncachedCommits = append(uncachedCommits, c)
		}
	}

	allTasks := map[string]*types.Task{}
	var mtx sync.Mutex

	// Fetch cached tasks.
	if len(cachedCommits) > 0 {
		cachedTasks, err := tCache.GetTasksForCommits(repoURL, cachedCommits)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to load tasks from cache")
		}
		for _, commitTasks := range cachedTasks {
			for _, t := range commitTasks {
				allTasks[t.Id] = t
			}
		}
	}

	// Fetch uncached tasks.
	if len(uncachedCommits) > 0 {
		var g errgroup.Group

		resultsLimit := 0 // No limit.
		for _, c := range uncachedCommits {
			c := c // https://golang.org/doc/faq#closures_and_goroutines
			g.Go(func() error {
				timestamp := c.Timestamp.AsTime()
				params := &db.TaskSearchParams{
					Repo:              &repoURL,
					BlamelistContains: &c.Hash,
					TimeStart:         &timestamp,
					Limit:             &resultsLimit,
				}
				tasks, err := tsDB.SearchTasks(ctx, params)
				if err != nil {
					return err
				}

				mtx.Lock()
				defer mtx.Unlock()
				for _, t := range tasks {
					allTasks[t.Id] = t
				}
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return nil, skerr.Wrapf(err, "failed to search tasks from DB")
		}
	}

	// Return the results.
	rv := make([]*types.Task, 0, len(allTasks))
	for _, t := range allTasks {
		rv = append(rv, t)
	}
	return rv, nil
}

func updateIsEmpty(resp *rpc.GetIncrementalCommitsResponse) bool {
	if len(resp.Update.BranchHeads) > 0 {
		return false
	}
	if len(resp.Update.Comments) > 0 {
		return false
	}
	if len(resp.Update.Commits) > 0 {
		return false
	}
	if len(resp.Update.Tasks) > 0 {
		return false
	}
	return true
}
