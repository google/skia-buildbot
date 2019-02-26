package gitstore

import (
	"context"
	"fmt"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vcsinfo"
	vcstu "go.skia.org/infra/go/vcsinfo/testutils"
)

const (
	skiaRepoURL      = "https://skia.googlesource.com/skia.git"
	skiaRepoDir      = "./skia"
	localRepoURL     = "https://example.com/local.git"
	concurrentWrites = 1000
)

var (
	btConf = &BTConfig{
		ProjectID:  "skia-public",
		InstanceID: "staging",
		TableID:    "test-git-repos",
	}
)

// setupVCSLocalRepo loads the test repo into a new GitStore and returns an instance of vcsinfo.VCS.
func setupVCSLocalRepo(t *testing.T) (vcsinfo.VCS, func()) {
	repoDir, cleanup := vcstu.InitTempRepo()
	_, _, gitStore := setupAndLoadGitStore(t, localRepoURL, repoDir, true)
	vcs, err := NewVCS(gitStore, "master", nil)
	assert.NoError(t, err)
	return vcs, cleanup
}

// setupAndLoadGitStore loads the Git repo in repoDir into the Gitstore. It assumes that the
// repo is checked out. repoURL is only used for creating the GitStore.
func setupAndLoadGitStore(t *testing.T, repoURL, repoDir string, load bool) ([]*vcsinfo.IndexCommit, []*vcsinfo.LongCommit, GitStore) {
	if load {
		// Delete the tables.
		assert.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
		assert.NoError(t, InitBT(btConf))
	}

	// Get a new gitstore.
	gitStore, err := NewBTGitStore(context.TODO(), btConf, repoURL)
	assert.NoError(t, err)

	// Get the commits of the last ~20 years and load them into the GitStore
	timeDelta := time.Hour * 24 * 365 * 20
	tLoad := timer.New("Loading all commits")
	indexCommits, longCommits := loadGitRepo(t, repoDir, gitStore, timeDelta, load)
	tLoad.Stop()

	return indexCommits, longCommits, gitStore
}

type commitInfo struct {
	commits []*vcsinfo.LongCommit
	indices []int
}

func loadGitRepo(t *testing.T, repoDir string, gitStore GitStore, timeDelta time.Duration, load bool) ([]*vcsinfo.IndexCommit, []*vcsinfo.LongCommit) {
	ctx := context.TODO()
	commitCh := make(chan *commitInfo, 10)
	indexCommits, branches := iterateCommits(t, repoDir, concurrentWrites, commitCh, timeDelta)
	longCommits := make([]*vcsinfo.LongCommit, 0, len(indexCommits))

	for ci := range commitCh {
		assert.True(t, len(ci.commits) > 0)
		longCommits = append(longCommits, ci.commits...)
		if load {
			// Add the commits.
			putT := timer.New(fmt.Sprintf("Put %d commits.", len(ci.commits)))
			assert.NoError(t, gitStore.Put(ctx, ci.commits))
			putT.Stop()
		}
	}

	for name, head := range branches {
		details, err := gitStore.Get(ctx, []string{head})
		assert.NoError(t, err)
		if details[0] == nil {
			delete(branches, name)
		} else {
			sklog.Infof("Found branches: %40s  :  %s", name, head)
		}
	}

	if load {
		assert.NoError(t, gitStore.PutBranches(ctx, branches))
	}
	return indexCommits, longCommits
}

// iterateCommits returns batches of commits via a channel. It returns all IndexCommits within
// the given timeDelta.
func iterateCommits(t *testing.T, repoDir string, maxCount int, targetCh chan<- *commitInfo, timeDelta time.Duration) ([]*vcsinfo.IndexCommit, map[string]string) {
	gitInfo, err := gitinfo.NewGitInfo(context.TODO(), repoDir, false, true)
	assert.NoError(t, err)

	start := time.Now().Add(-timeDelta)
	indexCommits := gitInfo.Range(start, time.Now().Add(time.Hour))
	sklog.Infof("Index commits: %d", len(indexCommits))

	gitBranches, err := gitInfo.GetBranches(context.TODO())
	assert.NoError(t, err)

	// Keep track of the branches.
	branches := map[string]string{}
	for _, gb := range gitBranches {
		branches[gb.Name] = gb.Head
	}

	go func() {
		ctx := context.TODO()
		longCommits := make([]*vcsinfo.LongCommit, 0, maxCount)
		indices := make([]int, 0, maxCount)
		retIdx := 0
		batchTimer := timer.New("Fetching commits starting with 0")
		for idx, indexCommit := range indexCommits {
			commitDetails, err := gitInfo.Details(ctx, indexCommit.Hash, false)
			if err != nil {
				sklog.Fatalf("Error fetching commits: %s", err)
			}
			longCommits = append(longCommits, commitDetails)
			indices = append(indices, indexCommit.Index)
			if len(longCommits) >= maxCount || idx == (len(indexCommits)-1) {
				batchTimer.Stop()
				targetCh <- &commitInfo{
					commits: longCommits,
					indices: indices,
				}
				longCommits = make([]*vcsinfo.LongCommit, 0, maxCount)
				indices = make([]int, 0, maxCount)
				retIdx = 0
				batchTimer = timer.New(fmt.Sprintf("Fetching commits starting with %d", idx))
			} else {
				retIdx++
			}
		}
		batchTimer.Stop()
		close(targetCh)
	}()
	return indexCommits, branches
}
