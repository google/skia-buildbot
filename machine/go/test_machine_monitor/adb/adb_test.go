// Package adb is a simple wrapper around calling adb.
package adb

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/executil"
)

const (
	adbShellGetPropSuccess = `[ro.product.manufacturer]: [asus]
[ro.product.model]: [Nexus 7]
[ro.product.name]: [razor]
`
	adbShellDumpSysBattery = `Current Battery Service state:
AC powered: true
USB powered: false
Wireless powered: false
Max charging current: 1500000
Max charging voltage: 5000000
Charge counter: 1928561
status: 2
health: 2
present: true
level: 75
scale: 100
voltage: 3997
temperature: 248
technology: Li-ion`

	adbShellGetUptimeSuccess = `135.7 523.8`

	nonZeroExitCode = 123
)

func TestReboot_HappyPath(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_AdbReboot_Success")

	a := New()
	err := a.Reboot(ctx)
	require.NoError(t, err)
}

func TestReboot_ErrFromAdbNonZeroExitCode_ReconnectRecovers_Success(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_Reboot_NonZeroExitCode",
		"Test_FakeExe_ReconnectOffline_Success",
		"Test_FakeExe_AdbReboot_Success",
	)

	a := New()
	err := a.Reboot(ctx)
	require.NoError(t, err)
}

func TestReboot_ErrFromAdbNonZeroExitCode(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_Reboot_NonZeroExitCode",
		"Test_FakeExe_ReconnectOffline_NoDevice",
		"Test_FakeExe_Reboot_NonZeroExitCode",
	)

	a := New()
	err := a.Reboot(ctx)
	require.Error(t, err)
}

func TestUptime_HappyPath(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_AdbShellGetUptime_Success")

	a := New()
	got, err := a.Uptime(ctx)
	require.NoError(t, err)
	// Note that we truncate the 135.7 seconds to 135.
	assert.Equal(t, 135*time.Second, got)
}

func TestUptime_ErrOnMalformedUptimeContents(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_AdbShellGetUptime_MalformedContents")

	a := New()
	got, err := a.Uptime(ctx)
	require.Error(t, err)
	assert.Equal(t, time.Duration(0), got)
}

func TestUptime_ErrFromAdbNonZeroExitCode(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_Uptime_NonZeroExitCode")

	a := New()
	got, err := a.Uptime(ctx)
	require.Error(t, err)
	assert.Equal(t, time.Duration(0), got)
}

func TestRawProperties_HappyPath(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_AdbShellGetProp_Success")

	a := New()
	got, err := a.RawProperties(ctx)
	require.NoError(t, err)
	assert.Equal(t, adbShellGetPropSuccess, got)
}

func TestRawProperties_ErrFromAdbNonZeroExitCode(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_AdbShellGetProp_NonZeroExitCode")

	a := New()
	_, err := a.RawProperties(ctx)
	require.Error(t, err)
}

func TestRawProperties_EmptyOutputFromAdb(t *testing.T) {
	ctx := executil.FakeTestsContext("Test_FakeExe_AdbShellGetProp_EmptyOutput")

	a := New()
	got, err := a.RawProperties(ctx)
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func Test_FakeExe_AdbShellGetProp_Success(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "shell", "getprop"}, args)

	fmt.Print(adbShellGetPropSuccess)

	// Force exit so we don't get PASS in the output.
	os.Exit(0)
}

func Test_FakeExe_AdbShellGetProp_EmptyOutput(t *testing.T) {
	if executil.IsCallingFakeCommand() {
		// Force exit so we don't get PASS in the output.
		os.Exit(0)
	}
}

func Test_FakeExe_AdbShellGetProp_NonZeroExitCode(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	fmt.Fprintf(os.Stderr, "error: no devices/emulators found")
	os.Exit(nonZeroExitCode)
}

func TestRawDumpSys_HappyPath(t *testing.T) {

	ctx := executil.FakeTestsContext("Test_FakeExe_RawDumpSysBattery_Success")

	a := New()
	got, err := a.RawDumpSys(ctx, "battery")
	require.NoError(t, err)
	assert.Equal(t, adbShellDumpSysBattery, got)
}

func Test_FakeExe_RawDumpSysBattery_Success(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "shell", "dumpsys", "battery"}, args)

	fmt.Print(adbShellDumpSysBattery)

	// Force exit so we don't get PASS in the output.
	os.Exit(0)
}

func TestRawDumpSys_ErrOnNonZeroExitCode(t *testing.T) {

	ctx := executil.FakeTestsContext("Test_FakeExe_RawDumpSys_NonZeroExitCode")

	a := New()
	_, err := a.RawDumpSys(ctx, "battery")
	require.Error(t, err)
	// Confirm that both the exit code and the adb stderr make it into the returned error.
	assert.Contains(t, err.Error(), fmt.Sprintf("exit status %d", nonZeroExitCode))
	assert.Contains(t, err.Error(), "error: no devices/emulators found")
}

func TestGetState_HappyPath_Success(t *testing.T) {

	ctx := executil.FakeTestsContext("Test_FakeExe_AdbGetState_Success")

	a := New()
	state, err := a.getState(ctx)
	require.NoError(t, err)
	assert.Empty(t, state)
}

func TestGetState_Offline_ErrOnOffline(t *testing.T) {

	ctx := executil.FakeTestsContext("Test_FakeExe_AdbGetState_Offline")

	a := New()
	state, err := a.getState(ctx)
	require.Error(t, err)
	assert.Contains(t, state, "offline")
}

func TestGetState_Offline_ErrOnNoDeviceWithNoEmptyReturned(t *testing.T) {

	ctx := executil.FakeTestsContext("Test_FakeExe_AdbGetState_NoDevice")

	a := New()
	state, err := a.getState(ctx)
	require.Error(t, err)
	assert.Contains(t, state, "no devices/emulators found")
}

func TestEnsureOnline_HappyPath_Success(t *testing.T) {

	ctx := executil.FakeTestsContext("Test_FakeExe_AdbGetState_Success")

	a := New()
	err := a.EnsureOnline(ctx)
	require.NoError(t, err)
}

func TestEnsureOnline_Unauthorized_ErrOnUnauthorized(t *testing.T) {

	ctx := executil.FakeTestsContext("Test_FakeExe_AdbGetState_NoDevice")

	a := New()
	err := a.EnsureOnline(ctx)
	require.Contains(t, err.Error(), "adb returned an error state we can't do anything about:")
}

func TestEnsureOnline_OfflineAndReconnectWorks_Success(t *testing.T) {

	ctx := executil.FakeTestsContext(
		"Test_FakeExe_AdbGetState_Success",
		"Test_FakeExe_AdbGetState_Offline",
		"Test_FakeExe_ReconnectOffline_Success",
	)

	a := New()
	err := a.EnsureOnline(ctx)
	require.NoError(t, err)
}

func TestEnsureOnline_OfflineAndReconnectFails_ReturnsError(t *testing.T) {

	ctx := executil.FakeTestsContext(
		"Test_FakeExe_AdbGetState_Offline",
		"Test_FakeExe_ReconnectOffline_NoDevice",
		"Test_FakeExe_AdbGetState_Offline",
	)

	a := New()
	err := a.EnsureOnline(ctx)
	require.Contains(t, err.Error(), "adb get-state: failed with stderr")
}

func Test_FakeExe_RawDumpSys_NonZeroExitCode(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	fmt.Fprintf(os.Stderr, "error: no devices/emulators found")
	os.Exit(nonZeroExitCode)
}

func Test_FakeExe_AdbShellGetUptime_Success(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "shell", "cat", "/proc/uptime"}, args)

	fmt.Print(adbShellGetUptimeSuccess)

	// Force exit so we don't get PASS in the output.
	os.Exit(0)
}

func Test_FakeExe_Uptime_NonZeroExitCode(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	fmt.Fprintf(os.Stderr, "error: no devices/emulators found")
	os.Exit(nonZeroExitCode)
}

func Test_FakeExe_AdbShellGetUptime_MalformedContents(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "shell", "cat", "/proc/uptime"}, args)

	fmt.Print("this is not valid contents for /proc/uptime")

	// Force exit so we don't get PASS in the output.
	os.Exit(0)
}

func Test_FakeExe_AdbReboot_Success(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "reboot"}, args)

	// Force exit so we don't get PASS in the output.
	os.Exit(0)
}

func Test_FakeExe_Reboot_NonZeroExitCode(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	fmt.Fprintf(os.Stderr, "error: no devices/emulators found")
	os.Exit(nonZeroExitCode)
}

func Test_FakeExe_AdbGetState_Success(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "get-state"}, args)
	fmt.Fprintf(os.Stderr, "device")

	// Force exit so we don't get PASS in the output.
	os.Exit(0)
}

func Test_FakeExe_AdbGetState_Offline(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "get-state"}, args)
	fmt.Fprintf(os.Stderr, "error: device offline")
	os.Exit(1)
}

func Test_FakeExe_AdbGetState_NoDevice(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "get-state"}, args)
	fmt.Fprintf(os.Stderr, "error: no devices/emulators found")

	os.Exit(1)
}

func Test_FakeExe_ReconnectOffline_Success(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "reconnect", "offline"}, args)

	// Force exit so we don't get PASS in the output.
	os.Exit(0)
}

func Test_FakeExe_ReconnectOffline_NoDevice(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "reconnect", "offline"}, args)
	// adb reconnect always exits with status code 0, even if it failed to
	// reconnect.
	os.Exit(0)
}
