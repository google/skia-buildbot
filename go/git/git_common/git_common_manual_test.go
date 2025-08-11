package git_common_test

import (
	"context"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/skerr"
)

func TestFindGit_BypassWrapper(t *testing.T) {
	// Lots of setup.
	wrapperGit := "/path/to/wrapper/git"
	realGit := "/path/to/real/git"

	// Mock out os.Stat to find our wrapped and real git executables.
	ctx := exec.WithOverrideExecutableExists(t.Context(), func(name string) bool {
		return name == wrapperGit || name == realGit
	})

	// Override PATH and os.Stat.
	oldPATH := os.Getenv("PATH")
	require.NoError(t, os.Setenv("PATH", strings.Join([]string{
		path.Dir(wrapperGit),
		path.Dir(realGit),
	}, string(os.PathListSeparator))))
	defer func() {
		require.NoError(t, os.Setenv("PATH", oldPATH))
	}()

	// Override exec run.
	mockRun := &exec.CommandCollector{}
	ctx = exec.NewContext(ctx, mockRun.Run)
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		var output string
		if cmd.Name == wrapperGit {
			output = "git version 2.45.2.chromium.11 / Infra wrapper (infra/tools/git/linux-amd64 @ EpkL_3RTtPZV2hGJqsC6xZ4SBj_KCQmdl3Vy2amJ4MAC)"
		} else if cmd.Name == realGit {
			output = "git version 2.45.2.chromium.11"
		} else {
			return skerr.Fmt("Unknown command %s %s", cmd.Name, strings.Join(cmd.Args, " "))
		}
		_, err := cmd.CombinedOutput.Write([]byte(output))
		return err
	})

	// Actually test.
	git, _, _, isWrapper, err := git_common.FindGit(ctx)
	require.NoError(t, err)
	require.Equal(t, wrapperGit, git)
	require.True(t, isWrapper)

	// Now set BypassWrapper and ensure that we get the real git.
	git_common.BypassWrapper(true)
	defer git_common.BypassWrapper(false)
	git, _, _, isWrapper, err = git_common.FindGit(ctx)
	require.NoError(t, err)
	require.Equal(t, realGit, git)
	require.False(t, isWrapper)
}
