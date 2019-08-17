package watcher

import (
	"context"
	"fmt"
	"path"
	"runtime"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	// batchSize is the size of a batch of commits that is imported into BTGit.
	batchSize = 10000
)

// RepoWatcher continuously watches a repository and uploads changes to a BigTable Gitstore.
type RepoWatcher struct {
	gitStore gitstore.GitStore
	repo     *repograph.Graph
	repoURL  string
}

// NewRepoWatcher creates a GitStore with the provided information. Its
// Start(...) method will watch a repo in the background.
func NewRepoWatcher(ctx context.Context, conf *bt_gitstore.BTConfig, repoURL, gitcookiesPath, gcsBucket, gcsPath string) (*RepoWatcher, error) {
	gitStore, err := bt_gitstore.New(ctx, conf, repoURL)
	if err != nil {
		return nil, skerr.Fmt("Error instantiating git store: %s", err)
	}
	gr := gitiles.NewRepo(repoURL, gitcookiesPath, nil)
	s, err := storage.NewClient(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create storage client.")
	}
	gcsClient := gcsclient.New(s, gcsBucket)
	ri, err := newRepoImpl(ctx, gitStore, gr, gcsClient, gcsPath)
	if err != nil {
		return nil, err
	}
	repo, err := repograph.NewWithRepoImpl(ctx, ri)
	if err != nil {
		return nil, err
	}
	return &RepoWatcher{
		gitStore: gitStore,
		repo:     repo,
		repoURL:  repoURL,
	}, nil
}

// Start watches the repo in the background and updates the BT GitStore. The frequency is
// defined by 'interval'.
func (r *RepoWatcher) Start(ctx context.Context, interval time.Duration) {
	lvGitSync := metrics2.NewLiveness("last_successful_git_sync", map[string]string{"repo": r.repoURL})
	cleanup.Repeat(interval, func(ctx context.Context) {
		// Catch any panic and log relevant information to find the root cause.
		defer func() {
			if err := recover(); err != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				sklog.Errorf("Panic updating %s:  %s\n%s", r.repoURL, err, buf)
			}
		}()

		if err := r.updateFn(ctx); err != nil {
			sklog.Errorf("Error updating %s: %s", r.repoURL, err)
		} else {
			lvGitSync.Reset()
		}
	}, nil)
}

// updateFn retrieves git info from the repository and updates the GitStore.
func (r *RepoWatcher) updateFn(ctx context.Context) error {
	sklog.Infof("Updating %s...", r.repoURL)
	return r.repo.UpdateWithCallback(ctx, func(g *repograph.Graph) error {
		branches := g.BranchHeads()
		branchMap := make(map[string]string, len(branches))
		for _, b := range branches {
			branchMap[b.Name] = b.Head
		}
		if err := r.gitStore.PutBranches(ctx, branchMap); err != nil {
			return err
		}
		gotBranches, err := r.gitStore.GetBranches(ctx)
		if err != nil {
			sklog.Errorf("Successfully updated %s but failed to retrieve branch heads: %s", r.repoURL, err)
			return nil
		}
		sklog.Infof("Successfully updated %s", r.repoURL)
		for name, branch := range gotBranches {
			sklog.Debugf("  %s@%s: %d, %s", path.Base(r.repoURL), name, branch.Index, branch.Head)
		}
		return nil
	})
}

// repoImpl is an implementation of repograph.RepoImpl which loads commits into
// a GitStore.
type repoImpl struct {
	branches  []*git.Branch
	commits   map[string]*vcsinfo.LongCommit
	gcsClient gcs.GCSClient
	gcsPath   string
	gitiles   *gitiles.Repo
	gitstore  gitstore.GitStore
}

// newRepoImpl returns a repograph.RepoImpl which uses both Gitiles and
// GitStore.
func newRepoImpl(ctx context.Context, gs gitstore.GitStore, repo *gitiles.Repo, gcsClient gcs.GCSClient, gcsPath string) (repograph.RepoImpl, error) {
	indexCommits, err := gs.RangeByTime(ctx, vcsinfo.MinTime, vcsinfo.MaxTime, gitstore.ALL_BRANCHES)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed loading IndexCommits from GitStore.")
	}
	hashes := make([]string, 0, len(indexCommits))
	for _, c := range indexCommits {
		hashes = append(hashes, c.Hash)
	}
	commits, err := gs.Get(ctx, hashes)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed loading LongCommits from GitStore.")
	}
	gb, err := gs.GetBranches(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed loading branches from GitStore.")
	}
	branches := make([]*git.Branch, 0, len(gb))
	for name, branch := range gb {
		branches = append(branches, &git.Branch{
			Name: name,
			Head: branch.Head,
		})
	}
	commitsMap := make(map[string]*vcsinfo.LongCommit, len(commits))
	for _, c := range commits {
		commitsMap[c.Hash] = c
	}
	sklog.Infof("Repo %s has %d commits and %d branches.", repo.URL, len(commits), len(branches))
	for _, b := range branches {
		sklog.Infof("  branch %s @ %s", b.Name, b.Head)
	}
	return &repoImpl{
		branches:  branches,
		commits:   commitsMap,
		gcsClient: gcsClient,
		gcsPath:   gcsPath,
		gitiles:   repo,
		gitstore:  gs,
	}, nil
}

// store is a simple interface which allows ingestCommits to write either to a
// GitStore or tmpGitStore.
type store interface {
	Put(context.Context, []*vcsinfo.LongCommit) error
}

// processCommits processes commits in a separate goroutine while the
// passed-in func loads commits from Gitiles.
func (r *repoImpl) processCommits(ctx context.Context, process func(context.Context, []*vcsinfo.LongCommit) error, loadCommits func(context.Context, chan<- []*vcsinfo.LongCommit) error) error {
	// Run GitStore ingestion in a goroutine. Create a cancelable context
	// to halt requests to Gitiles if GitStore ingestion fails.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	commitsCh := make(chan []*vcsinfo.LongCommit)
	errCh := make(chan error)
	go func() {
		var err error
		for commits := range commitsCh {
			if err != nil {
				// We've hit an error but we need to consume
				// all of the commits from the channel before
				// returning, or the passed-in func may block
				// forever.
				continue
			}
			sklog.Infof("Processing batch of %d commits", len(commits))
			// Process the commits.
			err = process(ctx, commits)
			if err != nil {
				// Cancel the context we passed to fn(). If it
				// respects context.Done() as it's supposed to,
				// then it should exit early with an error.
				cancel()
			} else {
				// Add the new commits to our local cache.
				for _, c := range commits {
					r.commits[c.Hash] = c
				}
			}
		}
		// Signal that we're done ingesting commits.
		errCh <- err
	}()

	// Run the passed-in func.
	loadingErr := loadCommits(ctx, commitsCh)

	// Close the commits channel, wait for the goroutine to complete.
	close(commitsCh)
	processErr := <-errCh

	// The error returned from the gitstore goroutine takes precedence,
	// because we cancel the context when gitstore.Put fails, and thus fn()
	// may return an error simply stating that the context was canceled.
	if processErr != nil {
		if loadingErr != nil {
			return fmt.Errorf("GitStore ingestion failed with: %s; and commit-loading func failed with: %s", processErr, loadingErr)
		}
		return processErr
	}
	return loadingErr
}

// ingestCommits ingests commits into GitStore in a separate goroutine while the
// passed-in func loads commits from Gitiles.
func (r *repoImpl) ingestCommits(ctx context.Context, gs store, fn func(context.Context, chan<- []*vcsinfo.LongCommit) error) error {
	return r.processCommits(ctx, gs.Put, fn)
}

// cacheCommits caches commits downloaded by the passed-in func.
func (r *repoImpl) cacheCommits(ctx context.Context, fn func(context.Context, chan<- []*vcsinfo.LongCommit) error) error {
	return r.processCommits(ctx, func(_ context.Context, _ []*vcsinfo.LongCommit) error { return nil }, fn)
}

// loadCommitsFromGitiles loads commits from Gitiles and pushes them onto the
// given channel, until we reach the optional from commit, or any other commit
// we've seen before.
func (r *repoImpl) loadCommitsFromGitiles(ctx context.Context, from, to string, commitsCh chan<- []*vcsinfo.LongCommit) error {
	sklog.Errorf("log %s..%s", from, to)
	return r.gitiles.LogFnBatch(ctx, to, func(ctx context.Context, commits []*vcsinfo.LongCommit) (rvErr error) {
		stopIdx := len(commits)
		for idx, c := range commits {
			// Stop when we reach the previous branch head or
			// any other commit we've seen before.
			if _, ok := r.commits[c.Hash]; ok || c.Hash == from {
				stopIdx = idx
				rvErr = gitiles.ErrStopIteration
			}
		}
		commits = commits[:stopIdx]
		if len(commits) > 0 {
			commitsCh <- commits[:stopIdx]
		}
		return
	})
}

// initialIngestion performs the first-time ingestion of the repo.
func (r *repoImpl) initialIngestion(ctx context.Context) error {
	sklog.Warningf("Performing initial ingestion of %s.", r.gitiles.URL)

	// Create a tmpGitStore.
	tmp, err := newGCSTmpGitStore(ctx, r.gcsClient, r.gcsPath, r.gitiles.URL)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create GCS client.")
	}

	// We may have already attempted the initial ingestion and failed; load
	// all commits from the tmpGitStore.
	sklog.Info("Retrieving commits from temporary store.")
	commits, err := tmp.GetAll(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed to retrieve commits from temporary store.")
	}
	r.commits = commits
	sklog.Infof("Loaded %d commits from temporary store.", len(r.commits))

	// Find the current set of branches.
	branches, err := r.gitiles.Branches(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed loading branches from Gitiles.")
	}

	// Push all commits into the tmpGitStore.
	sklog.Infof("Loading commits from gitiles for %d branches.", len(branches))
	if err := r.ingestCommits(ctx, tmp, func(ctx context.Context, commitsCh chan<- []*vcsinfo.LongCommit) error {
		for _, branch := range branches {
			if err := r.loadCommitsFromGitiles(ctx, "", branch.Head, commitsCh); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return skerr.Wrapf(err, "Failed to ingest commits.")
	}

	// Build the graph.
	sklog.Info("Building repo graph.")
	ri := repograph.NewMemCacheRepoImpl(r.commits, branches)
	graph, err := repograph.NewWithRepoImpl(ctx, ri)
	if err != nil {
		return skerr.Wrapf(err, "Failed to build Graph.")
	}
	graph.UpdateBranchInfo()

	// Load the commits into GitStore.
	sklog.Infof("Putting %d commits into GitStore.", graph.Len())
	putCommits := make([]*vcsinfo.LongCommit, 0, graph.Len())
	for _, c := range graph.GetAll() {
		putCommits = append(putCommits, c.LongCommit)
	}
	if err := r.gitstore.Put(ctx, putCommits); err != nil {
		return skerr.Wrapf(err, "Failed to put commits into GitStore.")
	}
	sklog.Info("Putting branches into GitStore.")
	branchMap := make(map[string]string, len(branches))
	for _, b := range branches {
		branchMap[b.Name] = b.Head
	}
	if err := r.gitstore.PutBranches(ctx, branchMap); err != nil {
		return skerr.Wrapf(err, "Failed to put branches into GitStore.")
	}
	r.branches = branches

	// Delete the tmpGitStore.
	sklog.Infof("Deleting temporary store.")
	if err := tmp.Delete(ctx); err != nil {
		sklog.Errorf("Failed to delete temporary store: %s", err)
	}
	sklog.Infof("Finished initial ingestion of %s.", r.gitiles.URL)
	return nil
}

// See documentation for RepoImpl interface.
func (r *repoImpl) Update(ctx context.Context) error {
	if len(r.branches) == 0 {
		return r.initialIngestion(ctx)
	}

	// Find the old and new branch heads.
	oldBranches := make(map[string]*git.Branch, len(r.branches))
	for _, branch := range r.branches {
		oldBranches[branch.Name] = branch
	}
	branches, err := r.gitiles.Branches(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed loading branches from Gitiles.")
	}

	// Download any new commits and add them to the local cache.
	if err := r.cacheCommits(ctx, func(ctx context.Context, commitsCh chan<- []*vcsinfo.LongCommit) error {
		for _, branch := range branches {
			oldBranch := oldBranches[branch.Name]
			oldBranchHead := ""
			if oldBranch != nil {
				// If there's nothing new, skip this branch.
				if branch.Head == oldBranch.Head {
					continue
				}
				// Only load back to the previous branch head.
				oldBranchHead = oldBranch.Head
			}
			if err := r.loadCommitsFromGitiles(ctx, oldBranchHead, branch.Head, commitsCh); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	r.branches = branches
	return nil
}

// See documentation for RepoImpl interface.
func (r *repoImpl) Details(ctx context.Context, hash string) (*vcsinfo.LongCommit, error) {
	if c, ok := r.commits[hash]; ok {
		return c, nil
	}
	// Fall back to retrieving from Gitiles, store any new commits in the
	// local cache.
	sklog.Errorf("Missing commit %s", hash)
	if err := r.cacheCommits(ctx, func(ctx context.Context, commitsCh chan<- []*vcsinfo.LongCommit) error {
		return r.loadCommitsFromGitiles(ctx, "", hash, commitsCh)
	}); err != nil {
		return nil, err
	}
	c, ok := r.commits[hash]
	if !ok {
		return nil, fmt.Errorf("Commit %s is still missing despite attempting to load it from gitiles.", hash)
	}
	return c, nil
}

// See documentation for RepoImpl interface.
func (r *repoImpl) Branches(_ context.Context) ([]*git.Branch, error) {
	return r.branches, nil
}

// See documentation for RepoImpl interface.
func (r *repoImpl) UpdateCallback(ctx context.Context, added, removed []*vcsinfo.LongCommit, graph *repograph.Graph) error {
	// Ensure that branch membership is up to date.
	modified := graph.UpdateBranchInfo()
	modifiedMap := make(map[string]*vcsinfo.LongCommit, len(modified)+len(added))
	for _, c := range added {
		modifiedMap[c.Hash] = c
	}
	for _, c := range modified {
		modifiedMap[c.Hash] = c
	}
	// Don't include commits in the 'removed' list.
	for _, c := range removed {
		delete(modifiedMap, c.Hash)
	}
	putCommits := make([]*vcsinfo.LongCommit, 0, len(modifiedMap))
	for _, c := range modifiedMap {
		putCommits = append(putCommits, c)
	}
	sklog.Infof("Inserting %d commits (%d added, %d modified)", len(putCommits), len(added), len(modified))
	for _, b := range graph.BranchHeads() {
		sklog.Infof("%s @ %s", b.Name, b.Head)
	}
	if err := r.gitstore.Put(ctx, putCommits); err != nil {
		return skerr.Wrapf(err, "Failed putting commits into GitStore.")
	}
	// TODO(borenet): Should we delete commits which were removed?
	branchHeads := graph.BranchHeads()
	branches := make(map[string]string, len(branchHeads))
	for _, b := range branchHeads {
		branches[b.Name] = b.Head
	}
	return r.gitstore.PutBranches(ctx, branches)
}
