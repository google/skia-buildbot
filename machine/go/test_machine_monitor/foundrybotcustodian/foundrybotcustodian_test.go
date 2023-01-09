package foundrybotcustodian

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/recentschannel"
	"go.skia.org/infra/go/testutils"
	tmmMachine "go.skia.org/infra/machine/go/test_machine_monitor/machine"
)

// launchTimeout is how long we're willing to wait for a process to spin up.
const launchTimeout = time.Second

// Test_FakeExe_FoundryBot_ExitsWithZero pretends to be a Foundry Bot binary that exits immediately
// with a successful exit code.
func Test_FakeExe_FoundryBot_ExitsWithZero(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}
	require.Contains(t, executil.OriginalArgs(), "session")
	os.Exit(0)
}

// Test_FakeExe_FoundryBot_RunsForever pretends to be a Foundry Bot which runs happily forever. In
// reality, it times out but after the test that uses it does.
func Test_FakeExe_FoundryBot_RunsForever(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}
	require.Contains(t, executil.OriginalArgs(), "session")

	// Can't select {}, or Go's deadlock detection aborts the program.
	time.Sleep(2 * launchTimeout)
}

func TestStart_RelaunchesIfProcessExits(t *testing.T) {
	// This also tests the initial launch.
	ctx, cancel := context.WithCancel(executil.FakeTestsContext("Test_FakeExe_FoundryBot_ExitsWithZero",
		"Test_FakeExe_FoundryBot_RunsForever"))
	defer cancel()
	wantFoundryBotUpCh := recentschannel.New[bool](1)
	wantFoundryBotUpCh.Send(true)

	machine := &tmmMachine.Machine{}
	machine.SetIsRunningSwarmingTask(true)

	require.NoError(t, Start(ctx, testutils.Executable(t), "ignored", wantFoundryBotUpCh, machine, "ignored"))
	require.Eventually(t, func() bool {
		return executil.FakeCommandsReturned(ctx) >= 2
	}, launchTimeout, launchTimeout/10, "Foundry Bot never got relaunched after exiting.")
	// Machine gets set to no-task-running state on unexpected FB exits:
	require.False(t, machine.IsRunningSwarmingTask())
}

func TestStart_DoesntFindFoundryBot_ReturnsError(t *testing.T) {
	err := Start(
		context.Background(),
		"/something-that-does-not-exist",
		"ignored",
		recentschannel.New[bool](1),
		&tmmMachine.Machine{},
		"ignored")
	require.Contains(t, err.Error(), "Foundry Bot not found")
}

// flagFileForProcessStartAndInterrupt returns the path to the file through which we synchronize the
// fake Foundry Bot process with the test harness.
func flagFileForProcessStartAndInterrupt(t *testing.T) string {
	return testutils.FlagPath(t, "foundryBotStartAndInterrupt.temp")
}

// Test_FakeExe_FoundryBot_RunsUntilInterruptAndMakesFlagFile pretends to be a Foundry Bot which
// creates a temp file next to the test binary when it starts (as a cue to the test that the process
// is up), runs until it receives an interrupt signal, then removes the file (as a cue that it's
// down). It times out after a bit to keep it from going on forever if something goes wrong.
func Test_FakeExe_FoundryBot_RunsUntilInterruptAndMakesFlagFile(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}
	require.Contains(t, executil.OriginalArgs(), "session")

	// Make flag file.
	flag := flagFileForProcessStartAndInterrupt(t)
	file, err := os.Create(flag)
	require.NoError(t, err)
	require.NoError(t, file.Close())

	// Wait for interrupt or timeout.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	timeout := time.NewTicker(launchTimeout)
	select {
	case <-interrupt:
		require.NoError(t, os.Remove(flag))
	case <-timeout.C:
		// Let the file leak. If under Bazel, it's in a temp dir anyway.
	}
}

func TestStart_GracefullyStopsProcessIfHeartbeatSaysFalse(t *testing.T) {
	// Show we send an interrupt to Foundry Bot when a false goes down the heartbeat channel. Also
	// show we launch Foundry Bot when a true goes down the channel.
	wantFoundryBotUpCh := recentschannel.New[bool](1)
	wantFoundryBotUpCh.Send(true)
	ctx := executil.FakeTestsContext("Test_FakeExe_FoundryBot_RunsUntilInterruptAndMakesFlagFile")
	flag := flagFileForProcessStartAndInterrupt(t)

	machine := &tmmMachine.Machine{}
	machine.SetIsRunningSwarmingTask(true)

	require.NoError(t, Start(ctx, testutils.Executable(t), "ignored", wantFoundryBotUpCh, machine, "ignored"))

	// Wait until foundryBotStartAndInterrupt.temp exists, showing the process is up.
	//
	// Using the FS (relative to the test executable) as a place to rendezvous and also a
	// synchronization mechanism lets us avoid shoehorning extra channels, mutexes, and struct-level
	// vars into the implementation just to give visibility to tests.
	require.Eventually(t, func() bool {
		_, err := os.Stat(flag)
		return err == nil
	}, launchTimeout, launchTimeout/10, "Foundry Bot process never came up.")

	// Ask the process to exit.
	wantFoundryBotUpCh.Send(false)

	// Wait until temp file disappears, indicating the process has received the requisite SIGINT.
	require.Eventually(t, func() bool {
		_, err := os.Stat(flag)
		return errors.Is(err, os.ErrNotExist)
	}, launchTimeout, launchTimeout/10, "Foundry Bot process never caught SIGINT.")

	// Machine gets set to no-task-running state when we take FB down on purpose:
	require.False(t, machine.IsRunningSwarmingTask())
}

// TestPingURLs makes sure leading slashes are removed from paths (a helpful but unspecified
// behavior of the net/url.URL). It also codifies that we expect hostless URLs to be constructed if
// no host is passed in.
func TestPingURLs(t *testing.T) {
	startURL, endURL := pingURLs(":1234")
	assert.Equal(t, startURL, "http://:1234/on_before_task")
	assert.Equal(t, endURL, "http://:1234/on_after_task")

	startURL, endURL = pingURLs("localhost:5678")
	assert.Equal(t, startURL, "http://localhost:5678/on_before_task")
	assert.Equal(t, endURL, "http://localhost:5678/on_after_task")
}
