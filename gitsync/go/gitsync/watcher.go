package main

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/git"
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
	repo     *git.Repo
	repoDir  string
	repoURL  string
}

// NewRepoWatcher creates a GitStore with the provided information and checks out the git repo
// at repoURL into repoDir. It's Start(...) function will watch a repo in the background.
func NewRepoWatcher(ctx context.Context, conf *bt_gitstore.BTConfig, repoURL, repoDir string) (*RepoWatcher, error) {
	repoDir, err := fileutil.EnsureDirExists(repoDir)
	if err != nil {
		return nil, err
	}

	gitStore, err := bt_gitstore.New(ctx, conf, repoURL)
	if err != nil {
		return nil, skerr.Fmt("Error instantiating git store: %s", err)
	}

	repo, err := git.NewRepo(ctx, repoURL, repoDir)
	if err != nil {
		return nil, fmt.Errorf("Failed to create git repo: %s", err)
	}

	return &RepoWatcher{
		gitStore: gitStore,
		repo:     repo,
		repoDir:  repoDir,
		repoURL:  repoURL,
	}, nil
}

// Start watches the repo in the background and updates the BT GitStore. The frequency is
// defined by 'interval'.
func (r *RepoWatcher) Start(ctx context.Context, interval time.Duration) {
	lvGitSync := metrics2.NewLiveness("last_successful_git_sync", map[string]string{"repo": r.repoURL})
	go util.RepeatCtx(interval, ctx, func(ctx context.Context) {
		// Catch any panic and log relevant information to find the root cause.
		defer func() {
			if err := recover(); err != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				sklog.Errorf("Panic updating %s in %s:  %s\n%s", r.repoURL, r.repoDir, err, buf)
			}
		}()

		if err := r.updateFn(); err != nil {
			sklog.Errorf("Error updating %s: %s", r.repoURL, err)
		} else {
			lvGitSync.Reset()
		}
	})
}

// updateFn retrieves git info from the repository and updates the GitStore.
func (r *RepoWatcher) updateFn() error {
	// Update the git repo.
	ctx := context.Background()
	sklog.Infof("Updating repo ...")
	if err := r.repo.Update(ctx); err != nil {
		return skerr.Fmt("Failed to update repo: %s", err)
	}

	// Get the branches from the repo.
	sklog.Info("Getting branches...")
	branches, err := r.repo.Branches(ctx)
	if err != nil {
		return skerr.Fmt("Failed to get branches from Git repo: %s", err)
	}

	// Get the current branches from the GitStore.
	currBranches, err := r.gitStore.GetBranches(ctx)
	if err != nil {
		return skerr.Fmt("Error retrieving branches from GitStore: %s", err)
	}

	// Find the hashes all all commits that need to be added to the GitStore. This
	// considers all branches in the repo and whether they are already in the GitStore.
	hashes := util.StringSet{}
	for _, newBranch := range branches {
		// revListStr is an argument to repo.RevList below and controls how many commits we
		// retrieve. By default we retrieve all commits in the branch, but may restrict that if
		// we find an ancester to the current branch (see below).
		revListStr := newBranch.Head

		// See if we have the branch in the repo already.
		foundBranch, ok := currBranches[newBranch.Name]
		if ok {
			// If the branch hasn't changed we  are done.
			if foundBranch.Head == newBranch.Head {
				continue
			}

			// See if the new branch head is a descendant of the old branch head.
			anc, err := r.repo.IsAncestor(ctx, foundBranch.Head, newBranch.Head)
			if err != nil {
				return skerr.Fmt("Error checking if %s is an ancestor of %s: %s", foundBranch.Head, newBranch.Head, err)
			}

			if anc {
				// Only get the commits between the old and new head.
				revListStr = fmt.Sprintf("%s..%s", foundBranch.Head, newBranch.Head)
			}
		}

		// Retrieve the target commits.
		foundHashes, err := r.repo.RevList(ctx, "--topo-order", revListStr)
		if err != nil {
			return skerr.Fmt("Error retrieving hashes with the argument %q: %s", revListStr, err)
		}
		hashes.AddLists(foundHashes)
	}
	sklog.Infof("Repo @ %s: Found %d unique hashes in %d branches.", r.repoURL, len(hashes), len(branches))

	// Iterate over the LongCommits that correspond to batches.
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	commitsCh, err := r.iterateLongCommits(ctx, hashes.Keys(), batchSize)
	if err != nil {
		return skerr.Fmt("Error iterating over new commits: %s", err)
	}

	// Make sure we iterate over all commits, so we don't leak go-routine.
	for commits := range commitsCh {
		if err := r.gitStore.Put(ctx, commits); err != nil {
			return skerr.Fmt("Error writing commits to BigTable: %s", err)
		}
	}

	branchMap := make(map[string]string, len(branches))
	for _, gb := range branches {
		branchMap[gb.Name] = gb.Head
	}
	if err := r.gitStore.PutBranches(ctx, branchMap); err != nil {
		return skerr.Fmt("Error calling PutBranches on GitStore: %s", err)
	}
	sklog.Infof("Repo @ %s: Branches updated successfully.", r.repoURL)
	return nil
}

// iterateLongCommit returns batches of commits corresponding to the given hashes.
func (r *RepoWatcher) iterateLongCommits(ctx context.Context, hashes []string, batchSize int) (<-chan []*vcsinfo.LongCommit, error) {
	// Allocate a channel so can always send all batches and are not dependent on the speed of the receiver.
	retCh := make(chan []*vcsinfo.LongCommit, len(hashes)/batchSize+1)

	go func() {
		longCommits := make([]*vcsinfo.LongCommit, 0, batchSize)
		for idx, hash := range hashes {
			// Check whether the context has been canceled.
			select {
			case <-ctx.Done():
				return
			default:
			}

			c, err := r.repo.Details(ctx, hash)
			if err != nil {
				sklog.Errorf("Error fetching commit %q: %s", hash, err)
				continue
			}

			longCommits = append(longCommits, c)
			if len(longCommits) >= batchSize || idx == (len(hashes)-1) {
				retCh <- longCommits
				longCommits = make([]*vcsinfo.LongCommit, 0, batchSize)
			}
		}
		close(retCh)
	}()
	return retCh, nil
}
