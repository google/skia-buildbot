package testutils

import (
	"context"
	"fmt"
	"sort"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	batchSize = 1000
)

var (
	btConf = &bt_gitstore.BTConfig{
		ProjectID:  "skia-public",
		InstanceID: "staging",
		TableID:    "test-git-repos",
	}
)

// SetupAndLoadBTGitStore loads the Git repo at repoUrl into the Gitstore.
func SetupAndLoadBTGitStore(t sktest.TestingT, ctx context.Context, workdir, repoURL string, load bool) ([]*vcsinfo.IndexCommit, []*vcsinfo.LongCommit, *bt_gitstore.BigTableGitStore) {
	if load {
		// Delete the tables.
		assert.NoError(t, bt.DeleteTables(btConf.ProjectID, btConf.InstanceID, btConf.TableID))
		assert.NoError(t, bt_gitstore.InitBT(btConf))
	}

	// Get a new gitstore.
	gitStore, err := bt_gitstore.New(ctx, btConf, repoURL)
	assert.NoError(t, err)

	// Get all commits and load them into the GitStore.
	tLoad := timer.New("Loading all commits")
	graph, err := repograph.NewLocalGraph(ctx, repoURL, workdir)
	assert.NoError(t, err)
	assert.NoError(t, graph.Update(ctx))
	graph.UpdateBranchInfo()
	indexCommits, longCommits := loadGitRepo(t, ctx, graph, gitStore, load)
	tLoad.Stop()

	return indexCommits, longCommits, gitStore
}

func loadGitRepo(t sktest.TestingT, ctx context.Context, graph *repograph.Graph, gitStore gitstore.GitStore, load bool) ([]*vcsinfo.IndexCommit, []*vcsinfo.LongCommit) {
	branchList := graph.BranchHeads()
	branches := make(map[string]string, len(branchList))
	for _, branch := range branchList {
		branches[branch.Name] = branch.Head
	}
	commitsMap := graph.GetAll()
	commits := make([]*repograph.Commit, 0, len(commitsMap))
	for _, c := range commitsMap {
		commits = append(commits, c)
	}
	sort.Sort(repograph.CommitSlice(commits))
	indexCommits := make([]*vcsinfo.IndexCommit, 0, len(commits))
	longCommits := make([]*vcsinfo.LongCommit, 0, len(commits))
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]
		indexCommits = append(indexCommits, &vcsinfo.IndexCommit{
			Hash:      c.Hash,
			Index:     len(indexCommits),
			Timestamp: c.Timestamp,
		})
		longCommits = append(longCommits, c.LongCommit)
	}

	if load && len(longCommits) > 0 {
		// Add the commits.
		assert.NoError(t, util.ChunkIter(len(longCommits), batchSize, func(start, end int) error {
			putT := timer.New(fmt.Sprintf("Put %d commits.", end-start))
			defer putT.Stop()
			return gitStore.Put(ctx, longCommits[start:end])
		}))
	}

	for name, head := range branches {
		details, err := gitStore.Get(ctx, []string{head})
		assert.NoError(t, err)
		if details[0] == nil {
			delete(branches, name)
		}
	}

	if load && len(branches) > 0 {
		assert.NoError(t, gitStore.PutBranches(ctx, branches))
	}
	return indexCommits, longCommits
}
