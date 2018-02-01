package incremental

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/window"
)

func TestIncrementalCommits(t *testing.T) {
	testutils.MediumTest(t)

	// Setup.
	ctx := context.Background()
	gb := git_testutils.GitInit(t, ctx)
	c0 := gb.CommitGen(ctx, "file1")
	wd, cleanupWd := testutils.TempDir(t)
	defer cleanupWd()
	repo, err := repograph.NewGraph(ctx, gb.Dir(), wd)
	assert.NoError(t, err)
	repos := repograph.Map{
		gb.RepoUrl(): repo,
	}
	N := 100
	w, err := window.New(24*time.Hour, N, repos)
	assert.NoError(t, err)
	cc := newCommitsCache(repos)

	// Initial update. Expect a single branch with one commit.
	branches, commits, err := cc.Update(ctx, w, false, N)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(branches))
	assert.Equal(t, 1, len(branches[gb.RepoUrl()]))
	assert.Equal(t, "master", branches[gb.RepoUrl()][0].Name)
	assert.Equal(t, c0, branches[gb.RepoUrl()][0].Head)
	assert.Equal(t, 1, len(commits))
	assert.Equal(t, 1, len(commits[gb.RepoUrl()]))
	assert.Equal(t, c0, commits[gb.RepoUrl()][0].Hash)

	// Update again, with no new commits. Expect empty response.
	branches, commits, err = cc.Update(ctx, w, false, N)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(branches))
	assert.Equal(t, 0, len(commits))

	// Passing in reset=true should give us ALL commits and branches,
	// regardless of whether they're new.
	branches, commits, err = cc.Update(ctx, w, true, N)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(branches))
	assert.Equal(t, 1, len(branches[gb.RepoUrl()]))
	assert.Equal(t, "master", branches[gb.RepoUrl()][0].Name)
	assert.Equal(t, c0, branches[gb.RepoUrl()][0].Head)
	assert.Equal(t, 1, len(commits))
	assert.Equal(t, 1, len(commits[gb.RepoUrl()]))
	assert.Equal(t, c0, commits[gb.RepoUrl()][0].Hash)

	// Add some new commits.
	c1 := gb.CommitGen(ctx, "file1")
	c2 := gb.CommitGen(ctx, "file1")
	branches, commits, err = cc.Update(ctx, w, false, N)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(branches))
	assert.Equal(t, 1, len(branches[gb.RepoUrl()]))
	assert.Equal(t, "master", branches[gb.RepoUrl()][0].Name)
	assert.Equal(t, c2, branches[gb.RepoUrl()][0].Head)
	assert.Equal(t, 1, len(commits))
	assert.Equal(t, 2, len(commits[gb.RepoUrl()]))
	// Commits could come back in any order.
	if commits[gb.RepoUrl()][0].Hash == c1 {
		assert.Equal(t, c2, commits[gb.RepoUrl()][1].Hash)
	} else {
		assert.Equal(t, c1, commits[gb.RepoUrl()][1].Hash)
		assert.Equal(t, c2, commits[gb.RepoUrl()][0].Hash)
	}

	// Add a new branch, with no commits.
	gb.CreateBranchTrackBranch(ctx, "branch2", "origin/master")
	branches, commits, err = cc.Update(ctx, w, false, N)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(branches))
	assert.Equal(t, 2, len(branches[gb.RepoUrl()]))
	if branches[gb.RepoUrl()][0].Name == "master" {
		assert.Equal(t, "branch2", branches[gb.RepoUrl()][1].Name)
		assert.Equal(t, c2, branches[gb.RepoUrl()][1].Head)
	} else {
		assert.Equal(t, "master", branches[gb.RepoUrl()][1].Name)
		assert.Equal(t, "branch2", branches[gb.RepoUrl()][0].Name)
		assert.Equal(t, c2, branches[gb.RepoUrl()][0].Head)
	}
	assert.Equal(t, 0, len(commits))
	assert.Equal(t, 0, len(commits[gb.RepoUrl()]))

	// Add a commit on the new branch.
	c3 := gb.CommitGen(ctx, "file2")
	branches, commits, err = cc.Update(ctx, w, false, N)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(branches))
	assert.Equal(t, 2, len(branches[gb.RepoUrl()]))
	if branches[gb.RepoUrl()][0].Name == "master" {
		assert.Equal(t, "branch2", branches[gb.RepoUrl()][1].Name)
		assert.Equal(t, c3, branches[gb.RepoUrl()][1].Head)
	} else {
		assert.Equal(t, "master", branches[gb.RepoUrl()][1].Name)
		assert.Equal(t, "branch2", branches[gb.RepoUrl()][0].Name)
		assert.Equal(t, c3, branches[gb.RepoUrl()][0].Head)
	}
	assert.Equal(t, 1, len(commits))
	assert.Equal(t, 1, len(commits[gb.RepoUrl()]))
	assert.Equal(t, c3, commits[gb.RepoUrl()][0].Hash)

	// Merge branch2 back into master. Note that, since there are no new
	// commits on master, this does not create a merge commit but just
	// updates HEAD of master to point at c3.
	gb.CheckoutBranch(ctx, "master")
	mergeCommit := gb.MergeBranch(ctx, "branch2")
	assert.Equal(t, c3, mergeCommit)
	branches, commits, err = cc.Update(ctx, w, false, N)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(branches))
	assert.Equal(t, 2, len(branches[gb.RepoUrl()]))
	if branches[gb.RepoUrl()][0].Name == "master" {
		assert.Equal(t, "branch2", branches[gb.RepoUrl()][1].Name)
	} else {
		assert.Equal(t, "master", branches[gb.RepoUrl()][1].Name)
		assert.Equal(t, "branch2", branches[gb.RepoUrl()][0].Name)
	}
	assert.Equal(t, c3, branches[gb.RepoUrl()][0].Head)
	assert.Equal(t, c3, branches[gb.RepoUrl()][1].Head)
	assert.Equal(t, 0, len(commits))
	assert.Equal(t, 0, len(commits[gb.RepoUrl()]))

	// Add a new branch. Add commits on both master and branch3.
	gb.CreateBranchTrackBranch(ctx, "branch3", "origin/master")
	c4 := gb.CommitGen(ctx, "file3")
	gb.CheckoutBranch(ctx, "master")
	c5 := gb.CommitGen(ctx, "file1")
	branches, commits, err = cc.Update(ctx, w, false, N)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(branches))
	assert.Equal(t, 3, len(branches[gb.RepoUrl()]))
	var master *gitinfo.GitBranch
	var branch2 *gitinfo.GitBranch
	var branch3 *gitinfo.GitBranch
	for _, b := range branches[gb.RepoUrl()] {
		if b.Name == "master" {
			master = b
		} else if b.Name == "branch2" {
			branch2 = b
		} else {
			assert.Equal(t, "branch3", b.Name)
			branch3 = b
		}
	}
	assert.Equal(t, c5, master.Head)
	assert.Equal(t, c4, branch3.Head)
	assert.Equal(t, c3, branch2.Head)
	assert.Equal(t, 1, len(commits))
	assert.Equal(t, 2, len(commits[gb.RepoUrl()]))
	if commits[gb.RepoUrl()][0].Hash == c4 {
		assert.Equal(t, c5, commits[gb.RepoUrl()][1].Hash)
	} else {
		assert.Equal(t, c4, commits[gb.RepoUrl()][1].Hash)
		assert.Equal(t, c5, commits[gb.RepoUrl()][0].Hash)
	}

	// Merge branch3 back into master. Because there are commits on both
	// branches, a merge commit will be created.
	c6 := gb.MergeBranch(ctx, "branch3")
	assert.NotEqual(t, c6, branch3.Head)
	branches, commits, err = cc.Update(ctx, w, false, N)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(branches))
	assert.Equal(t, 3, len(branches[gb.RepoUrl()]))
	for _, b := range branches[gb.RepoUrl()] {
		if b.Name == "master" {
			master = b
		} else if b.Name == "branch2" {
			branch2 = b
		} else {
			assert.Equal(t, "branch3", b.Name)
			branch3 = b
		}
	}
	assert.Equal(t, c6, master.Head)
	assert.Equal(t, c4, branch3.Head)
	assert.Equal(t, c3, branch2.Head)
	assert.Equal(t, 1, len(commits))
	assert.Equal(t, 1, len(commits[gb.RepoUrl()]))
	assert.Equal(t, c6, commits[gb.RepoUrl()][0].Hash)
}
