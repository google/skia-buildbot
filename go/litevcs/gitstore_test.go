package litevcs

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.skia.org/infra/go/timer"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	repoURL          = "https://skia.googlesource.com/skia.git"
	repoDir          = "./skia"
	concurrentWrites = 1000
)

func TestGitStore(t *testing.T) {
	ctx := context.TODO()
	conf := &BTConfig{
		ProjectID:  "skia-public",
		InstanceID: "staging",
		TableID:    "test-git-repos",
		Shards:     32,
	}
	assert.NoError(t, bt.DeleteTables(conf.ProjectID, conf.InstanceID, conf.TableID))
	assert.NoError(t, bt.InitBigtable(conf.ProjectID, conf.InstanceID, conf.TableID, []string{cfCommit}))

	gitStore, err := NewBTGitStore(conf)
	assert.NoError(t, err)

	// Get the commits of ~20 years.
	timeDelta := time.Hour * 24 * 365 * 20

	// timeDelta := time.Hour * 24 * 7
	tLoad := timer.New("Loading commits")
	indexCommits, longCommits := loadGitRepo(t, repoURL, repoDir, gitStore, timeDelta)
	tLoad.Stop()

	tQuery := timer.New(fmt.Sprintf("RangeN %d commits", len(indexCommits)))
	foundIndexCommits, err := gitStore.RangeN(ctx, indexCommits[0].Index, indexCommits[0].Index+len(indexCommits))
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

type commitInfo struct {
	commits []*vcsinfo.LongCommit
	indices []int
}

func loadGitRepo(t *testing.T, repoURL, repoDir string, gitStore GitStore, timeDelta time.Duration) ([]*vcsinfo.IndexCommit, []*vcsinfo.LongCommit) {
	ctx := context.TODO()
	commitCh := make(chan *commitInfo)
	indexCommits, err := iterateCommits(repoDir, repoURL, concurrentWrites, commitCh, timeDelta)
	longCommits := make([]*vcsinfo.LongCommit, 0, len(indexCommits))
	assert.NoError(t, err)

	for ci := range commitCh {
		longCommits = append(longCommits, ci.commits...)
		assert.NoError(t, gitStore.Put(ctx, ci.commits, ci.indices))
	}
	// gitStore.(*btGitStore).AllRange(ctx)
	sklog.Infof("Loaded")
	return indexCommits, longCommits
}

func iterateCommits(repoDir, repoURL string, maxCount int, targetCh chan<- *commitInfo, timeDelta time.Duration) ([]*vcsinfo.IndexCommit, error) {
	// repo, err := gitingo.
	var vcs vcsinfo.VCS
	var err error
	vcs, err = gitinfo.NewGitInfo(context.TODO(), repoDir, false, true)
	if err != nil {
		return nil, err
	}

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
	return indexCommits, nil
}
