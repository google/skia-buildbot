package incremental

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/task_scheduler/go/window"
)

func assertBranches(t *testing.T, gb *git_testutils.GitBuilder, actual map[string][]*git.Branch, expect map[string]string) {
	actualBranches := make(map[string]string, len(actual[gb.RepoUrl()]))
	for _, branch := range actual[gb.RepoUrl()] {
		actualBranches[branch.Name] = branch.Head
	}
	assertdeep.Equal(t, expect, actualBranches)
}

func assertCommits(t *testing.T, gb *git_testutils.GitBuilder, actual map[string][]*vcsinfo.LongCommit, expect []string) {
	actualMap := util.NewStringSet(nil)
	for _, c := range actual[gb.RepoUrl()] {
		actualMap[c.Hash] = true
	}
	expectMap := util.NewStringSet(expect)
	assertdeep.Equal(t, expectMap, actualMap)
}

func TestIncrementalCommits(t *testing.T) {
	// Setup.
	ctx := cipd_git.UseGitFinder(context.Background())
	gb := git_testutils.GitInit(t, ctx)
	defer gb.Cleanup()
	c0 := gb.CommitGen(ctx, "file1")
	wd := t.TempDir()
	repo, err := repograph.NewLocalGraph(ctx, gb.Dir(), wd)
	require.NoError(t, err)
	repos := repograph.Map{
		gb.RepoUrl(): repo,
	}
	N := 100
	w, err := window.New(ctx, 24*time.Hour, N, repos)
	require.NoError(t, err)
	cc := newCommitsCache(repos)

	// Initial update. Expect a single branch with one commit.
	branches, commits, err := cc.Update(ctx, w, false, N)
	require.NoError(t, err)
	assertBranches(t, gb, branches, map[string]string{
		git.MainBranch: c0,
	})
	assertCommits(t, gb, commits, []string{c0})

	// Update again, with no new commits. Expect empty response.
	branches, commits, err = cc.Update(ctx, w, false, N)
	require.NoError(t, err)
	assertBranches(t, gb, branches, map[string]string{})
	assertCommits(t, gb, commits, []string{})

	// Passing in reset=true should give us ALL commits and branches,
	// regardless of whether they're new.
	branches, commits, err = cc.Update(ctx, w, true, N)
	require.NoError(t, err)
	assertBranches(t, gb, branches, map[string]string{
		git.MainBranch: c0,
	})
	assertCommits(t, gb, commits, []string{c0})

	// Add some new commits.
	c1 := gb.CommitGen(ctx, "file1")
	c2 := gb.CommitGen(ctx, "file1")
	branches, commits, err = cc.Update(ctx, w, false, N)
	require.NoError(t, err)
	assertBranches(t, gb, branches, map[string]string{
		git.MainBranch: c2,
	})
	assertCommits(t, gb, commits, []string{c1, c2})

	// Add a new branch, with no commits.
	gb.CreateBranchTrackBranch(ctx, "branch2", git.DefaultRemoteBranch)
	branches, commits, err = cc.Update(ctx, w, false, N)
	require.NoError(t, err)
	assertBranches(t, gb, branches, map[string]string{
		git.MainBranch: c2,
		"branch2":      c2,
	})
	assertCommits(t, gb, commits, []string{})

	// Add a commit on the new branch.
	c3 := gb.CommitGen(ctx, "file2")
	branches, commits, err = cc.Update(ctx, w, false, N)
	require.NoError(t, err)
	assertBranches(t, gb, branches, map[string]string{
		git.MainBranch: c2,
		"branch2":      c3,
	})
	assertCommits(t, gb, commits, []string{c3})

	// Merge branch2 back into main. Note that, since there are no new
	// commits on main, this does not create a merge commit but just
	// updates HEAD of main to point at c3.
	gb.CheckoutBranch(ctx, git.MainBranch)
	mergeCommit := gb.MergeBranch(ctx, "branch2")
	require.Equal(t, c3, mergeCommit)
	branches, commits, err = cc.Update(ctx, w, false, N)
	require.NoError(t, err)
	assertBranches(t, gb, branches, map[string]string{
		git.MainBranch: c3,
		"branch2":      c3,
	})
	assertCommits(t, gb, commits, []string{})

	// Add a new branch. Add commits on both main and branch3.
	gb.CreateBranchTrackBranch(ctx, "branch3", git.DefaultRemoteBranch)
	c4 := gb.CommitGen(ctx, "file3")
	gb.CheckoutBranch(ctx, git.MainBranch)
	c5 := gb.CommitGen(ctx, "file1")
	branches, commits, err = cc.Update(ctx, w, false, N)
	require.NoError(t, err)
	assertBranches(t, gb, branches, map[string]string{
		git.MainBranch: c5,
		"branch2":      c3,
		"branch3":      c4,
	})
	assertCommits(t, gb, commits, []string{c4, c5})

	// Merge branch3 back into main. Because there are commits on both
	// branches, a merge commit will be created.
	c6 := gb.MergeBranch(ctx, "branch3")
	require.NotEqual(t, c6, c4) // Ensure that we actually created a merge commit.
	branches, commits, err = cc.Update(ctx, w, false, N)
	require.NoError(t, err)
	assertBranches(t, gb, branches, map[string]string{
		git.MainBranch: c6,
		"branch2":      c3,
		"branch3":      c4,
	})
	assertCommits(t, gb, commits, []string{c6})
}
