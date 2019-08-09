package main

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"go.skia.org/infra/go/cleanup"
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
	// Create a repograph.Graph using the GitStore. This ensures that the
	// Graph matches the contents of the GitStore.
	gitStore, err := bt_gitstore.New(ctx, conf, repoURL)
	if err != nil {
		return nil, skerr.Fmt("Error instantiating git store: %s", err)
	}
	repo, err := repograph.NewGitStoreGraph(ctx, gitStore)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize repo graph: %s", err)
	}
	// Point the repograph.Graph at Gitiles.
	gr := gitiles.NewRepo(repoURL, gitcookiesPath, nil)
	repo.SwapRepoImpl(repograph.NewGitilesRepoImpl(gr))
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
	cleanup.Repeat(interval, func() {
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
	return r.repo.UpdateWithCallback(ctx, func(g *repograph.Graph, added, _ []*vcsinfo.LongCommit) error {
		if err := r.gitStore.Put(ctx, added); err != nil {
			return err
		}
		branches := g.BranchHeads()
		branchMap := make(map[string]string, len(branches))
		for _, b := range branches {
			branchMap[b.Name] = b.Head
		}
		if err := r.gitStore.PutBranches(ctx, branchMap); err != nil {
			return err
		}
		sklog.Infof("Found %d new commits for %s", len(added), r.repoURL)
		return nil
	})
}
