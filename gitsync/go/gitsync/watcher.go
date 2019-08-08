package main

import (
	"context"
	"runtime"
	"time"

	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	// batchSize is the size of a batch of commits that is imported into BTGit.
	batchSize = 10000
)

// RepoWatcher continuously watches a repository and uploads changes to a BigTable Gitstore.
type RepoWatcher struct {
	gitStore gitstore.GitStore
	repo     *gitiles.Repo
}

// NewRepoWatcher creates a GitStore with the provided information. Its
// Start(...) method will watch a repo in the background.
func NewRepoWatcher(ctx context.Context, conf *bt_gitstore.BTConfig, repoURL, gitcookiesPath string) (*RepoWatcher, error) {
	gitStore, err := bt_gitstore.New(ctx, conf, repoURL)
	if err != nil {
		return nil, skerr.Fmt("Error instantiating git store: %s", err)
	}
	return &RepoWatcher{
		gitStore: gitStore,
		repo:     gitiles.NewRepo(repoURL, gitcookiesPath, nil),
	}, nil
}

// Start watches the repo in the background and updates the BT GitStore. The frequency is
// defined by 'interval'.
func (r *RepoWatcher) Start(ctx context.Context, interval time.Duration) {
	lvGitSync := metrics2.NewLiveness("last_successful_git_sync", map[string]string{"repo": r.repo.URL})
	go util.RepeatCtx(interval, ctx, func() {
		// Catch any panic and log relevant information to find the root cause.
		defer func() {
			if err := recover(); err != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				sklog.Errorf("Panic updating %s:  %s\n%s", r.repo.URL, err, buf)
			}
		}()

		if err := r.updateFn(); err != nil {
			sklog.Errorf("Error updating %s: %s", r.repo.URL, err)
		} else {
			lvGitSync.Reset()
		}
	})
}

// updateFn retrieves git info from the repository and updates the GitStore.
func (r *RepoWatcher) updateFn() error {
	// Update the git repo.
	ctx := context.Background()

	// Get the branches from the repo.
	sklog.Info("Getting branches...")
	branches, err := r.repo.Branches()
	if err != nil {
		return skerr.Fmt("Failed to get branches from Git repo: %s", err)
	}

	// Get the current branches from the GitStore.
	currBranches, err := r.gitStore.GetBranches(ctx)
	if err != nil {
		return skerr.Fmt("Error retrieving branches from GitStore: %s", err)
	}

	// Find the commits that need to be added to the GitStore. This
	// considers all branches in the repo and whether they are already in
	// the GitStore.
	// TODO(borenet): Hide the latency of pulling from gitiles and pushing
	// to gitstore as much as possible, ie. pull the next batch from gitiles
	// while we're pushing the previous into gitstore.
	newCommits := []*vcsinfo.LongCommit{}
	for _, newBranch := range branches {
		// See if we have the branch in the repo already.
		foundBranch, ok := currBranches[newBranch.Name]
		if ok {
			// If the branch hasn't changed we are done.
			if foundBranch.Head == newBranch.Head {
				continue
			}

			// Load new commits from the repo.
			commits, err := r.repo.Log(foundBranch.Head, newBranch.Head)
			if err != nil {
				return skerr.Fmt("Error loading commit range %s..%s: %s", foundBranch.Head, newBranch.Head, err)
			}
			if len(commits) == 0 {
				// History has been changed. We have no choice
				// but to load commits until we find one we've
				// already seen.
				ok = false
			} else {
				newCommits = append(newCommits, commits...)
			}
		}

		// If we haven't seen this branch before, or if history has
		// changed, load commits until we find one we've seen before.
		if !ok {
			if err := r.repo.LogFnBatch(newBranch.Head, func(commits []*vcsinfo.LongCommit) error {
				hashes := make([]string, 0, len(commits))
				for _, c := range commits {
					hashes = append(hashes, c.Hash)
				}
				exist, err := r.gitStore.Get(ctx, hashes)
				if err != nil {
					return err
				}
				for idx, c := range commits {
					if exist[idx] != nil {
						return gitiles.ErrStopIteration
					}
					newCommits = append(newCommits, c)
				}
				return nil
			}); err != nil {
				return skerr.Fmt("Error retrieving new commits for branch %q: %s", newBranch.Name, err)
			}
		}
	}
	sklog.Infof("Repo @ %s: Found %d new commits in %d branches.", r.repo.URL, len(newCommits), len(branches))

	// Insert the new commits into the GitStore.
	if err := r.gitStore.Put(ctx, newCommits); err != nil {
		return skerr.Fmt("Error writing commits to BigTable: %s", err)
	}
	branchMap := make(map[string]string, len(branches))
	for _, gb := range branches {
		branchMap[gb.Name] = gb.Head
	}
	if err := r.gitStore.PutBranches(ctx, branchMap); err != nil {
		return skerr.Fmt("Error calling PutBranches on GitStore: %s", err)
	}
	sklog.Infof("Repo @ %s: Branches updated successfully.", r.repo.URL)
	return nil
}
