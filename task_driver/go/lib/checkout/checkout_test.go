package checkout

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_driver/go/td"
	"go.skia.org/infra/task_scheduler/go/types"
)

func TestEnsureGitCheckout(t *testing.T) {
	unittest.LargeTest(t)

	// Setup.
	ctx := context.Background()
	f := "myfile.txt"
	gb := git_testutils.GitInit(t, ctx)
	c1 := gb.CommitGen(ctx, f)

	rs := types.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}

	// check is a helper function which creates a temporary dir, runs the
	// passed-in function so that the caller can manipulate the dir, then
	// runs EnsureGitCheckout and verifies that it did the correct thing.
	check := func(rs types.RepoState, fn func(string)) {
		require.True(t, rs.Valid())

		// Create temp dir.
		wd := t.TempDir()
		dest := filepath.Join(wd, "repo")

		// Let the caller manipulate the dir.
		fn(dest)

		// Run EnsureGitCheckout, asserting that it returns no error.
		var co *git.Checkout
		_ = td.RunTestSteps(t, false, func(ctx context.Context) error {
			var err error
			co, err = EnsureGitCheckout(ctx, dest, rs)
			require.NoError(t, err)
			return nil
		})

		// Verify that the checkout is in the correct state.

		// Assert that we're at the correct revision.
		if rs.IsTryJob() {
			// For try jobs, we end up with a different HEAD, so we
			// can only check that the patch ref was applied on top
			// of the requested revision.
			ancestor, err := co.IsAncestor(ctx, rs.Revision, "HEAD")
			require.NoError(t, err)
			require.True(t, ancestor)
			// This is only true for our test case, but assert that
			// exactly one commit was applied on top of rs.Revision.
			revs, err := co.RevList(ctx, fmt.Sprintf("%s..HEAD", rs.Revision))
			require.NoError(t, err)
			require.Equal(t, 1, len(revs))
		} else {
			gotRev, err := co.RevParse(ctx, "HEAD")
			require.NoError(t, err)
			require.Equal(t, rs.Revision, gotRev)
		}

		// Assert that there are no modified or untracked files.
		output, err := co.Git(ctx, "status", "--porcelain")
		require.NoError(t, err)
		require.Equal(t, output, "")

		// Assert that we're on the main branch, regardless of what
		// commit we wanted.
		output, err = co.Git(ctx, "rev-parse", "--abbrev-ref", "HEAD")
		require.NoError(t, err)
		require.Equal(t, git.DefaultBranch, strings.TrimSpace(output))
	}

	// Case 1: Dest dir does not exist.
	check(rs, func(dest string) {})

	// Case 2: Dest dir exists and is empty. We should remove the dir before
	// syncing in this case.
	check(rs, func(dest string) {
		require.NoError(t, os.MkdirAll(dest, os.ModePerm))
	})

	// Case 3: Dest dir exists and has a .git dir in it. It is NOT a valid
	// git checkout, however.
	check(rs, func(dest string) {
		require.NoError(t, os.MkdirAll(filepath.Join(dest, ".git"), os.ModePerm))
	})

	// Case 4: Dest dir is a git checkout with no remote.
	check(rs, func(dest string) {
		require.NoError(t, os.MkdirAll(dest, os.ModePerm))
		_, err := git.GitDir(dest).Git(ctx, "init")
		require.NoError(t, err)
	})

	// Case 5: Dest dir is a git checkout with the wrong remote.
	check(rs, func(dest string) {
		require.NoError(t, os.MkdirAll(dest, os.ModePerm))
		g := git.GitDir(dest)
		_, err := g.Git(ctx, "init")
		require.NoError(t, err)
		_, err = g.Git(ctx, "remote", "add", git.DefaultRemote, common.REPO_SKIA)
		require.NoError(t, err)
	})

	// Case 6: Dest dir is a git checkout with the right remote, but it has
	// never fetched. In this case, there is no HEAD.
	check(rs, func(dest string) {
		require.NoError(t, os.MkdirAll(dest, os.ModePerm))
		g := git.GitDir(dest)
		_, err := g.Git(ctx, "init")
		require.NoError(t, err)
		_, err = g.Git(ctx, "remote", "add", git.DefaultRemote, rs.Repo)
		require.NoError(t, err)
	})

	// Case 7: Checkout already exists and is clean. We should not remove
	// the dir in this case.
	gitExec, err := git.Executable(ctx)
	require.NoError(t, err)
	check(rs, func(dest string) {
		_, err := exec.RunCwd(ctx, ".", gitExec, "clone", rs.Repo, dest)
		require.NoError(t, err)
	})

	// Case 8: Checkout already exists and has modified / untracked files.
	check(rs, func(dest string) {
		_, err := exec.RunCwd(ctx, ".", gitExec, "clone", rs.Repo, dest)
		require.NoError(t, err)
		testutils.WriteFile(t, filepath.Join(dest, f), "modified file")
		testutils.WriteFile(t, filepath.Join(dest, "untracked_file"), "this file is untracked")
	})

	// Case 9: Checkout already exists and has new commits.
	check(rs, func(dest string) {
		_, err := exec.RunCwd(ctx, ".", gitExec, "clone", rs.Repo, dest)
		require.NoError(t, err)
		updated := filepath.Join(dest, f)
		testutils.WriteFile(t, updated, "modified file")
		_, err = git.GitDir(dest).Git(ctx, "commit", "-a", "-m", "modified a file")
		require.NoError(t, err)
	})

	// Case 10: Checkout already exists and is on a different branch.
	check(rs, func(dest string) {
		_, err := exec.RunCwd(ctx, ".", gitExec, "clone", rs.Repo, dest)
		require.NoError(t, err)
		gd := git.GitDir(dest)
		_, err = gd.Git(ctx, "checkout", "-b", "newbranch", "-t", git.DefaultBranch)
		require.NoError(t, err)
		updated := filepath.Join(dest, f)
		testutils.WriteFile(t, updated, "modified file")
		_, err = gd.Git(ctx, "commit", "-a", "-m", "modified a file")
		require.NoError(t, err)
	})

	// Case 11: Apply a patch.
	rs.Issue = "11011"
	rs.Patchset = "2"
	rs.Server = "https://fake-review.googlesource.com"
	gb.CreateFakeGerritCLGen(ctx, rs.Issue, rs.Patchset)
	check(rs, func(dest string) {})
}
