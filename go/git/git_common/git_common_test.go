package git_common_test

import (
	"context"
	"testing"

	"go.skia.org/infra/go/git/git_common"

	"github.com/stretchr/testify/require"
)

func TestFindGit(t *testing.T) {
	check := func() {
		git, major, minor, err := git_common.FindGit(context.Background())
		require.NoError(t, err)
		require.NotEqual(t, "", git)
		require.NotEqual(t, "git", git)
		require.NotEqual(t, 0, major)
		require.NotEqual(t, 0, minor)
	}
	check()
}
