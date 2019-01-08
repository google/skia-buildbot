package litevcs

import (
	"context"
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
	repoURL          = "https://skia.googlesource.com/skia.git"
	repoDir          = "./skia"
	concurrentWrites = 1000
)

var (
	btConf = &BTConfig{
		ProjectID:  "skia-public",
		InstanceID: "staging",
		TableID:    "test-git-repos",
		Shards:     32,
	}
)

func setupAndLoadRepo(t *testing.T) (vcsinfo.VCS, func()) {
	repoDir, cleanup := vcstu.InitTempRepo()
	_, _, gitStore := setupAndLoadGitStore(t, repoDir)
	vcs, err := NewVCS(gitStore, nil)
	assert.NoError(t, err)
	return vcs, cleanup
}

func setupAndLoadGitStore(t *testing.T, repoDir string) ([]*vcsinfo.IndexCommit, []*vcsinfo.LongCommit, GitStore) {
	// Delete the tables.
	assert.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
	assert.NoError(t, bt.InitBigtable(btConf.ProjectID, btConf.InstanceID, btConf.TableID, []string{cfCommit}))

	// Get a new gitstore.
	gitStore, err := NewBTGitStore(btConf)
	assert.NoError(t, err)

	// Get the commits of ~20 years.
	timeDelta := time.Hour * 24 * 365 * 20

	// timeDelta := time.Hour * 24 * 7
	tLoad := timer.New("Loading commits")
	indexCommits, longCommits := loadGitRepo(t, repoDir, gitStore, timeDelta)
	tLoad.Stop()

	return indexCommits, longCommits, gitStore
}

type commitInfo struct {
	commits []*vcsinfo.LongCommit
	indices []int
}

func loadGitRepo(t *testing.T, repoDir string, gitStore GitStore, timeDelta time.Duration) ([]*vcsinfo.IndexCommit, []*vcsinfo.LongCommit) {
	sklog.Infof("XXX: %s", repoDir)
	ctx := context.TODO()
	commitCh := make(chan *commitInfo)
	indexCommits := iterateCommits(t, repoDir, concurrentWrites, commitCh, timeDelta)
	longCommits := make([]*vcsinfo.LongCommit, 0, len(indexCommits))

	for ci := range commitCh {
		longCommits = append(longCommits, ci.commits...)
		assert.NoError(t, gitStore.Put(ctx, ci.commits, ci.indices))
	}
	// gitStore.(*btGitStore).AllRange(ctx)
	sklog.Infof("Loaded")
	return indexCommits, longCommits
}

func iterateCommits(t *testing.T, repoDir string, maxCount int, targetCh chan<- *commitInfo, timeDelta time.Duration) []*vcsinfo.IndexCommit {
	var vcs vcsinfo.VCS
	var err error
	vcs, err = gitinfo.NewGitInfo(context.TODO(), repoDir, false, true)
	assert.NoError(t, err)

	// Get all commits of the last years
	start := time.Now().Add(-timeDelta)
	indexCommits := vcs.Range(start, time.Now())

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
	return indexCommits
}
