package sse

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	commitPollInterval  = 10 * time.Second
	commentPollInterval = 10 * time.Second
	keepaliveInterval   = 30 * time.Second
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
	lastComments     map[string][]*rpc.Comment
	lastCommentsMtx  sync.RWMutex
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
		lastComments:    make(map[string][]*rpc.Comment),
		cfg:             cfg,
	}
	// Initial branch heads and ancestry cache.
	for _, repoURL := range cfg.RepoUrls {
		repo := cfg.Repos[repoURL]
		s.lastBranchHeads[repoURL] = repo.BranchHeads()
		s.ancestryCache[repoURL] = make(map[string][]string, repo.Len())
		updateAncestryCache(s.ancestryCache[repoURL], repo, nil)
	}

	// Initial comments.
	repoComments, err := cfg.TaskDb.GetCommentsForRepos(ctx, cfg.RepoUrls, cfg.Window.EarliestStart())
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to load initial comments")
	}
	for _, rc := range repoComments {
		s.lastComments[rc.Repo] = convertComments(rc)
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

	taskFilter := taskFilter(r.URL.Query().Get("taskFilter"))
	if taskFilter == "" {
		taskFilter = taskFilter_Default
	}
	if !taskFilter.Valid() {
		httputils.ReportError(w, skerr.Fmt("invalid value for taskFilter: %s", taskFilter), "invalid value for taskFilter", http.StatusBadRequest)
		return
	}

	var taskSearch *regexp.Regexp
	if taskFilter == taskFilter_Search {
		taskSearchStr := r.URL.Query().Get("taskSearch")
		if taskSearchStr != "" {
			taskSearch, err = regexp.Compile(taskSearchStr)
			if err != nil {
				httputils.ReportError(w, skerr.Wrapf(err, "failed to parse taskSearch"), "invalid regex for taskSearch", http.StatusBadRequest)
				return
			}
		}
	}

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
		activeTaskSpecs: map[string]bool{},
		branchRegex:     branchRegex,
		ctx:             ctx,
		db:              s.cfg.TaskDb,
		limit:           n,
		repoURL:         repoURL,
		taskFilter:      taskFilter,
		taskSearch:      taskSearch,
		tCache:          s.cfg.TCache,
		w: &doubleFlushingWriter{
			flushWriter: gzipWriter,
			flusher:     flusher,
		},
		wantsNewCommits: true,
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

	// Get initial comments.
	s.lastCommentsMtx.RLock()
	initialComments := s.lastComments[repoURL]
	s.lastCommentsMtx.RUnlock()

	// Build the initial response.
	rq := newRequestCache()
	if err := clientStream.sendUpdateLocked(rpcCommits, branchHeads, repoURL, nil, rq, initialComments); err != nil {
		clientStream.mtx.Unlock()
		httputils.ReportError(w, err, "failed to build send response", http.StatusInternalServerError)
		return
	}
	if cursor != "" {
		clientStream.wantsNewCommits = false
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
	commitsTicker := time.NewTicker(commitPollInterval)
	defer commitsTicker.Stop()
	commentsTicker := time.NewTicker(commentPollInterval)
	defer commentsTicker.Stop()
	keepaliveTicker := time.NewTicker(keepaliveInterval)
	defer keepaliveTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case tasks := <-s.modifiedTasksCh:
			s.broadcastTasks(tasks)
		case <-commitsTicker.C:
			s.checkForNewCommits()
		case <-commentsTicker.C:
			s.checkForNewComments(ctx)
		case <-keepaliveTicker.C:
			s.broadcastKeepalives()
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
		s.broadcastUpdate(nil, nil, repoURL, repoTasks, nil)
	}
}

func (s *SSEServer) broadcastKeepalives() {
	s.clientsMtx.Lock()
	var eg errgroup.Group
	for client := range s.clients {
		client := client // https://golang.org/doc/faq#closures_and_goroutines
		eg.Go(func() error {
			select {
			case <-client.Done():
				s.unregister(client)
				return nil
			default:
			}
			if err := client.SendKeepalive(); err != nil {
				s.unregister(client)
				return skerr.Wrapf(err, "failed to send keepalive; unregistered the client")
			}
			return nil
		})
	}
	s.clientsMtx.Unlock()
	if err := eg.Wait(); err != nil {
		sklog.Error(err)
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
			s.broadcastUpdate(rpcCommits, newBranchHeads, repoURL, nil, nil)
		}
	}
}

func (s *SSEServer) checkForNewComments(ctx context.Context) {
	s.lastCommentsMtx.Lock()
	defer s.lastCommentsMtx.Unlock()

	repoComments, err := s.cfg.TaskDb.GetCommentsForRepos(ctx, s.cfg.RepoUrls, s.cfg.Window.EarliestStart())
	if err != nil {
		sklog.Errorf("failed to retrieve comments: %v", err)
		return
	}

	for _, rc := range repoComments {
		prevComments := s.lastComments[rc.Repo]
		newComments := convertComments(rc)

		var updatedComments []*rpc.Comment

		prevCommentsMap := make(map[string]*rpc.Comment, len(prevComments))
		for _, c := range prevComments {
			prevCommentsMap[c.Id] = c
		}

		newCommentsMap := make(map[string]*rpc.Comment, len(newComments))
		for _, c := range newComments {
			newCommentsMap[c.Id] = c
		}

		// Find new or updated comments
		for _, c := range newComments {
			if prev, ok := prevCommentsMap[c.Id]; !ok || !proto.Equal(prev, c) {
				updatedComments = append(updatedComments, c)
			}
		}

		// Find deleted comments
		for _, prev := range prevComments {
			if _, ok := newCommentsMap[prev.Id]; !ok {
				deletedComment := proto.Clone(prev).(*rpc.Comment)
				deletedComment.Deleted = true
				updatedComments = append(updatedComments, deletedComment)
			}
		}

		s.lastComments[rc.Repo] = newComments

		if len(updatedComments) > 0 {
			s.broadcastUpdate(nil, nil, rc.Repo, nil, updatedComments)
		}
	}
}

func (s *SSEServer) broadcastUpdate(commits []*rpc.LongCommit, branchHeads []*git.Branch, repoURL string, tasks []*types.Task, comments []*rpc.Comment) {
	s.clientsMtx.Lock()
	var eg errgroup.Group
	rq := newRequestCache()
	for client := range s.clients {
		eg.Go(func() error {
			select {
			case <-client.Done():
				s.unregister(client)
				return nil
			default:
			}
			if err := client.SendUpdate(commits, branchHeads, repoURL, tasks, rq, comments); err != nil {
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

type flushWriter interface {
	io.Writer
	Flush() error
}

type doubleFlushingWriter struct {
	flushWriter
	flusher http.Flusher
}

func (d *doubleFlushingWriter) Flush() error {
	if err := d.flushWriter.Flush(); err != nil {
		return err
	}
	if d.flusher != nil {
		d.flusher.Flush()
	}
	return nil
}

type clientStream struct {
	activeTaskSpecs  map[string]bool
	branchRegex      *regexp.Regexp
	ctx              context.Context
	db               db.TaskReader
	displayedCommits []*rpc.LongCommit
	limit            int
	mtx              sync.Mutex
	repoURL          string
	taskFilter       taskFilter
	taskSearch       *regexp.Regexp
	tCache           cache.TaskCache
	w                flushWriter
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
func (s *clientStream) SendUpdate(newCommits []*rpc.LongCommit, branchHeads []*git.Branch, repoURL string, newTasks []*types.Task, rq *requestCache, newComments []*rpc.Comment) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.sendUpdateLocked(newCommits, branchHeads, repoURL, newTasks, rq, newComments)
}

// sendUpdateLocked is a helper function for SendUpdate which assumes that the
// caller holds clientStream.mtx.
func (s *clientStream) sendUpdateLocked(newCommits []*rpc.LongCommit, branchHeads []*git.Branch, repoURL string, newTasks []*types.Task, rq *requestCache, newComments []*rpc.Comment) error {
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
	}

	// Determine which task specs the client needs to be displaying.
	// Unfortunately, to correctly handle filtering, we must consider ALL tasks
	// in the commit range, not just the new ones. Fortunately, those tasks
	// should be cached in the vast majority of cases.
	var err error
	resp.Update.Tasks, resp.Update.HideTaskSpecs, err = rq.GetOrCache(repoURL, s.displayedCommits, s.taskFilter, s.taskSearch, func() ([]*rpc.Task, []string, error) {
		allTasks, err := getTasksForCommits(s.ctx, s.window, s.tCache, s.db, s.repoURL, s.displayedCommits)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		tasksByName := map[string][]*types.Task{}
		for _, t := range allTasks {
			tasksByName[t.Name] = append(tasksByName[t.Name], t)
		}
		wantTaskSpecs := map[string]bool{}
		for spec, tasks := range tasksByName {
			if specMatchesFilter(tasks, s.taskFilter, s.taskSearch) {
				wantTaskSpecs[spec] = true
			}
		}

		// Determine which tasks to send.
		tasksToSend := make(map[string]*types.Task, len(newTasks))
		for _, t := range newTasks {
			if s.repoURL == t.Repo && wantTaskSpecs[t.Name] {
				tasksToSend[t.Id] = t
			}
		}

		// Add any task specs that aren't currently displayed but should be. Note
		// that in this case we need to send ALL tasks of this spec, not just the
		// new ones.
		for spec := range wantTaskSpecs {
			if !s.activeTaskSpecs[spec] {
				s.activeTaskSpecs[spec] = true
				for _, t := range tasksByName[spec] {
					tasksToSend[t.Id] = t
				}
			}
		}
		// Remove any task specs that are currently displayed but should not be.
		var hideTaskSpecs []string
		for spec := range s.activeTaskSpecs {
			if !wantTaskSpecs[spec] {
				delete(s.activeTaskSpecs, spec)
				hideTaskSpecs = append(hideTaskSpecs, spec)
			}
		}

		// Add the tasks to the response.
		displayTasks := make([]*rpc.Task, 0, len(tasksToSend))
		for _, t := range tasksToSend {
			displayTasks = append(displayTasks, &rpc.Task{
				Commits:        t.Commits,
				Name:           t.Name,
				Id:             t.Id,
				Revision:       t.Revision,
				Status:         string(t.Status),
				SwarmingTaskId: t.SwarmingTaskId,
				TaskExecutor:   t.TaskExecutor,
			})
		}
		return displayTasks, hideTaskSpecs, nil
	})
	if err != nil {
		return skerr.Wrap(err)
	}

	// Add comments.
	if len(newComments) > 0 {
		resp.Update.Comments = append(resp.Update.Comments, newComments...)
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
	}
	return nil
}

func (c *clientStream) matchBranch(branchName string) bool {
	return c.branchRegex == nil || c.branchRegex.MatchString(branchName)
}

// SendKeepalive sends a named "keepalive" event keeping the connection alive.
func (s *clientStream) SendKeepalive() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	_, err := fmt.Fprint(s.w, "event: keepalive\ndata: {}\n\n")
	if err != nil {
		return skerr.Wrapf(err, "failed to send SSE keepalive")
	}
	if err := s.w.Flush(); err != nil {
		return skerr.Wrapf(err, "failed to flush SSE stream")
	}
	return nil
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

type taskFilter string

const (
	taskFilter_All         = "All"
	taskFilter_Interesting = "Interesting"
	taskFilter_Failures    = "Failures"
	taskFilter_Search      = "Search"
	taskFilter_Default     = taskFilter_Interesting
)

func (f taskFilter) Valid() bool {
	switch f {
	case taskFilter_All, taskFilter_Interesting, taskFilter_Failures, taskFilter_Search:
		return true
	default:
		return false
	}
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
	if len(resp.Update.HideTaskSpecs) > 0 {
		return false
	}
	if len(resp.Update.Tasks) > 0 {
		return false
	}
	return true
}

func specMatchesFilter(tasks []*types.Task, filter taskFilter, search *regexp.Regexp) bool {
	if len(tasks) == 0 {
		return false
	}
	switch filter {
	case taskFilter_All:
		return true
	case taskFilter_Search:
		if search == nil {
			return true
		}
		return search.MatchString(tasks[0].Name)
	case taskFilter_Failures:
		for _, t := range tasks {
			if t.Done() && !t.Success() {
				return true
			}
		}
		return false
	case taskFilter_Interesting:
		// A task spec is considered interesting if it has both successes and
		// failures within the set of commits.
		hasSuccess := false
		hasFailure := false
		for _, task := range tasks {
			if !task.Done() {
				continue
			}
			if task.Success() {
				hasSuccess = true
			} else {
				hasFailure = true
			}
			if hasSuccess && hasFailure {
				return true
			}
		}
		return false
	}
	return false
}

func convertComments(rc *types.RepoComments) []*rpc.Comment {
	if rc == nil {
		return nil
	}
	var rv []*rpc.Comment
	for hash, comments := range rc.CommitComments {
		for _, c := range comments {
			rv = append(rv, &rpc.Comment{
				Commit:        hash,
				Id:            c.Id(),
				Repo:          c.Repo,
				Timestamp:     timestamppb.New(c.Timestamp),
				User:          c.User,
				IgnoreFailure: c.IgnoreFailure,
				Message:       c.Message,
				Deleted:       c.Deleted != nil && *c.Deleted,
			})
		}
	}
	for _, comments := range rc.TaskSpecComments {
		for _, c := range comments {
			rv = append(rv, &rpc.Comment{
				Id:            c.Id(),
				Repo:          c.Repo,
				TaskSpecName:  c.Name,
				Timestamp:     timestamppb.New(c.Timestamp),
				User:          c.User,
				Flaky:         c.Flaky,
				IgnoreFailure: c.IgnoreFailure,
				Message:       c.Message,
				Deleted:       c.Deleted != nil && *c.Deleted,
			})
		}
	}
	for hash, commitComments := range rc.TaskComments {
		for _, comments := range commitComments {
			for _, c := range comments {
				rv = append(rv, &rpc.Comment{
					Commit:       hash,
					Id:           c.Id(),
					Repo:         c.Repo,
					TaskSpecName: c.Name,
					Timestamp:    timestamppb.New(c.Timestamp),
					TaskId:       c.TaskId,
					User:         c.User,
					Message:      c.Message,
					Deleted:      c.Deleted != nil && *c.Deleted,
				})
			}
		}
	}
	return rv
}
