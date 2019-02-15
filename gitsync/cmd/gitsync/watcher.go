package main

import (
	"context"
	"runtime"
	"time"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/gitstore"
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
	gitInfo  *gitinfo.GitInfo
	repoDir  string
	repoURL  string
}

// NewRepoWatcher creates a GitStore with the provided information and checks out the git repo
// at repoURL into repoDir. It's Start(...) function will watch a repo in the background.
func NewRepoWatcher(conf *gitstore.BTConfig, repoURL, repoDir string) (*RepoWatcher, error) {
	gitStore, err := gitstore.NewBTGitStore(conf, repoURL)
	if err != nil {
		return nil, skerr.Fmt("Error instantiating git store: %s", err)
	}

	gitInfo, err := gitinfo.CloneOrUpdate(context.Background(), repoURL, repoDir, true)
	if err != nil {
		return nil, skerr.Fmt("Error instantiating GitInfo: %s", err)
	}

	return &RepoWatcher{
		gitStore: gitStore,
		gitInfo:  gitInfo,
		repoDir:  repoDir,
		repoURL:  repoURL,
	}, nil
}

// Start watches the repo in the background and updates the BT GitStore. The frequency is
// defined by 'interval'.
func (r *RepoWatcher) Start(ctx context.Context, interval time.Duration) {
	go util.RepeatCtx(interval, ctx, r.updateFn)
}

// updateFn retrieves git info from the repository and updates the GitStore.
func (r *RepoWatcher) updateFn() {
	// Catch any panic and log relevant information to find the root cause.
	defer func() {
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			sklog.Errorf("Panic updating %s in %s:  %s\n%s", r.repoURL, r.repoDir, err, buf)
		}
	}()

	// Update the git repo.
	ctx := context.Background()
	if err := r.gitInfo.Update(ctx, true, true); err != nil {
		sklog.Errorf("Error updating git repo: %s", err)
		return
	}

	// Retrieve the last commit added.
	branches, err := r.gitStore.GetBranches(ctx)
	if err != nil {
		sklog.Errorf("Error retrieving branches from GitStore: %s", err)
		return
	}

	startTime := gitstore.MinTime
	allCommits, ok := branches[""]
	for branchName, b := range branches {
		sklog.Infof("B: %s     %s   %d", branchName, b.Head, b.Index)
	}
	if ok {
		details, err := r.gitStore.Get(ctx, []string{allCommits.Head})
		if err != nil {
			sklog.Errorf("Error retrieving head. Continuing as if there were no commits in BT.")
		} else if details[0] == nil {
			sklog.Errorf("Unable to retrieve details for 'head' of all commits at %s", allCommits.Head)
		} else {
			startTime = details[0].Timestamp
		}
	}
	sklog.Infof("Start time: %v", startTime)

	// Iterate over the new commits.
	commitsCh, err := r.iterateLongCommits(batchSize, startTime)
	if err != nil {
		sklog.Errorf("Error iterating over new commits: %s", err)
	}

	for commits := range commitsCh {
		if err := r.gitStore.Put(ctx, commits); err != nil {
			sklog.Errorf("Error writing commits to BigTable: %s", err)
		}
	}

	// Write the branches to the BT store.
	gitBranches, err := r.gitInfo.GetBranches(ctx)
	if err != nil {
		sklog.Errorf("Error retrieving git branches via gitinfo: %s", err)
		return
	}
	branchMap := make(map[string]string, len(gitBranches))
	for _, gb := range gitBranches {
		branchMap[gb.Name] = gb.Head
	}
	if err := r.gitStore.PutBranches(ctx, branchMap); err != nil {
		sklog.Errorf("Error calling PutBranches on GitStore: %s", err)
		return
	}
}

// iterateLongCommit returns batches of commits from the checkout starting with 'startTime'.
func (r *RepoWatcher) iterateLongCommits(batchSize int, startTime time.Time) (<-chan []*vcsinfo.LongCommit, error) {
	retCh := make(chan []*vcsinfo.LongCommit, 10)
	indexCommits := r.gitInfo.Range(startTime, time.Now().Add(24*time.Hour))
	sklog.Infof("Index commits: %d", len(indexCommits))

	go func() {
		ctx := context.TODO()
		longCommits := make([]*vcsinfo.LongCommit, 0, batchSize)
		for idx, indexCommit := range indexCommits {
			commitDetails, err := r.gitInfo.Details(ctx, indexCommit.Hash, false)
			if err != nil {
				sklog.Errorf("Error fetching commits: %s", err)
				continue
			}
			longCommits = append(longCommits, commitDetails)
			if len(longCommits) >= batchSize || idx == (len(indexCommits)-1) {
				retCh <- longCommits
				longCommits = make([]*vcsinfo.LongCommit, 0, batchSize)
			}
		}
		close(retCh)
	}()
	return retCh, nil
}
