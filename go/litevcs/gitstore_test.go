package litevcs

import (
	"context"
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vcsinfo"
)

func TestGitStore(t *testing.T) {
	testutils.LargeTest(t)
	// t.Skip()
	ctx := context.TODO()
	_, longCommits, gitStore := setupAndLoadGitStore(t, repoURL, repoDir, true)

	// Enumerate the index commits
	indexCommits := make([]*vcsinfo.IndexCommit, len(longCommits))
	for idx, commit := range longCommits {
		indexCommits[idx] = &vcsinfo.IndexCommit{Index: idx, Hash: commit.Hash, Timestamp: commit.Timestamp}
	}

	tQuery := timer.New(fmt.Sprintf("RangeN %d commits", len(indexCommits)))
	foundIndexCommits, err := gitStore.RangeN(ctx, 0, len(longCommits), "")
	assert.NoError(t, err)
	tQuery.Stop()
	assert.Equal(t, len(indexCommits), len(longCommits))

	hashes := make([]string, 0, len(indexCommits))
	assert.Equal(t, len(indexCommits), len(foundIndexCommits))
	for idx, expected := range indexCommits {
		assert.Equal(t, expected, foundIndexCommits[idx])
		hashes = append(hashes, expected.Hash)
	}

	tLongCommits := timer.New("Get LongCommits")
	foundLongCommits, foundIndices, err := gitStore.Get(ctx, hashes)
	assert.NoError(t, err)
	tLongCommits.Stop()

	assert.Equal(t, len(longCommits), len(foundLongCommits))
	for idx, expected := range longCommits {
		if foundLongCommits[idx].Branches == nil {
			foundLongCommits[idx].Branches = map[string]bool{}
		}
		assert.Equal(t, expected, foundLongCommits[idx])
		assert.Equal(t, indexCommits[idx].Index, foundIndices[idx])
		// sklog.Infof("ExpFound:  %d   %d    %s     %s    %s", foundIndices[idx], indexCommits[idx].Index, indexCommits[idx].Hash, expected.Hash, foundLongCommits[idx].Hash)
	}
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
