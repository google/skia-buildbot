package main

import (
	"context"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"

	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/litevcs"
	"go.skia.org/infra/go/skerr"
)

const (
	// batchSize is the size of a batch of commits that is imported into BTGit.
	batchSize = 10000
)

type RepoWatcher struct {
	gitStore litevcs.GitStore
	gitInfo  *gitinfo.GitInfo
}

func NewRepoWatcher(conf *litevcs.BTConfig, repoURL, repoDir string) (*RepoWatcher, error) {
	gitStore, err := litevcs.NewBTGitStore(conf, repoURL)
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
	}, nil
}

func (r *RepoWatcher) Start(ctx context.Context) {
	util.RepeatCtx(10*time.Second, ctx, r.updateFn)
}

func (r *RepoWatcher) updateFn() {
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

	startTime := litevcs.MinTime
	allCommits, ok := branches[""]
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

// gitStore, err := litevcs.NewBTGitStore(config)
// if err != nil {
// 	sklog.Fatalf("Error creating bt git: %s", err)
// }

// if !*skipLoad {
// 	loadGitRepo(*repoURL, *repoDir, gitStore)
// }

// ctx := context.TODO()
// now := time.Now()
// indexCommits, err := gitStore.RangeByTime(ctx, now.Add(-time.Hour*24*365*20), now)
// if err != nil {
// 	sklog.Fatalf("Error reading index commits: %s", err)
// }
// sklog.Infof("Read %d index commits", len(indexCommits))
// }

// func loadGitRepo(repoURL, repoDir string, gitStore litevcs.GitStore) {
// ctx := context.TODO()
// commitCh := make(chan *commitInfo)
// indexCommits, err := iterateCommits(repoDir, repoURL, concurrentWrites, commitCh)
// if err != nil {
// 	sklog.Fatalf("Error iterating repo: %s", err)
// }

// for ci := range commitCh {
// 	sklog.Infof("Loading %d commits", len(ci.commits))
// 	if err := gitStore.Put(ctx, ci.commits, ci.indices); err != nil {
// 		sklog.Fatalf("Error writing to gitstore: %s", err)
// 	}
// 	sklog.Infof("Done loading %d commits", len(ci.commits))
// }

// sklog.Infof("Last commit: %s", spew.Sdump(indexCommits[len(indexCommits)-1]))
// }

// func iterateCommits(repoDir, repoURL string, maxCount int, targetCh chan<- *commitInfo) ([]*vcsinfo.IndexCommit, error) {
// // repo, err := gitingo.
// var vcs vcsinfo.VCS
// var err error
// vcs, err = gitinfo.NewGitInfo(context.TODO(), repoDir, true, true)
// if err != nil {
// 	return nil, err
// }

// // Get all commits of the last ~20 years
// start := time.Now().Add(-time.Hour * 24 * 365 * 20)
// indexCommits := vcs.Range(start, time.Now())

// sklog.Infof("Found %d commits", len(indexCommits))

// go func() {
// 	ctx := context.TODO()
// 	longCommits := make([]*vcsinfo.LongCommit, 0, maxCount)
// 	indices := make([]int, 0, maxCount)
// 	retIdx := 0
// 	for idx, indexCommit := range indexCommits {
// 		commitDetails, err := vcs.Details(ctx, indexCommit.Hash, false)
// 		if err != nil {
// 			sklog.Fatalf("Error fetching commits: %s", err)
// 		}
// 		longCommits = append(longCommits, commitDetails)
// 		indices = append(indices, indexCommit.Index)
// 		// sklog.Infof("Fetched %d commits", len(longCommits))
// 		if len(longCommits) >= maxCount || idx == (len(indexCommits)-1) {
// 			targetCh <- &commitInfo{
// 				commits: longCommits,
// 				indices: indices,
// 			}
// 			longCommits = make([]*vcsinfo.LongCommit, 0, maxCount)
// 			indices = make([]int, 0, maxCount)
// 			retIdx = 0
// 		} else {
// 			retIdx++
// 		}
// 	}
// 	close(targetCh)
// }()
// return indexCommits, nil
// }
