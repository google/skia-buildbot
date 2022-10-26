package foundrybotrunner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/util"
)

const relaunchTimeout = time.Second

// Test_FakeExe_FoundryBot_ExitsWithZero pretends to be a Foundry Bot binary that exits immediately
// with a successful exit code.
func Test_FakeExe_FoundryBot_ExitsWithZero(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}
	require.Contains(t, executil.OriginalArgs(), "session")
	os.Exit(0)
}

// Test_FakeExe_FoundryBot_RunsForever pretends to be a Foundry Bot which runs happily forever.
func Test_FakeExe_FoundryBot_RunsForever(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}
	require.Contains(t, executil.OriginalArgs(), "session")

	// Can't select {}, or Go's deadlock detection aborts the program.
	time.Sleep(2 * relaunchTimeout)
}

func TestRunUntilCancelled_RelaunchesIfProcessExits(t *testing.T) {
	runner := &Runner{path: "ignored"}
	ctx, cancel := context.WithCancel(executil.FakeTestsContext("Test_FakeExe_FoundryBot_ExitsWithZero",
		"Test_FakeExe_FoundryBot_RunsForever"))
	defer cancel()
	go func() {
		_ = runner.RunUntilCancelled(ctx)
	}()
	const periods = 100
	for i := 0; i <= periods; i++ { // Time out after a second.
		if executil.FakeCommandsReturned(ctx) >= 2 {
			// Success
			return
		}
		time.Sleep(relaunchTimeout / periods)
	}
	require.FailNow(t, "Timed out while waiting for Foundry Bot to be relaunched after exiting.")
}

func TestRunUntilCancelled_StopsRelaunchingWhenContextIsCancelled(t *testing.T) {
	// Dodge New() so we don't have to provide a Foundry Bot next to our executable.
	executable, err := os.Executable()
	require.NoError(t, err)
	runner := &Runner{path: executable}

	ctx, cancel := context.WithCancel(executil.FakeTestsContext("Test_FakeExe_FoundryBot_ExitsWithZero"))
	cancel()

	err = runner.RunUntilCancelled(ctx)
	require.Contains(t, err.Error(), "context was cancelled")
}

func TestBotPath_DoesntFindFoundryBot_ReturnsError(t *testing.T) {
	_, err := botPath()
	require.Contains(t, err.Error(), "Foundry Bot not found")
}

func TestBotPath_FindsFoundryBot_ReturnsPath(t *testing.T) {
	// Create Foundry Bot right next to our executable, making sure we don't overwrite anything that
	// somehow is already there.
	tmm, err := os.Executable()
	require.NoError(t, err)
	foundryBot := filepath.Join(filepath.Dir(tmm), "bot.1")
	require.NoFileExists(t, foundryBot, "Avoiding overwriting an existing bot.1 file. Please move it aside and re-run the test.")
	_, err = os.Create(foundryBot)
	require.NoError(t, err)
	defer util.Remove(foundryBot)

	path, err := botPath()
	require.Equal(t, foundryBot, path)
}
