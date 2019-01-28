package main

import (
	"context"
	"flag"
	"time"

	"github.com/davecgh/go-spew/spew"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/litevcs"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	concurrentCommits = 1000
	concurrentWrites  = 1000
)

var (
	instanceID = flag.String("instance", "git-bt", "Repo url")
	projectID  = flag.String("project", "skia-public", "Repo url")
	repoURL    = flag.String("repo_url", "https://chromium.googlesource.com/chromium/src.git", "Repo url")
	repoDir    = flag.String("repo_dir", "/home/stephana/dev/chromium/src", "Repository with the target dir")
	skipLoad   = flag.Bool("skipload", false, "Skip the load step")
	tableID    = flag.String("table", "git-repos", "Repo url")
)

type commitInfo struct {
	commits []*vcsinfo.LongCommit
	indices []int
}

func main() {
	common.Init()

	config := &litevcs.BTConfig{
		ProjectID:  *projectID,
		InstanceID: *instanceID,
		TableID:    *tableID,
		Shards:     32,
	}

	if err := litevcs.InitBT(config); err != nil {
		sklog.Fatalf("Error initializing BT: %s", err)
	}

	gitStore, err := litevcs.NewBTGitStore(config)
	if err != nil {
		sklog.Fatalf("Error creating bt git: %s", err)
	}

	if !*skipLoad {
		loadGitRepo(*repoURL, *repoDir, gitStore)
	}

	ctx := context.TODO()
	now := time.Now()
	indexCommits, err := gitStore.RangeByTime(ctx, now.Add(-time.Hour*24*365*20), now)
	if err != nil {
		sklog.Fatalf("Error reading index commits: %s", err)
	}
	sklog.Infof("Read %d index commits", len(indexCommits))
}

func loadGitRepo(repoURL, repoDir string, gitStore litevcs.GitStore) {
	ctx := context.TODO()
	commitCh := make(chan *commitInfo)
	indexCommits, err := iterateCommits(repoDir, repoURL, concurrentWrites, commitCh)
	if err != nil {
		sklog.Fatalf("Error iterating repo: %s", err)
	}

	for ci := range commitCh {
		sklog.Infof("Loading %d commits", len(ci.commits))
		if err := gitStore.Put(ctx, ci.commits, ci.indices); err != nil {
			sklog.Fatalf("Error writing to gitstore: %s", err)
		}
		sklog.Infof("Done loading %d commits", len(ci.commits))
	}

	sklog.Infof("Last commit: %s", spew.Sdump(indexCommits[len(indexCommits)-1]))
}

func iterateCommits(repoDir, repoURL string, maxCount int, targetCh chan<- *commitInfo) ([]*vcsinfo.IndexCommit, error) {
	// repo, err := gitingo.
	var vcs vcsinfo.VCS
	var err error
	vcs, err = gitinfo.NewGitInfo(context.TODO(), repoDir, true, true)
	if err != nil {
		return nil, err
	}

	// Get all commits of the last ~20 years
	start := time.Now().Add(-time.Hour * 24 * 365 * 20)
	indexCommits := vcs.Range(start, time.Now())

	sklog.Infof("Found %d commits", len(indexCommits))

	go func() {
		ctx := context.TODO()
		longCommits := make([]*vcsinfo.LongCommit, 0, maxCount)
		indices := make([]int, 0, maxCount)
		retIdx := 0
		for idx, indexCommit := range indexCommits {
			commitDetails, err := vcs.Details(ctx, indexCommit.Hash, false)
			if err != nil {
				sklog.Fatalf("Error fetching commits: %s", err)
			}
			longCommits = append(longCommits, commitDetails)
			indices = append(indices, indexCommit.Index)
			// sklog.Infof("Fetched %d commits", len(longCommits))
			if len(longCommits) >= maxCount || idx == (len(indexCommits)-1) {
				targetCh <- &commitInfo{
					commits: longCommits,
					indices: indices,
				}
				longCommits = make([]*vcsinfo.LongCommit, 0, maxCount)
				indices = make([]int, 0, maxCount)
				retIdx = 0
			} else {
				retIdx++
			}
		}
		close(targetCh)
	}()
	return indexCommits, nil
}
