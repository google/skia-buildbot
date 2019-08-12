package main

import (
	"context"
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
	sklog.Infof("Repo %s has %d commits so far.", repo.URL, len(commits))
	gb, err := gs.GetBranches(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed loading branches from GitStore.")
	}
	branches := make([]*git.Branch, 0, len(gb))
	for name, branch := range gb {
		sklog.Debugf("%s@%s: %d, %s", path.Base(repo.URL), name, branch.Index, branch.Head)
		branches = append(branches, &git.Branch{
			Name: name,
			Head: branch.Head,
		})
	}
	commitsMap := make(map[string]*vcsinfo.LongCommit, len(commits))
	for _, c := range commits {
		commitsMap[c.Hash] = c
	}
	return &repoImpl{
		branches: branches,
		commits:  commitsMap,
		gitiles:  repo,
		gitstore: gs,
	}, nil
}

// See documentation for RepoImpl interface.
func (r *repoImpl) Update(ctx context.Context) error {
	// Load new data from gitiles and push it into gitstore.
	oldBranches := make(map[string]*git.Branch, len(r.branches))
	for _, branch := range r.branches {
		oldBranches[branch.Name] = branch
	}
	branches, err := r.gitiles.Branches()
	if err != nil {
		return skerr.Wrapf(err, "Failed loading branches from Gitiles.")
	}
	for _, branch := range branches {
		oldBranch := oldBranches[branch.Name]
		if oldBranch != nil {
			// If there's nothing new, skip this branch.
			if branch.Head == oldBranch.Head {
				continue
			}
			// Find any new commits.
			commits, err := r.gitiles.Log(oldBranch.Head, branch.Head)
			if err != nil {
				return skerr.Wrapf(err, "Failed loading commits from Gitiles.")
			}
			if len(commits) > 0 {
				if err := r.gitstore.Put(ctx, commits); err != nil {
					return skerr.Wrapf(err, "Failed inserting commits into GitStore.")
				}
				for _, c := range commits {
					r.commits[c.Hash] = c
				}
				// Skip the below fallback case.
				continue
			} else {
				sklog.Warningf("History has changed for %s", r.gitiles.URL)
			}
		}
		// This is a new branch, or history has changed.
		sklog.Infof("Loading all commits for branch %s of %s", branch.Name, r.gitiles.URL)
		// Get() loads batches of commits until it finds one we've seen
		// before, so Get(branch.Head) will load the entire branch.
		_, err := r.Details(ctx, branch.Head)
		if err != nil {
			return err
		}
	}
	r.branches = branches
	return nil
}

// See documentation for RepoImpl interface.
func (r *repoImpl) Details(ctx context.Context, hash string) (*vcsinfo.LongCommit, error) {
	if c, ok := r.commits[hash]; ok {
		return c, nil
	}
	// We haven't seen this commit before. For efficiency's sake, don't load
	// a single commit at a time. Instead, load commits until we find one
	// we've seen before.
	addedCommits := 0
	if err := r.gitiles.LogFnBatch(hash, func(commits []*vcsinfo.LongCommit) error {
		stop := false
		newCommits := make([]*vcsinfo.LongCommit, 0, len(commits))
		for _, c := range commits {
			if _, ok := r.commits[c.Hash]; ok {
				stop = true
				break
			}
			newCommits = append(newCommits, c)
		}
		if err := r.gitstore.Put(ctx, newCommits); err != nil {
			return skerr.Wrapf(err, "Failed inserting commits into GitStore.")
		}
		for _, c := range newCommits {
			r.commits[c.Hash] = c
			addedCommits++
			if addedCommits%500 == 0 {
				sklog.Infof("Added %d commits so far.", addedCommits)
			}
		}
		if stop {
			return gitiles.ErrStopIteration
		}
		return nil
	}); err != nil {
		return nil, skerr.Wrapf(err, "Failed loading commits from Gitiles")
	}
	return r.commits[hash], nil
}

// See documentation for RepoImpl interface.
func (r *repoImpl) Branches(_ context.Context) ([]*git.Branch, error) {
	return r.branches, nil
}

// See documentation for RepoImpl interface.
func (r *repoImpl) UpdateCallback(_ context.Context, _ *repograph.Graph) error {
	return nil
}
