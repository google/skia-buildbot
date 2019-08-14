package watcher

import (
	"context"
	"fmt"
	"path"
	"runtime"
	"time"

	"go.skia.org/infra/go/cleanup"
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
func NewRepoWatcher(ctx context.Context, conf *bt_gitstore.BTConfig, repoURL, gitcookiesPath string) (*RepoWatcher, error) {
	gitStore, err := bt_gitstore.New(ctx, conf, repoURL)
	if err != nil {
		return nil, skerr.Fmt("Error instantiating git store: %s", err)
	}
	gr := gitiles.NewRepo(repoURL, gitcookiesPath, nil)
	ri, err := newRepoImpl(ctx, gitStore, gr)
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
	branches []*git.Branch
	commits  map[string]*vcsinfo.LongCommit
	gitiles  *gitiles.Repo
	gitstore gitstore.GitStore
}

// newRepoImpl returns a repograph.RepoImpl which uses both Gitiles and
// GitStore.
func newRepoImpl(ctx context.Context, gs gitstore.GitStore, repo *gitiles.Repo) (repograph.RepoImpl, error) {
	indexCommits, err := gs.RangeByTime(ctx, vcsinfo.MinTime, vcsinfo.MaxTime, "")
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
		branches: branches,
		commits:  commitsMap,
		gitiles:  repo,
		gitstore: gs,
	}, nil
}

// ingestCommits runs GitStore ingestion in a separate goroutine while the
// passed-in func loads commits from Gitiles.
func (r *repoImpl) ingestCommits(ctx context.Context, fn func(context.Context, chan<- []*vcsinfo.LongCommit) error) error {
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
			// Insert the new commits into GitStore.
			// TODO(borenet): Can we merge batches from commitsCh?
			// That'd be more efficient if GitStore is slower than
			// Gitiles.
			err = r.gitstore.Put(ctx, commits)
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
	loadingErr := fn(ctx, commitsCh)

	// Close the commits channel, wait for the goroutine to complete.
	close(commitsCh)
	gitstoreErr := <-errCh

	// The error returned from the gitstore goroutine takes precedence,
	// because we cancel the context when gitstore.Put fails, and thus fn()
	// may return an error simply stating that the context was canceled.
	if gitstoreErr != nil {
		if loadingErr != nil {
			return fmt.Errorf("GitStore ingestion failed with: %s; and commit-loading func failed with: %s", gitstoreErr, loadingErr)
		}
		return gitstoreErr
	}
	return loadingErr
}

// loadCommitsFromGitiles loads commits from Gitiles and pushes them onto the
// given channel, until we reach the optional stopAt commit, or any other commit
// we've seen before.
func (r *repoImpl) loadCommitsFromGitiles(ctx context.Context, startAt, stopAt string, commitsCh chan<- []*vcsinfo.LongCommit) error {
	return r.gitiles.LogFnBatch(ctx, startAt, func(ctx context.Context, commits []*vcsinfo.LongCommit) (rvErr error) {
		stopIdx := len(commits)
		for idx, c := range commits {
			// Stop when we reach the previous branch head or
			// any other commit we've seen before.
			if _, ok := r.commits[c.Hash]; ok || c.Hash == stopAt {
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

// See documentation for RepoImpl interface.
func (r *repoImpl) Update(ctx context.Context) error {
	// Find the old and new branch heads.
	oldBranches := make(map[string]*git.Branch, len(r.branches))
	for _, branch := range r.branches {
		oldBranches[branch.Name] = branch
	}
	branches, err := r.gitiles.Branches(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed loading branches from Gitiles.")
	}

	// Ingest any new commits.
	if err := r.ingestCommits(ctx, func(ctx context.Context, commitsCh chan<- []*vcsinfo.LongCommit) error {
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
			if err := r.loadCommitsFromGitiles(ctx, branch.Head, oldBranchHead, commitsCh); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	branchMap := make(map[string]string, len(branches))
	for _, branch := range branches {
		branchMap[branch.Name] = branch.Head
	}
	if err := r.gitstore.PutBranches(ctx, branchMap); err != nil {
		return err
	}
	sklog.Infof("Repo %s has %d commits and %d branches.", r.gitiles.URL, len(r.commits), len(branches))
	for _, b := range branches {
		sklog.Infof("  branch %s @ %s", b.Name, b.Head)
	}
	r.branches = branches
	return nil
}

// See documentation for RepoImpl interface.
func (r *repoImpl) Details(ctx context.Context, hash string) (*vcsinfo.LongCommit, error) {
	if c, ok := r.commits[hash]; ok {
		return c, nil
	}
	// Fall back to retrieving from Gitiles, while ingesting any new commits
	// into GitStore.
	if err := r.ingestCommits(ctx, func(ctx context.Context, commitsCh chan<- []*vcsinfo.LongCommit) error {
		return r.loadCommitsFromGitiles(ctx, hash, "", commitsCh)
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
func (r *repoImpl) UpdateCallback(_ context.Context, _ *repograph.Graph) error {
	return nil
}
