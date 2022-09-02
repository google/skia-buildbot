package buildskia

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGNGen(t *testing.T) {
	unittest.LinuxOnlyTest(t)
	mock := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mock.Run)

	err := GNGen(ctx, "/mnt/pd0/skia/", "/mnt/pd0/depot_tools", "Debug", []string{"is_debug=true"})
	require.NoError(t, err)

	got, want := exec.DebugString(mock.Commands()[0]), `gn gen out/Debug --args=is_debug=true`
	if !strings.HasSuffix(got, want) {
		t.Errorf("Failed: Command %q doesn't end with %q", got, want)
	}
}

func TestGNNinjaBuild(t *testing.T) {
	unittest.LinuxOnlyTest(t)
	mock := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mock.Run)

	_, err := GNNinjaBuild(ctx, "/mnt/pd0/skia/", "/mnt/pd0/depot_tools", "Debug", "", false)
	require.NoError(t, err)
	got, want := exec.DebugString(mock.Commands()[0]), "/mnt/pd0/depot_tools/ninja -C out/Debug"
	if !strings.HasSuffix(got, want) {
		t.Errorf("Failed: Command %q doesn't end with %q", got, want)
	}
}

func TestGNDownloadSkia(t *testing.T) {
	mock := exec.CommandCollector{}
	mock.SetDelegateRun(git_common.MocksForFindGit)
	ctx := exec.NewContext(context.Background(), mock.Run)

	checkout, err := ioutil.TempDir("", "download-test")
	require.NoError(t, err)
	defer func() {
		err := os.RemoveAll(checkout)
		if err != nil {
			t.Logf("Failed to clean up checkout: %s", err)
		}
	}()
	err = os.MkdirAll(filepath.Join(checkout, "skia"), 0777)
	require.NoError(t, err)

	_, err = GNDownloadSkia(ctx, git.MasterBranch, "aabbccddeeff", checkout, "/mnt/pd0/fiddle/depot_tools", false, false)
	// Not all of exec is mockable, so GNDownloadSkia will fail, but check the correctness
	// of the commands we did issue before hitting the failure point.
	require.Error(t, err)
	gitExec, err := git.Executable(ctx)
	require.NoError(t, err)
	expectedCommands := []string{
		"fetch skia",
		fmt.Sprintf("%s --version", gitExec),
		fmt.Sprintf("%s show-ref", gitExec),
		fmt.Sprintf("%s rev-list --max-parents=0 --first-parent HEAD", gitExec),
		fmt.Sprintf("%s reset --hard aabbccddeeff", gitExec),
		"gclient sync",
		"fetch-gn",
		gitExec + " log -n 1 --format=format:%H%n%P%n%an%x20(%ae)%n%s%n%b aabbccddeeff",
	}
	require.Equal(t, len(expectedCommands), len(mock.Commands()))
	for i, want := range expectedCommands {
		got := exec.DebugString(mock.Commands()[i])
		if !strings.HasSuffix(got, want) {
			t.Errorf("Failed: Command %q doesn't end with %q", got, want)
		}
	}
}

func TestGNNinjaBuildTarget(t *testing.T) {
	unittest.LinuxOnlyTest(t)
	mock := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mock.Run)

	_, err := GNNinjaBuild(ctx, "/mnt/pd0/skia/", "/mnt/pd0/depot_tools", "Debug", "fiddle", false)
	require.NoError(t, err)
	got, want := exec.DebugString(mock.Commands()[0]), "/mnt/pd0/depot_tools/ninja -C out/Debug fiddle"
	if !strings.HasSuffix(got, want) {
		t.Errorf("Failed: Command %q doesn't end with %q", got, want)
	}
}
