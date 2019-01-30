package litevcs

import (
	"context"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vcsinfo"
	vcstu "go.skia.org/infra/go/vcsinfo/testutils"
)

const (
	skiaRepoURL  = "https://skia.googlesource.com/skia.git"
	skiaRepoDir  = "./skia"
	localRepoURL = "https://example.com/local.git"
	// concurrentWrites = 1000
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

func setupAndLoadRepo(t *testing.T, repoURL, repoDir string, loadNew bool) (vcsinfo.VCS, func()) {
	repoDir, cleanup := vcstu.InitTempRepo()
	_, _, gitStore := setupAndLoadGitStore(t, repoURL, repoDir, true)
	vcs, err := NewVCS(gitStore, "master", nil)
	assert.NoError(t, err)
	return vcs, cleanup
}

func setupAndLoadGitStore(t *testing.T, repoURL, repoDir string, load bool) ([]*vcsinfo.IndexCommit, []*vcsinfo.LongCommit, GitStore) {
	if load {
		// Delete the tables.
		assert.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
		assert.NoError(t, InitBT(btConf))
	}
	sklog.Infof("Deleted old tables if requested.")

	// Get a new gitstore.
	gitStore, err := NewBTGitStore(btConf, repoURL)
	assert.NoError(t, err)

	// Get the commits of ~20 years.
	// timeDelta := time.Hour * 24 * 365 * 20

	timeDelta := time.Hour * 24 * 10
	// timeDelta := time.Hour * 24
	tLoad := timer.New("Loading commits")
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
	indexCommits := iterateCommits(t, repoDir, concurrentWrites, commitCh, timeDelta)
	longCommits := make([]*vcsinfo.LongCommit, 0, len(indexCommits))

	// Keep track of the branches.
	branches := map[string]string{}
	for ci := range commitCh {
		// Update the branch info.
		for _, oneCommit := range ci.commits {
			for foundBranch := range oneCommit.Branches {
				branches[foundBranch] = oneCommit.Hash
			}
		}

		longCommits = append(longCommits, ci.commits...)
		if load {
			sklog.Infof("Loading batch of %d", len(ci.commits))

			// Add the commits.
			assert.NoError(t, gitStore.Put(ctx, ci.commits))
			sklog.Infof("Done put of %d", len(ci.commits))

			// Update the current branches.
			gitBranches := make([]*git.Branch, 0, len(branches))
			for name, head := range branches {
				gitBranches = append(gitBranches, &git.Branch{Name: name, Head: head})
			}
			assert.NoError(t, gitStore.PutBranches(ctx, gitBranches))
			sklog.Infof("Done put branches %d", len(ci.commits))

			// Retrieve the branches and make sure they match.
			foundBranches, err := gitStore.GetBranches(ctx)
			assert.NoError(t, err)
			for name, head := range branches {
				pointer, ok := foundBranches[name]
				assert.True(t, ok)
				assert.Equal(t, head, pointer.Head)
			}
			sklog.Infof("Done loading batch of %d", len(ci.commits))
		}
	}

	sklog.Infof("Found branches: %v", branches)
	if load {
		gitBranches := make([]*git.Branch, 0, len(branches))
		for name, head := range branches {
			gitBranches = append(gitBranches, &git.Branch{Name: name, Head: head})
		}
		assert.NoError(t, gitStore.PutBranches(ctx, gitBranches))
	}

	// gitStore.(*btGitStore).AllRange(ctx)
	sklog.Infof("Loaded.")
	return indexCommits, longCommits
}

func iterateCommits(t *testing.T, repoDir string, maxCount int, targetCh chan<- *commitInfo, timeDelta time.Duration) []*vcsinfo.IndexCommit {
	var vcs vcsinfo.VCS
	var err error
	vcs, err = gitinfo.NewGitInfo(context.TODO(), repoDir, false, true)
	assert.NoError(t, err)

	start := time.Now().Add(-timeDelta)
	indexCommits := vcs.Range(start, time.Now())
	sklog.Infof("Index commits: %d", len(indexCommits))

	go func() {
		ctx := context.TODO()
		longCommits := make([]*vcsinfo.LongCommit, 0, maxCount)
		indices := make([]int, 0, maxCount)
		retIdx := 0
		for idx, indexCommit := range indexCommits {
			commitDetails, err := vcs.Details(ctx, indexCommit.Hash, true)
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
