package watcher

import (
	"context"
	"fmt"
	"path"
	"runtime"
	"sync"
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
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/sync/errgroup"
)

const (
	// batchSize is the size of a batch of commits that is imported into BTGit.
	batchSize = 10000
)

// Start creates a GitStore with the provided information and starts periodic
// ingestion.
func Start(ctx context.Context, conf *bt_gitstore.BTConfig, repoURL, gitcookiesPath, gcsBucket, gcsPath string, interval time.Duration) error {
	gitStore, err := bt_gitstore.New(ctx, conf, repoURL)
	if err != nil {
		return skerr.Wrapf(err, "Error instantiating git store.")
	}
	gr := gitiles.NewRepo(repoURL, gitcookiesPath, nil)
	s, err := storage.NewClient(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create storage client.")
	}
	gcsClient := gcsclient.New(s, gcsBucket)
	ri, err := newRepoImpl(ctx, gitStore, gr, gcsClient, gcsPath)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create RepoImpl.")
	}
	repo, err := repograph.NewWithRepoImpl(ctx, ri)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create repo graph.")
	}

	// Start periodic ingestion.
	lvGitSync := metrics2.NewLiveness("last_successful_git_sync", map[string]string{"repo": repoURL})
	cleanup.Repeat(interval, func(ctx context.Context) {
		// Catch any panic and log relevant information to find the root cause.
		defer func() {
			if err := recover(); err != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				sklog.Errorf("Panic updating %s:  %s\n%s", repoURL, err, buf)
			}
		}()

		sklog.Infof("Updating %s...", repoURL)
		if err := repo.Update(ctx); err != nil {
			sklog.Errorf("Error updating %s: %s", repoURL, err)
		} else {
			gotBranches, err := gitStore.GetBranches(ctx)
			if err != nil {
				sklog.Errorf("Successfully updated %s but failed to retrieve branch heads: %s", repoURL, err)
			} else {
				sklog.Infof("Successfully updated %s", repoURL)
				for name, branch := range gotBranches {
					sklog.Debugf("  %s@%s: %d, %s", path.Base(repoURL), name, branch.Index, branch.Head)
				}
			}
			lvGitSync.Reset()
		}
	}, nil)
	return nil
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

type commitBatch struct {
	branch  string
	commits []*vcsinfo.LongCommit
}

// processCommits processes commits in a separate goroutine while the
// passed-in func loads commits from Gitiles.
func (r *repoImpl) processCommits(ctx context.Context, process func(context.Context, *commitBatch) error, loadCommits func(context.Context, chan<- *commitBatch) error) error {
	// Run GitStore ingestion in a goroutine. Create a cancelable context
	// to halt requests to Gitiles if GitStore ingestion fails.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	commitsCh := make(chan *commitBatch)
	errCh := make(chan error)
	go func() {
		var err error
		for cb := range commitsCh {
			if err != nil {
				// We've hit an error but we need to consume
				// all of the commits from the channel before
				// returning, or the passed-in func may block
				// forever.
				sklog.Warningf("Skipping %d commits due to previous error.", len(cb.commits))
				continue
			}
			// Process the commits.
			if process != nil {
				err = process(ctx, cb)
			}
			if err != nil {
				if err != gitiles.ErrStopIteration {
					sklog.Errorf("Encountered error: %s", err)
				}
				// Cancel the context we passed to fn(). If it
				// respects context.Done() as it's supposed to,
				// then it should exit early with an error.
				cancel()
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
		if processErr == gitiles.ErrStopIteration {
			return nil
		}
		if loadingErr != nil {
			return fmt.Errorf("GitStore ingestion failed with: %s; and commit-loading func failed with: %s", processErr, loadingErr)
		}
		return processErr
	}
	return loadingErr
}

// loadCommitsFromGitiles loads commits from Gitiles and pushes them onto the
// given channel, until we reach the optional from commit, or any other commit
// we've seen before.
func (r *repoImpl) loadCommitsFromGitiles(ctx context.Context, branch, logExpr string, commitsCh chan<- *commitBatch, opts ...gitiles.LogOption) error {
	return r.gitiles.LogFnBatch(ctx, logExpr, func(ctx context.Context, commits []*vcsinfo.LongCommit) error {
		commitsCh <- &commitBatch{
			branch:  branch,
			commits: commits,
		}
		return nil
	}, opts...)
}

// initialIngestion performs the first-time ingestion of the repo.
func (r *repoImpl) initialIngestion(ctx context.Context) error {
	sklog.Warningf("Performing initial ingestion of %s.", r.gitiles.URL)
	defer timer.New("Initial ingestion").Stop()

	// Create a tmpGitStore.
	sklog.Info("Retrieving graph from temporary store.")
	t := timer.New("Retrieving graph from temp store")
	graph, ri, err := setupInitialIngest(ctx, r.gcsClient, r.gcsPath, r.gitiles.URL)
	if err != nil {
		return skerr.Wrapf(err, "Failed initial ingestion.")
	}
	for _, c := range graph.GetAll() {
		r.commits[c.Hash] = c.LongCommit
	}
	oldBranches := map[string]string{}
	for _, b := range graph.BranchHeads() {
		oldBranches[b.Name] = b.Head
	}
	t.Stop()

	// Find the current set of branches.
	t = timer.New("Loading commits from gitiles")
	branches, err := r.gitiles.Branches(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed loading branches from Gitiles.")
	}

	// Load commits from gitiles.

	// We assume that master contains the majority of the commits in the
	// repo, and the other branches are comparatively small, with most of
	// their ancestry being on master itself.
	var master *git.Branch
	for _, b := range branches {
		if b.Name == "master" {
			master = b
			break
		}
	}
	if master != nil {
		sklog.Info("Loading commits from gitiles for master.")
		if err := r.processCommits(ctx, func(ctx context.Context, cb *commitBatch) error {
			// Add the new commits to our local cache.
			sklog.Infof("Adding batch of %d commits", len(cb.commits))
			for _, c := range cb.commits {
				r.commits[c.Hash] = c
			}
			return initialIngestCommitBatch(ctx, graph, ri, cb)
		}, func(ctx context.Context, commitsCh chan<- *commitBatch) error {
			logExpr := master.Head
			if oldHead, ok := oldBranches[master.Name]; ok {
				logExpr = fmt.Sprintf("%s..%s", oldHead, master.Head)
			}
			return r.loadCommitsFromGitiles(ctx, master.Name, logExpr, commitsCh, gitiles.Reverse(), gitiles.BatchSize(batchSize))
		}); err != nil {
			return skerr.Wrapf(err, "Failed to ingest commits for master.")
		}
	}
	ri.Wait()

	// Load commits for other branches, in non-reverse order, so that we can
	// stop once we reach commits already on the master branch.
	var egroup errgroup.Group
	var mtx sync.Mutex
	for _, branch := range branches {
		// https://golang.org/doc/faq#closures_and_goroutines
		branch := branch
		if branch == master {
			continue
		}
		sklog.Infof("Loading commits for %s", branch.Name)
		if _, ok := r.commits[branch.Head]; ok {
			sklog.Infof(" ... already have %s, skip", branch.Head)
			continue
		}
		egroup.Go(func() error {
			var commits []*vcsinfo.LongCommit
			mtx.Lock()
			localCache := make(map[string]*vcsinfo.LongCommit, len(r.commits))
			for h, c := range r.commits {
				localCache[h] = c
			}
			mtx.Unlock()
			lookingFor := map[string]bool{}
			if err := r.processCommits(ctx, func(ctx context.Context, cb *commitBatch) error {
				numIngested := 0
				defer func() {
					sklog.Infof("Added %d of batch of %d commits", numIngested, len(cb.commits))
				}()
				for _, c := range cb.commits {
					delete(lookingFor, c.Hash)
					if _, ok := localCache[c.Hash]; !ok {
						commits = append(commits, c)
						localCache[c.Hash] = c
						numIngested++
					}
					for _, p := range c.Parents {
						if _, ok := localCache[p]; !ok {
							lookingFor[p] = true
						}
					}
					if len(lookingFor) == 0 {
						return gitiles.ErrStopIteration
					}
				}
				return nil
			}, func(ctx context.Context, commitsCh chan<- *commitBatch) error {
				logExpr := branch.Head
				if oldHead, ok := oldBranches[branch.Name]; ok {
					logExpr = fmt.Sprintf("%s..%s", oldHead, branch.Head)
				}
				return r.loadCommitsFromGitiles(ctx, branch.Name, logExpr, commitsCh, gitiles.BatchSize(batchSize))
			}); err != nil {
				return skerr.Wrapf(err, "Failed to ingest commits.")
			}
			// Reverse the slice of commits so that they can be added in
			// order.
			for i := 0; i < len(commits)/2; i++ {
				j := len(commits) - i - 1
				commits[i], commits[j] = commits[j], commits[i]
			}
			mtx.Lock()
			defer mtx.Unlock()
			if err := initialIngestCommitBatch(ctx, graph, ri, &commitBatch{
				branch:  branch.Name,
				commits: commits,
			}); err != nil {
				return skerr.Wrapf(err, "Failed to add commits to graph for %s", branch.Name)
			}
			for _, c := range commits {
				r.commits[c.Hash] = c
			}
			sklog.Infof("Loading commits for %s done", branch.Name)
			return nil
		})
	}
	if err := egroup.Wait(); err != nil {
		return skerr.Wrap(err)
	}
	ri.Wait()
	t.Stop()

	// Replace the fake branches with real ones. Update branch membership.
	ri.BranchList = branches
	if err := graph.Update(ctx); err != nil {
		return skerr.Wrapf(err, "Failed final Graph update.")
	}
	r.branches = branches
	sklog.Infof("Finished initial ingestion of %s.", r.gitiles.URL)
	return nil
}

// See documentation for RepoImpl interface.
func (r *repoImpl) Update(ctx context.Context) error {
	if len(r.branches) == 0 {
		if err := r.initialIngestion(ctx); err != nil {
			return skerr.Wrapf(err, "Failed initial ingestion.")
		}
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
	if err := r.processCommits(ctx, func(ctx context.Context, cb *commitBatch) error {
		// Add the new commits to our local cache.
		for _, c := range cb.commits {
			r.commits[c.Hash] = c
		}
		return nil
	}, func(ctx context.Context, commitsCh chan<- *commitBatch) error {
		for _, branch := range branches {
			logExpr := branch.Head
			if oldBranch, ok := oldBranches[branch.Name]; ok {
				// If there's nothing new, skip this branch.
				if branch.Head == oldBranch.Head {
					continue
				}
				// Only load back to the previous branch head.
				logExpr = fmt.Sprintf("%s..%s", oldBranch.Head, branch.Head)
			}
			if err := r.loadCommitsFromGitiles(ctx, branch.Name, logExpr, commitsCh); err != nil {
				return skerr.Wrapf(err, "Failed loading commits for %s", branch.Head)
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
	lookingFor := map[string]bool{}
	if err := r.processCommits(ctx, func(ctx context.Context, cb *commitBatch) error {
		// Add the new commits to our local cache.
		for _, c := range cb.commits {
			delete(lookingFor, c.Hash)
			for _, p := range c.Parents {
				if _, ok := r.commits[p]; !ok {
					lookingFor[p] = true
				}
			}
			r.commits[c.Hash] = c
			if len(lookingFor) == 0 {
				return gitiles.ErrStopIteration
			}
		}
		return nil
	}, func(ctx context.Context, commitsCh chan<- *commitBatch) error {
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
	sklog.Infof("Putting %d new commits into GitStore.", len(added))
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
