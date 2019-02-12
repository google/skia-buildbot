package litevcs

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vcsinfo"
	vcs_testutils "go.skia.org/infra/go/vcsinfo/testutils"
)

func TestLargeGitStore(t *testing.T) {
	testutils.LargeTest(t)

	// Comment for local testing against the Skia repository. Which is assumed to at ./skia.
	t.Skip()
	testGitStore(t, skiaRepoURL, skiaRepoDir, true)
}

func TestGitStoreLocalRepo(t *testing.T) {
	testutils.LargeTest(t)

	repoDir, cleanup := vcs_testutils.InitTempRepo()
	defer cleanup()
	testGitStore(t, localRepoURL, repoDir, true)
}

func testGitStore(t *testing.T, repoURL, repoDir string, freshLoad bool) {
	// Get all commits that have been added to the gitstore.
	_, longCommits, gitStore := setupAndLoadGitStore(t, repoURL, repoDir, freshLoad)

	// Sort long commits they way they are sorted by BigTable (by timestamp/hash)
	sort.Slice(longCommits, func(i, j int) bool {
		tsI := longCommits[i].Timestamp.UnixNano() / int64(time.Second)
		tsJ := longCommits[j].Timestamp.UnixNano() / int64(time.Second)
		return (tsI < tsJ) || ((tsI == tsJ) && (longCommits[i].Hash < longCommits[j].Hash))
	})

	// Make sure th
	indexCommits := make([]*vcsinfo.IndexCommit, len(longCommits))
	for idx, commit := range longCommits {
		indexCommits[idx] = &vcsinfo.IndexCommit{
			Index:     idx,
			Hash:      commit.Hash,
			Timestamp: commit.Timestamp,
		}
	}

	// Find all the commits in the repository independent of branches.
	foundIndexCommits, foundLongCommits := getFromRange(t, gitStore, 0, len(longCommits), "")
	assert.Equal(t, len(indexCommits), len(foundIndexCommits))
	assert.Equal(t, len(longCommits), len(foundLongCommits))

	for idx, expected := range longCommits {
		// REMOVE THIS and make sure we get the same results from GitStore
		foundLongCommits[idx].Branches = expected.Branches

		// if foundLongCommits[idx].Branches == nil {
		// 	foundLongCommits[idx].Branches = map[string]bool{}
		// }
		assert.Equal(t, expected, foundLongCommits[idx])
		// sklog.Infof("ExpFound:  %d   %d    %s     %s    %s", foundIndices[idx], indexCommits[idx].Index, indexCommits[idx].Hash, expected.Hash, foundLongCommits[idx].Hash)
	}

	branchNames, branchCommits := getBranchCommits(t, repoDir)

	// Get the branch info, so we can check it.
	// foundBranches, err := gitStore.GetBranches(ctx)
	// assert.NoError(t, err)

	// byBranchCommits := getByBranch(longCommits)
	for branchIdx, branchName := range branchNames {
		// if branchName != "master" {
		// 	continue
		// }

		expHashes := branchCommits[branchIdx]
		foundIndexCommits, foundLongCommits := getFromRange(t, gitStore, 0, len(longCommits), branchName)
		assert.Equal(t, len(expHashes), len(foundIndexCommits))
		assert.Equal(t, len(expHashes), len(foundLongCommits))
		expIdx := len(expHashes) - 1
		for idx := len(foundIndexCommits) - 1; idx >= 0; idx-- {
			expHash := expHashes[expIdx]
			if expHash != foundIndexCommits[idx].Hash {
				for delta := -5; delta < 6; delta++ {
					sklog.Infof("FUTURE: %d       %s       %s", idx-delta, expHashes[expIdx-delta], foundIndexCommits[idx-delta].Hash)
				}
			}

			assert.Equal(t, foundIndexCommits[idx].Hash, foundLongCommits[idx].Hash)
			assert.Equal(t, expHash, foundIndexCommits[idx].Hash)
			expIdx--
		}
		sklog.Infof("Branch %s verified. %d commits.", branchName, len(longCommits))

		// // Get the portion of expected
		// targetIdx := 0
		// for ; targetIdx < len(expHashes); targetIdx++ {
		// 	if expHashes[targetIdx] == foundLongCommits[0].Hash {
		// 		break
		// 	}
		// }
		// if targetIdx == len(expHashes) {
		// 	continue
		// }
		// sklog.Infof("TargetIdx: %d  %d", targetIdx, len(expHashes))
		// assert.False(t, targetIdx >= len(expHashes))
		// expHashes = expHashes[targetIdx:]

		// // if len(expHashes) != len(foundLongCommits) {
		// // 	for _, hash := range expHashes {
		// // 		sklog.Infof("HASH: %s", hash)
		// // 	}
		// // 	sklog.Infof("---------------------------------------------")
		// // 	for _, commit := range foundLongCommits {
		// // 		sklog.Infof("LONGC: %s ", commit.Hash)
		// // 	}
		// // }

		// assert.Equal(t, len(expHashes), len(foundLongCommits))
		// assert.Equal(t, len(expHashes), len(foundIndexCommits))
		// for idx, expectedHash := range expHashes {
		// 	assert.Equal(t, expectedHash, foundLongCommits[idx].Hash)
		// 	expIndexCommit := &vcsinfo.IndexCommit{
		// 		Index:     idx,
		// 		Hash:      expectedHash,
		// 		Timestamp: foundLongCommits[idx].Timestamp,
		// 	}
		// 	assert.Equal(t, expIndexCommit, foundIndexCommits[idx])
		// 	// sklog.Infof("ExpFound:  %d   %d    %s     %s    %s", foundIndices[idx], indexCommits[idx].Index, indexCommits[idx].Hash, expected.Hash, foundLongCommits[idx].Hash)
		// }
	}
}

func getByBranch(longCommits []*vcsinfo.LongCommit) map[string][2]interface{} {
	longCommitsMap := map[string][]*vcsinfo.LongCommit{}
	for _, commit := range longCommits {
		for branchName := range commit.Branches {
			longCommitsMap[branchName] = append(longCommitsMap[branchName], commit)
		}
	}

	ret := map[string][2]interface{}{}
	for name, commits := range longCommitsMap {
		sklog.Infof("BRAAANCH1: %s   %d", name, len(commits))
		indexCommits := make([]*vcsinfo.IndexCommit, len(commits))
		for idx, c := range commits {
			indexCommits[idx] = &vcsinfo.IndexCommit{Index: idx, Hash: c.Hash, Timestamp: c.Timestamp}
		}
		ret[name] = [2]interface{}{indexCommits, commits}
		sklog.Infof("BRAAANCH2: %s   %d/%d", name, len(indexCommits), len(commits))
	}
	return ret
}

func getBranchCommits(t *testing.T, repoDir string) ([]string, [][]string) {
	ctx := context.TODO()
	vcs, err := gitinfo.NewGitInfo(ctx, repoDir, false, true)
	assert.NoError(t, err)

	branches, err := vcs.GetBranches(ctx)
	assert.NoError(t, err)

	branchNames := make([]string, 0, len(branches))
	branchCommits := make([][]string, 0, len(branches))
	for _, branch := range branches {
		// if strings.Contains(branch.Name, "m62") {
		// 	continue
		// }
		branchNames = append(branchNames, branch.Name)
		indexCommits, err := gitinfo.GetBranchCommits(ctx, repoDir, branch.Name)
		assert.NoError(t, err)
		commitHashes := make([]string, len(indexCommits))
		for idx, idxCommit := range indexCommits {
			commitHashes[idx] = idxCommit.Hash
		}
		branchCommits = append(branchCommits, commitHashes)
	}

	return branchNames, branchCommits
}

func getFromRange(t *testing.T, gitStore GitStore, startIdx, endIdx int, branchName string) ([]*vcsinfo.IndexCommit, []*vcsinfo.LongCommit) {
	ctx := context.TODO()

	tQuery := timer.New(fmt.Sprintf("RangeN %d - %d commits from branch %q", startIdx, endIdx, branchName))
	foundIndexCommits, err := gitStore.RangeN(ctx, startIdx, endIdx, branchName)
	assert.NoError(t, err)
	tQuery.Stop()

	// for idx, commit := range foundIndexCommits {
	// 	sklog.Infof("CCC: %d              %s    -    %d", idx, commit.Hash, commit.Index)
	// }

	hashes := make([]string, 0, len(foundIndexCommits))
	for _, commit := range foundIndexCommits {
		hashes = append(hashes, commit.Hash)
	}

	tLongCommits := timer.New(fmt.Sprintf("Get %d LongCommits from branch %q", len(hashes), branchName))
	foundLongCommits, err := gitStore.Get(ctx, hashes)
	assert.NoError(t, err)
	assert.Equal(t, len(foundIndexCommits), len(foundLongCommits))
	tLongCommits.Stop()

	return foundIndexCommits, foundLongCommits
}

// func TestGitStore(t *testing.T) {
// 	ctx := context.TODO()
// 	assert.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
// 	assert.NoError(t, bt.InitBigtable(btConf.ProjectID, btConf.InstanceID, btConf.TableID, []string{cfCommit}))

// 	gitStore, err := NewBTGitStore(btConf)
// 	assert.NoError(t, err)

// 	// Get the commits of ~20 years.
// 	timeDelta := time.Hour * 24 * 365 * 20

// 	// timeDelta := time.Hour * 24 * 7
// 	tLoad := timer.New("Loading commits")
// 	indexCommits, longCommits := loadGitRepo(t, repoURL, repoDir, gitStore, timeDelta)
// 	tLoad.Stop()

// 	tQuery := timer.New(fmt.Sprintf("RangeN %d commits", len(indexCommits)))
// 	foundIndexCommits, err := gitStore.RangeN(ctx, indexCommits[0].Index, indexCommits[0].Index+len(indexCommits))
// 	assert.NoError(t, err)
// 	tQuery.Stop()
// 	assert.Equal(t, len(indexCommits), len(longCommits))

// 	hashes := make([]string, 0, len(indexCommits))
// 	assert.Equal(t, len(indexCommits), len(foundIndexCommits))
// 	for idx, expected := range indexCommits {
// 		assert.Equal(t, expected, foundIndexCommits[idx])
// 		hashes = append(hashes, expected.Hash)
// 	}

// 	tLongCommits := timer.New("Get LongCommits")
// 	foundLongCommits, foundIndices, err := gitStore.Get(ctx, hashes)
// 	assert.NoError(t, err)
// 	tLongCommits.Stop()

// 	assert.Equal(t, len(longCommits), len(foundLongCommits))
// 	for idx, expected := range longCommits {
// 		if foundLongCommits[idx].Branches == nil {
// 			foundLongCommits[idx].Branches = map[string]bool{}
// 		}
// 		assert.Equal(t, expected, foundLongCommits[idx])
// 		assert.Equal(t, indexCommits[idx].Index, foundIndices[idx])
// 		// sklog.Infof("ExpFound:  %d   %d    %s     %s    %s", foundIndices[idx], indexCommits[idx].Index, indexCommits[idx].Hash, expected.Hash, foundLongCommits[idx].Hash)
// 	}
// }
