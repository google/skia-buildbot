package litevcs

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

	// Get a new gitstore.
	gitStore, err := NewBTGitStore(btConf, repoURL)
	assert.NoError(t, err)

	// Get the commits of ~20 years.
	// timeDelta := time.Hour * 24 * 365 * 20

	// timeDelta := time.Hour * 24 * 365
	timeDelta := time.Hour * 24
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

	// Keep track of the branches.
	// branches := map[string]string{}
	for ci := range commitCh {
		// Update the branch info.
		// for _, oneCommit := range ci.commits {
		// 	for bn := range oneCommit.Branches {
		// 		branches[bn] = oneCommit.Hash
		// 	}
		// 	hashes = append(hashes, oneCommit.Hash)
		// }
		assert.True(t, len(ci.commits) > 0)
		longCommits = append(longCommits, ci.commits...)
		if load {
			sklog.Infof("Loading batch of %d", len(ci.commits))

			// Add the commits.
			putT := timer.New(fmt.Sprintf("Put %d commits.", len(ci.commits)))
			assert.NoError(t, gitStore.Put(ctx, ci.commits))
			putT.Stop()

			// if slowTrack {
			// 	// Make sure the add was successful.
			// 	getT := timer.New(fmt.Sprintf("Get %d commits.", len(hashes)))
			// 	foundLongCommits, foundIndices, err := gitStore.Get(ctx, hashes)
			// 	assert.NoError(t, err)
			// 	getT.Stop()
			// 	assert.Equal(t, len(foundLongCommits), len(foundIndices))
			// 	for idx, foundIndex := range foundIndices {
			// 		foundLongCommits[idx].Branches = longCommits[idx].Branches
			// 		assert.Equal(t, longCommits[idx], foundLongCommits[idx])
			// 		assert.Equal(t, idx, foundIndex)
			// 	}

			// 	// Update the current branches.
			// 	// putBranchesT := timer.New(fmt.Sprintf("PutBranches %d branches.", len(branches)))
			// 	// assert.NoError(t, gitStore.PutBranches(ctx, branches))
			// 	// putBranchesT.Stop()
			// 	// sklog.Infof("Done put branches %d", len(ci.commits))

			// 	// // Retrieve the branches and make sure they match.
			// 	// getBranchesT := timer.New(fmt.Sprintf("GetBranches %d branches.", len(branches)))
			// 	// foundBranches, err := gitStore.GetBranches(ctx)
			// 	// assert.NoError(t, err)
			// 	// getBranchesT.Stop()

			// 	// for name, head := range branches {
			// 	// 	pointer, ok := foundBranches[name]
			// 	// 	assert.True(t, ok)
			// 	// 	assert.Equal(t, head, pointer.Head)
			// 	// }
			// }
			sklog.Infof("Done loading batch of %d", len(ci.commits))
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

	// gitStore.(*btGitStore).AllRange(ctx)
	sklog.Infof("Loaded.")
	return indexCommits, longCommits
}

func iterateCommits(t *testing.T, repoDir string, maxCount int, targetCh chan<- *commitInfo, timeDelta time.Duration) ([]*vcsinfo.IndexCommit, map[string]string) {
	vcs, err := gitinfo.NewGitInfo(context.TODO(), repoDir, false, true)
	assert.NoError(t, err)

	start := time.Now().Add(-timeDelta)
	indexCommits := vcs.Range(start, time.Now().Add(time.Hour))
	sklog.Infof("Index commits: %d", len(indexCommits))

	gitBranches, err := vcs.GetBranches(context.TODO())
	assert.NoError(t, err)

	branches := map[string]string{}
	for _, gb := range gitBranches {
		// branches[gitinfo.TrimBranchPrefix(gb.Name)] = gb.Head
		branches[gb.Name] = gb.Head
		sklog.Infof("Branch: %40s : %s", gb.Name, gb.Head)
	}

	go func() {
		ctx := context.TODO()
		longCommits := make([]*vcsinfo.LongCommit, 0, maxCount)
		indices := make([]int, 0, maxCount)
		retIdx := 0
		batchTimer := timer.New("Fetching commits starting with 0")
		for idx, indexCommit := range indexCommits {
			commitDetails, err := vcs.Details(ctx, indexCommit.Hash, false)
			if err != nil {
				sklog.Fatalf("Error fetching commits: %s", err)
			}
			longCommits = append(longCommits, commitDetails)
			indices = append(indices, indexCommit.Index)
			// sklog.Infof("Fetched %d commits", len(longCommits))
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
