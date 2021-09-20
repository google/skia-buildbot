package machine

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/testutils/noop"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/test_machine_monitor/adb"
	"go.skia.org/infra/machine/go/test_machine_monitor/ssh"
)

const (
	// For example values, see adb_test.go
	getPropPlaceholder        = "Placeholder get prop response"
	dumpSysBatteryPlaceholder = "Placeholder dumpsys battery response"
	dumpSysThermalPlaceholder = "Placeholder dumpsys thermal response"
	// This formatting matters because it is processed in adb.go
	adbUptimePlaceholder = "123.4 567.8"

	testUserIP = "root@skia-foobar-01"
	// This example was taken directly from a production ChromeOS device
	sampleChromeOSlsbrelease = `CHROMEOS_DEVSERVER=
CHROMEOS_RELEASE_APPID={9A3BE5D2-C3DC-4AE6-9943-E2C113895DC5}
CHROMEOS_RELEASE_BOARD=octopus-signed-mp-v23keys
CHROMEOS_RELEASE_BRANCH_NUMBER=56
CHROMEOS_RELEASE_BUILDER_PATH=octopus-release/R89-13729.56.0
CHROMEOS_RELEASE_BUILD_NUMBER=13729
CHROMEOS_RELEASE_BUILD_TYPE=Official Build
CHROMEOS_RELEASE_CHROME_MILESTONE=89
CHROMEOS_RELEASE_DESCRIPTION=13729.56.0 (Official Build) stable-channel octopus
CHROMEOS_RELEASE_KEYSET=mp-v23
CHROMEOS_RELEASE_NAME=Chrome OS
CHROMEOS_RELEASE_PATCH_NUMBER=0
CHROMEOS_RELEASE_TRACK=stable-channel
CHROMEOS_RELEASE_UNIBUILD=1
CHROMEOS_RELEASE_VERSION=13729.56.0
DEVICETYPE=CHROMEBOOK
GOOGLE_RELEASE=13729.56.0`

	// This example was taken directly from a production ChromeOS device
	sampleChromeOSUptime = "1234.5 5678.9"
)

func TestTryInterrogatingAndroidDevice_DeviceAttached_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_AdbGetState_Success",
		"Test_FakeExe_ADBUptime_ReturnsPlaceholder",
		"Test_FakeExe_AdbShellGetProp_ReturnsPlaceholder",
		"Test_FakeExe_RawDumpSysBattery_ReturnsPlaceholder",
		"Test_FakeExe_RawDumpSysThermal_ReturnsPlaceholder",
	)

	m := &Machine{adb: adb.New()}
	actual, ok := m.tryInterrogatingAndroidDevice(ctx)
	assert.True(t, ok)
	assert.Equal(t, machine.Android{
		GetProp:               getPropPlaceholder,
		DumpsysBattery:        dumpSysBatteryPlaceholder,
		DumpsysThermalService: dumpSysThermalPlaceholder,
		Uptime:                123 * time.Second,
	}, actual)
}

func TestTryInterrogatingAndroidDevice_UptimeFails_DeviceConsideredNotAttached(t *testing.T) {
	unittest.SmallTest(t)
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_ExitCodeOne",
	)

	m := &Machine{adb: adb.New()}
	_, ok := m.tryInterrogatingAndroidDevice(ctx)
	assert.False(t, ok)
}

func TestTryInterrogatingAndroidDevice_ThermalFails_PartialSuccess(t *testing.T) {
	unittest.SmallTest(t)
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_AdbGetState_Success",
		"Test_FakeExe_ADBUptime_ReturnsPlaceholder",
		"Test_FakeExe_AdbShellGetProp_ReturnsPlaceholder",
		"Test_FakeExe_RawDumpSysBattery_ReturnsPlaceholder",
		"Test_FakeExe_ExitCodeOne",
	)

	m := &Machine{adb: adb.New()}
	actual, ok := m.tryInterrogatingAndroidDevice(ctx)
	assert.True(t, ok)
	assert.Equal(t, machine.Android{
		GetProp:        getPropPlaceholder,
		DumpsysBattery: dumpSysBatteryPlaceholder,
		Uptime:         123 * time.Second,
	}, actual)
}

func TestTryInterrogatingChromeOS_DeviceReachable_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_SSHLSBRelease_ReturnsPlaceholder",
		"Test_FakeExe_SSHUptime_ReturnsPlaceholder",
	)

	m := &Machine{ssh: ssh.ExeImpl{}, description: machine.Description{SSHUserIP: testUserIP}}
	actual, ok := m.tryInterrogatingChromeOSDevice(ctx)
	assert.True(t, ok)
	assert.Equal(t, machine.ChromeOS{
		Channel:        "stable-channel",
		Milestone:      "89",
		ReleaseVersion: "13729.56.0",
		Uptime:         1234500 * time.Millisecond,
	}, actual)
}

func TestTryInterrogatingChromeOS_CatLSBReleaseFails_DeviceConsideredUnattached(t *testing.T) {
	unittest.SmallTest(t)
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_ExitCodeOne",
	)

	m := &Machine{ssh: ssh.ExeImpl{}, description: machine.Description{SSHUserIP: testUserIP}}
	_, ok := m.tryInterrogatingChromeOSDevice(ctx)
	assert.False(t, ok)
}

func TestTryInterrogatingChromeOS_NoSSHUserIP_ReturnFalse(t *testing.T) {
	unittest.SmallTest(t)
	ctx := executil.FakeTestsContext() // Any exe call will panic

	m := &Machine{ssh: ssh.ExeImpl{}}
	_, ok := m.tryInterrogatingChromeOSDevice(ctx)
	assert.False(t, ok)
}

func TestTryInterrogatingChromeOS_PartialData_PartialSuccess(t *testing.T) {
	unittest.SmallTest(t)
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_SSHLSBRelease_ReturnsPlaceholder",
		"Test_FakeExe_ExitCodeOne", // pretend uptime fails
	)

	m := &Machine{ssh: ssh.ExeImpl{}, description: machine.Description{SSHUserIP: testUserIP}}
	actual, ok := m.tryInterrogatingChromeOSDevice(ctx)
	assert.True(t, ok)
	assert.Equal(t, machine.ChromeOS{
		Channel:        "stable-channel",
		Milestone:      "89",
		ReleaseVersion: "13729.56.0",
		// No uptime reported
	}, actual)
}

func TestTryInterrogatingChromeOS_NoChromeOSData_AssumesNotAttached(t *testing.T) {
	unittest.SmallTest(t)
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_SSHLSBRelease_ReturnsNonChromeOS",
	)

	m := &Machine{ssh: ssh.ExeImpl{}, description: machine.Description{SSHUserIP: testUserIP}}
	_, ok := m.tryInterrogatingChromeOSDevice(ctx)
	assert.False(t, ok)
}

func TestInterrogate_NoDeviceAttached_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_ExitCodeOne", // No Android device
	)

	m := &Machine{
		adb:              adb.New(),
		MachineID:        "some-machine",
		Version:          "some-version",
		runningTask:      true,
		startSwarming:    true,
		startTime:        time.Date(2021, time.September, 2, 2, 2, 2, 2, time.UTC),
		interrogateTimer: noop.Float64SummaryMetric{},
	}
	actual := m.interrogate(ctx)
	assert.Equal(t, machine.Event{
		EventType:           machine.EventTypeRawState,
		LaunchedSwarming:    true,
		RunningSwarmingTask: true,
		Host: machine.Host{
			Name:      "some-machine",
			Version:   "some-version",
			StartTime: time.Date(2021, time.September, 2, 2, 2, 2, 2, time.UTC),
		},
	}, actual)
}

func TestInterrogate_AndroidDeviceAttached_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_AdbGetState_Success",
		"Test_FakeExe_ADBUptime_ReturnsPlaceholder",
		"Test_FakeExe_AdbShellGetProp_ReturnsPlaceholder",
		"Test_FakeExe_RawDumpSysBattery_ReturnsPlaceholder",
		"Test_FakeExe_RawDumpSysThermal_ReturnsPlaceholder",
	)

	m := &Machine{
		adb:              adb.New(),
		MachineID:        "some-machine",
		Version:          "some-version",
		runningTask:      true,
		startSwarming:    true,
		startTime:        time.Date(2021, time.September, 2, 2, 2, 2, 2, time.UTC),
		interrogateTimer: noop.Float64SummaryMetric{},
	}
	actual := m.interrogate(ctx)
	assert.Equal(t, machine.Event{
		EventType:           machine.EventTypeRawState,
		LaunchedSwarming:    true,
		RunningSwarmingTask: true,
		Host: machine.Host{
			Name:      "some-machine",
			Version:   "some-version",
			StartTime: time.Date(2021, time.September, 2, 2, 2, 2, 2, time.UTC),
		},
		Android: machine.Android{
			GetProp:               getPropPlaceholder,
			DumpsysBattery:        dumpSysBatteryPlaceholder,
			DumpsysThermalService: dumpSysThermalPlaceholder,
			Uptime:                123 * time.Second,
		},
	}, actual)
}

func TestInterrogate_ChromeOSDeviceAttached_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_SSHLSBRelease_ReturnsPlaceholder",
		"Test_FakeExe_SSHUptime_ReturnsPlaceholder",
		// We found a device, no need to check for adb
	)

	m := &Machine{
		ssh:       ssh.ExeImpl{},
		MachineID: "some-machine",
		description: machine.Description{
			SSHUserIP: testUserIP,
		},
		Version:          "some-version",
		runningTask:      true,
		startSwarming:    true,
		startTime:        time.Date(2021, time.September, 2, 2, 2, 2, 2, time.UTC),
		interrogateTimer: noop.Float64SummaryMetric{},
	}
	actual := m.interrogate(ctx)
	assert.Equal(t, machine.Event{
		EventType:           machine.EventTypeRawState,
		LaunchedSwarming:    true,
		RunningSwarmingTask: true,
		Host: machine.Host{
			Name:      "some-machine",
			Version:   "some-version",
			StartTime: time.Date(2021, time.September, 2, 2, 2, 2, 2, time.UTC),
		},
		ChromeOS: machine.ChromeOS{
			Channel:        "stable-channel",
			Milestone:      "89",
			ReleaseVersion: "13729.56.0",
			Uptime:         1234500 * time.Millisecond,
		},
	}, actual)
}

func Test_FakeExe_ADBUptime_ReturnsPlaceholder(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}
	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "shell", "cat", "/proc/uptime"}, args)

	fmt.Print(adbUptimePlaceholder)
	os.Exit(0)
}

func Test_FakeExe_AdbShellGetProp_ReturnsPlaceholder(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "shell", "getprop"}, args)

	fmt.Print(getPropPlaceholder)
	os.Exit(0)
}

func Test_FakeExe_RawDumpSysBattery_ReturnsPlaceholder(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "shell", "dumpsys", "battery"}, args)

	fmt.Print(dumpSysBatteryPlaceholder)
	os.Exit(0)
}

func Test_FakeExe_RawDumpSysThermal_ReturnsPlaceholder(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}
	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "shell", "dumpsys", "thermalservice"}, args)

	fmt.Print(dumpSysThermalPlaceholder)
	os.Exit(0)
}

func Test_FakeExe_SSHLSBRelease_ReturnsPlaceholder(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}
	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Contains(t, args, "ssh")
	require.Contains(t, args, testUserIP)
	require.Contains(t, args, "/etc/lsb-release")

	fmt.Print(sampleChromeOSlsbrelease)
	os.Exit(0)
}

func Test_FakeExe_SSHLSBRelease_ReturnsNonChromeOS(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}
	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Contains(t, args, "ssh")
	require.Contains(t, args, testUserIP)
	require.Contains(t, args, "/etc/lsb-release")

	fmt.Print("FOO=bar")
	os.Exit(0)
}

func Test_FakeExe_SSHUptime_ReturnsPlaceholder(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}
	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Contains(t, args, "ssh")
	require.Contains(t, args, testUserIP)
	require.Contains(t, args, "/proc/uptime")

	fmt.Print(sampleChromeOSUptime)
	os.Exit(0)
}

func Test_FakeExe_ExitCodeOne(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	os.Exit(1)
}

func Test_FakeExe_AdbGetState_Success(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "get-state"}, args)
	_, _ = fmt.Fprintf(os.Stderr, "device")

	// Force exit so we don't get PASS in the output.
	os.Exit(0)
}
