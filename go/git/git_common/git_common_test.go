package git_common_test

import (
	"context"
	"testing"

	"go.skia.org/infra/go/git/git_common"

	"github.com/stretchr/testify/require"

	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
)

func TestFindGit_WithGitFinder(t *testing.T) {
	execCount := 0
	gitFinder := func() (string, error) {
		execCount++
		return cipd_git.Find()
	}
	ctx := git_common.WithGitFinder(context.Background(), gitFinder)

	check := func() {
		git, major, minor, isWrapper, err := git_common.FindGit(ctx)
		require.NoError(t, err)
		require.NotEqual(t, "", git)
		require.NotEqual(t, "git", git)
		require.NotEqual(t, 0, major)
		require.NotEqual(t, 0, minor)
		require.False(t, isWrapper)
	}
	check()
	require.Equal(t, 1, execCount)
}
