package git_common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
)

func TestFindGit(t *testing.T) {

	execCount := 0
	mockRun := exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		execCount++
		return exec.DefaultRun(ctx, cmd)
	})
	ctx := exec.NewContext(context.Background(), mockRun.Run)

	check := func() {
		git, major, minor, err := FindGit(ctx)
		require.NoError(t, err)
		require.NotEqual(t, "", git)
		require.NotEqual(t, "git", git)
		require.NotEqual(t, 0, major)
		require.NotEqual(t, 0, minor)
		// TODO(borenet): We want to ensure that we get Git from CIPD
		// on all bots and servers, but we don't want to impose that
		// restriction on developers.
		//require.True(t, IsFromCIPD(git))
	}
	check()
	require.Equal(t, 1, execCount)

	// Ensure that we cached the results.
	check()
	require.Equal(t, 1, execCount)
}
