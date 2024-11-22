package machine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/noop"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machine/change/source/mocks"
	sinkMocks "go.skia.org/infra/machine/go/machine/event/sink/mocks"
	"go.skia.org/infra/machine/go/machineserver/rpc"
	"go.skia.org/infra/machine/go/test_machine_monitor/adb"
	"go.skia.org/infra/machine/go/test_machine_monitor/ios"
	"go.skia.org/infra/machine/go/test_machine_monitor/pyocd"
	"go.skia.org/infra/machine/go/test_machine_monitor/ssh"
)

const (
	// For example values, see adb_test.go
	getPropPlaceholder        = "Placeholder get prop response"
	dumpSysBatteryPlaceholder = "Placeholder dumpsys battery response"
	dumpSysThermalPlaceholder = "Placeholder dumpsys thermal response"
	// This formatting matters because it is processed in adb.go
	adbUptimePlaceholder = "123.4 567.8"

	iOSVersionPlaceholder    = "99.9.9"
	iOSDeviceTypePlaceholder = "iPhone88,8"

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

	machineID = "skia-rpi2-rack4-shelf1-001"

	adbGetStateSuccess       = "device"
	adbShellGetUptimeSuccess = "27 8218793.04"
	adbShellGetPropSuccess   = "[ro.product.manufacturer]: [asus]"
	adbShellDumpSysBattery   = "This is dumpsys output."
	versionForTest           = "some-version-string-for-testing-purposes"
	maintenanceModeMessage   = "This is a note about why the machine was put in maintenance mode."
	machineServerHost        = "https://machines.skia.org"
)

func TestTryInterrogatingAndroidDevice_DeviceAttached_Success(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_AdbGetState_Success",
		"Test_FakeExe_ADBUptime_ReturnsPlaceholder",
		"Test_FakeExe_AdbShellGetProp_ReturnsPlaceholder",
		"Test_FakeExe_RawDumpSysBattery_ReturnsPlaceholder",
		"Test_FakeExe_RawDumpSysThermal_ReturnsPlaceholder",
	)

	m := &Machine{adb: adb.New()}
	actual, err := m.tryInterrogatingAndroidDevice(ctx)
	require.NoError(t, err)
	assert.Equal(t, machine.Android{
		GetProp:               getPropPlaceholder,
		DumpsysBattery:        dumpSysBatteryPlaceholder,
		DumpsysThermalService: dumpSysThermalPlaceholder,
		Uptime:                123 * time.Second,
	}, actual)
}

func TestTryInterrogatingAndroidDevice_UptimeFails_DeviceConsideredNotAttached(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_ExitCodeOne",
	)

	m := &Machine{adb: adb.New()}
	_, err := m.tryInterrogatingAndroidDevice(ctx)
	require.Error(t, err)
}

func TestTryInterrogatingAndroidDevice_ThermalFails_PartialSuccess(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_AdbGetState_Success",
		"Test_FakeExe_ADBUptime_ReturnsPlaceholder",
		"Test_FakeExe_AdbShellGetProp_ReturnsPlaceholder",
		"Test_FakeExe_RawDumpSysBattery_ReturnsPlaceholder",
		"Test_FakeExe_ExitCodeOne",
	)

	m := &Machine{adb: adb.New()}
	actual, err := m.tryInterrogatingAndroidDevice(ctx)
	require.NoError(t, err)
	assert.Equal(t, machine.Android{
		GetProp:        getPropPlaceholder,
		DumpsysBattery: dumpSysBatteryPlaceholder,
		Uptime:         123 * time.Second,
	}, actual)
}

func TestTryInterrogatingChromeOS_DeviceReachable_Success(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_SSHUptime_ReturnsPlaceholder",
		"Test_FakeExe_SSHLSBRelease_ReturnsPlaceholder",
	)
	sshFile := filepath.Join(t.TempDir(), "test.json")
	m := &Machine{
		ssh:                ssh.ExeImpl{},
		sshMachineLocation: sshFile,
		description:        machine.Description{SSHUserIP: testUserIP},
	}
	actual, err := m.tryInterrogatingChromeOSDevice(ctx)
	require.NoError(t, err)
	assert.Equal(t, machine.ChromeOS{
		Channel:        "stable-channel",
		Milestone:      "89",
		ReleaseVersion: "13729.56.0",
		Uptime:         1234500 * time.Millisecond,
	}, actual)

	require.FileExists(t, sshFile)
	b, err := os.ReadFile(sshFile)
	require.NoError(t, err)
	const expected = `{
  "Comment": "This file is written to by test_machine_monitor. Do not edit by hand.",
  "user_ip": "root@skia-foobar-01"
}
`
	assert.Equal(t, expected, string(b))
}

func TestTryInterrogatingChromeOS_CatLSBReleaseFails_DeviceConsideredUnattached(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_SSHUptime_ReturnsPlaceholder",
		"Test_FakeExe_ExitCodeOne", // pretend LSBRelease failed
	)

	m := &Machine{ssh: ssh.ExeImpl{}, description: machine.Description{SSHUserIP: testUserIP}}
	_, err := m.tryInterrogatingChromeOSDevice(ctx)
	require.Error(t, err)
}

func TestTryInterrogatingChromeOS_NoSSHUserIP_ReturnFalse(t *testing.T) {
	ctx := executil.FakeTestsContext() // Any exe call will panic

	m := &Machine{ssh: ssh.ExeImpl{}}
	_, err := m.tryInterrogatingChromeOSDevice(ctx)
	require.Error(t, err)
}

func TestTryInterrogatingChromeOS_UptimeFails_ReturnFalse(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_ExitCodeOne", // pretend uptime fails
	)

	m := &Machine{ssh: ssh.ExeImpl{}, description: machine.Description{SSHUserIP: testUserIP}}
	_, err := m.tryInterrogatingChromeOSDevice(ctx)
	require.Error(t, err)
}

func TestTryInterrogatingChromeOS_NoChromeOSData_AssumesNotAttached(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_SSHLSBRelease_ReturnsNonChromeOS",
	)

	m := &Machine{ssh: ssh.ExeImpl{}, description: machine.Description{SSHUserIP: testUserIP}}
	_, err := m.tryInterrogatingChromeOSDevice(ctx)
	require.Error(t, err)
}

func TestInterrogate_NoDeviceAttached_Success(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_ExitCodeOne", // No Android device
		"Test_FakeExe_ExitCodeOne", // No iOS device
	)

	m := &Machine{
		adb:              adb.New(),
		ios:              ios.New(),
		MachineID:        "some-machine",
		Version:          "some-version",
		runningTask:      true,
		startSwarming:    true,
		startTime:        time.Date(2021, time.September, 2, 2, 2, 2, 2, time.UTC),
		interrogateTimer: noop.Float64SummaryMetric{},
	}
	actual, err := m.interrogate(ctx)
	require.NoError(t, err)
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
		description: machine.Description{
			AttachedDevice: machine.AttachedDeviceAdb,
		},
	}
	actual, err := m.interrogate(ctx)
	require.NoError(t, err)
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

func goodIOSInterrogationResult(timePlaceholder time.Time) (*Machine, machine.Event) {
	m := &Machine{
		adb:              adb.New(),
		ios:              ios.New(),
		MachineID:        "some-machine",
		Version:          "some-version",
		runningTask:      true,
		startSwarming:    true,
		startTime:        timePlaceholder,
		interrogateTimer: noop.Float64SummaryMetric{},
		description: machine.Description{
			AttachedDevice: machine.AttachedDeviceIOS,
		},
	}
	expected := machine.Event{
		EventType:           machine.EventTypeRawState,
		LaunchedSwarming:    true,
		RunningSwarmingTask: true,
		Host: machine.Host{
			Name:      "some-machine",
			Version:   "some-version",
			StartTime: timePlaceholder,
		},
		IOS: machine.IOS{
			OSVersion:  iOSVersionPlaceholder,
			DeviceType: iOSDeviceTypePlaceholder,
			Battery:    33,
		},
	}
	return m, expected
}

// Just make sure it can get into tryInterrogatingIOSDevice(), and test success while we're at it.
// Tests that call tryInterrogatingIOSDevice() directly cover the other cases.
func TestInterrogate_IOSDeviceAttached_Success(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_IDeviceInfo_ReturnsDeviceType",
		"Test_FakeExe_IDeviceInfo_ReturnsOSVersion",
		"Test_FakeExe_IDeviceInfo_ReturnsGoodBatteryLevel",
	)

	timePlaceholder := time.Date(2021, time.September, 2, 2, 2, 2, 2, time.UTC)
	m, expected := goodIOSInterrogationResult(timePlaceholder)
	actual, err := m.interrogate(ctx)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestInterrogate_IOSDeviceAttachedButBatteryCallFails_StillSuccess(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_IDeviceInfo_ReturnsDeviceType",
		"Test_FakeExe_IDeviceInfo_ReturnsOSVersion",
		"Test_FakeExe_ExitCodeOne", // battery level fails
	)

	timePlaceholder := time.Date(2021, time.September, 2, 2, 2, 2, 2, time.UTC)
	m, expected := goodIOSInterrogationResult(timePlaceholder)
	expected.IOS.Battery = machine.BadBatteryLevel // Battery value should reflect failure.
	actual, err := m.interrogate(ctx)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestTryInterrogatingIOSDevice_OSVersionFails_Fails(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_IDeviceInfo_ReturnsDeviceType",
		"Test_FakeExe_ExitCodeOne", // OS version check goes kaboom.
		"Test_FakeExe_IDeviceInfo_ReturnsGoodBatteryLevel",
	)
	m := &Machine{ios: ios.New()}
	_, err := m.tryInterrogatingIOSDevice(ctx)
	require.Error(t, err)
}

func TestTryInterrogatingIOSDevice_DeviceTypeFails_DeviceConsideredUnattached(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_ExitCodeOne", // Device-type check fails.
	)
	m := &Machine{ios: ios.New()}
	_, err := m.tryInterrogatingIOSDevice(ctx)
	require.Error(t, err)
}

func TestInterrogate_EVKDeviceAttached_Success(t *testing.T) {
	ctx := context.Background()

	m := &Machine{
		pyocd:     pyocd.WithHardcodedMachine("STM32U5"),
		MachineID: "some-machine",
		description: machine.Description{
			AttachedDevice: machine.AttachedDevicePyOCD,
		},
		Version:          "some-version",
		startTime:        time.Date(2024, time.July, 7, 7, 7, 7, 0, time.UTC),
		interrogateTimer: noop.Float64SummaryMetric{},
	}
	actual, err := m.interrogate(ctx)
	require.NoError(t, err)
	assert.Equal(t, machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name:      "some-machine",
			Version:   "some-version",
			StartTime: time.Date(2024, time.July, 7, 7, 7, 7, 0, time.UTC),
		},
		PyOCD: machine.PyOCD{
			DeviceType: "STM32U5",
		},
	}, actual)
}

func TestInterrogate_ChromeOSDeviceAttached_Success(t *testing.T) {
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_SSHUptime_ReturnsPlaceholder",
		"Test_FakeExe_SSHLSBRelease_ReturnsPlaceholder",
		// We found a device, no need to check for adb
	)
	sshFile := filepath.Join(t.TempDir(), "test.json")
	m := &Machine{
		ssh:                ssh.ExeImpl{},
		sshMachineLocation: sshFile,
		MachineID:          "some-machine",
		description: machine.Description{
			SSHUserIP:      testUserIP,
			AttachedDevice: machine.AttachedDeviceSSH,
		},
		Version:          "some-version",
		runningTask:      true,
		startSwarming:    true,
		startTime:        time.Date(2021, time.September, 2, 2, 2, 2, 2, time.UTC),
		interrogateTimer: noop.Float64SummaryMetric{},
	}
	actual, err := m.interrogate(ctx)
	require.NoError(t, err)
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

	require.FileExists(t, sshFile)
	b, err := os.ReadFile(sshFile)
	require.NoError(t, err)
	const expected = `{
  "Comment": "This file is written to by test_machine_monitor. Do not edit by hand.",
  "user_ip": "root@skia-foobar-01"
}
`
	assert.Equal(t, expected, string(b))
}

func TestRebootDevice_AndroidDeviceAttached_Success(t *testing.T) {

	ctx := executil.FakeTestsContext(
		"Test_FakeExe_AdbReboot_Success",
	)

	m := &Machine{
		adb: adb.New(),
		description: machine.Description{
			Dimensions: machine.SwarmingDimensions{
				machine.DimAndroidDevices: []string{"sprout"},
			},
		},
	}

	require.NoError(t, m.RebootDevice(ctx))
	assert.Equal(t, 1, executil.FakeCommandsReturned(ctx))
}

func TestRebootDevice_AndroidDeviceAttached_ErrOnNonZeroExitCode(t *testing.T) {

	ctx := executil.FakeTestsContext(
		"Test_FakeExe_Reboot_NonZeroExitCode",
		"Test_FakeExe_ReconnectOffline_Success",
		"Test_FakeExe_Reboot_NonZeroExitCode",
	)

	m := &Machine{
		adb: adb.New(),
		description: machine.Description{
			Dimensions: machine.SwarmingDimensions{
				machine.DimAndroidDevices: []string{"sprout"},
			},
		},
	}

	require.Error(t, m.RebootDevice(ctx))
	assert.Equal(t, 3, executil.FakeCommandsReturned(ctx))
}

func TestRebootDevice_NoErrorIfNoDevicesAttached(t *testing.T) {

	ctx := executil.FakeTestsContext() // Any exe call will panic

	m := &Machine{
		description: machine.Description{},
	}

	require.NoError(t, m.RebootDevice(ctx))
}

func TestRebootDevice_IOSDeviceAttached_Success(t *testing.T) {

	ctx := executil.FakeTestsContext("Test_FakeExe_IDeviceDiagnosticsReboot_Success")

	m := &Machine{
		ios: ios.New(),
		description: machine.Description{
			Dimensions: machine.SwarmingDimensions{
				machine.DimOS: []string{"iOS", "iOS-1.2.3"},
			},
		},
	}

	require.NoError(t, m.RebootDevice(ctx))
	assert.Equal(t, 1, executil.FakeCommandsReturned(ctx)) // Make sure it was really called.
}

func TestRebootDevice_ChromeOSDeviceAttached_Success(t *testing.T) {

	ctx := executil.FakeTestsContext(
		"Test_FakeExe_SSHReboot_Success",
	)

	m := &Machine{
		ssh: ssh.ExeImpl{},
		description: machine.Description{
			SSHUserIP: testUserIP,
		},
	}

	require.NoError(t, m.RebootDevice(ctx))
	assert.Equal(t, 1, executil.FakeCommandsReturned(ctx))
}

func TestRebootDevice_ChromeOSDeviceAttached_ErrOnNonZeroExitCode(t *testing.T) {

	ctx := executil.FakeTestsContext(
		"Test_FakeExe_ExitCodeOne",
	)

	m := &Machine{
		ssh: ssh.ExeImpl{},
		description: machine.Description{
			SSHUserIP: testUserIP,
		},
	}

	require.Error(t, m.RebootDevice(ctx))
	assert.Equal(t, 1, executil.FakeCommandsReturned(ctx))
}

func TestInterrogateAndSend_InterrogateSuccessful_EmitsEventViaSink(t *testing.T) {
	ctx := context.Background()

	start := time.Date(2020, time.May, 1, 0, 0, 0, 0, time.UTC)
	expectedEvent := machine.Event{
		EventType: "raw_state",
		Android: machine.Android{
			GetProp:               adbShellGetPropSuccess,
			DumpsysBattery:        adbShellDumpSysBattery,
			DumpsysThermalService: adbShellDumpSysBattery,
			Uptime:                27 * time.Second,
		},
		Host: machine.Host{
			Name:      "my-test-bot-001",
			StartTime: start,
		},
	}

	eventSink := &sinkMocks.Sink{}
	eventSink.On("Send", testutils.AnyContext, expectedEvent).Return(nil)

	desc := machine.NewDescription(ctx)
	desc.AttachedDevice = machine.AttachedDeviceAdb

	// Create a Machine instance.
	m := &Machine{
		httpSink:         eventSink,
		startTime:        start,
		description:      desc,
		adb:              adb.New(),
		interrogateTimer: metrics2.GetFloat64SummaryMetric("bot_config_machine_interrogate_timer", map[string]string{"machine": machineID}),
		MachineID:        "my-test-bot-001",
	}

	ctx = executil.WithFakeTests(ctx,
		"Test_FakeExe_AdbGetState_Success",
		"Test_FakeExe_AdbShellGetUptime_Success",
		"Test_FakeExe_AdbShellGetProp_Success",
		"Test_FakeExe_RawDumpSys_Success",
		"Test_FakeExe_RawDumpSys_Success",
	)

	err := m.interrogateAndSend(ctx)
	require.NoError(t, err)
	eventSink.AssertExpectations(t)
}

func TestInterrogateAndSend_AdbFailsToTalkToDevice_EmptyEventsSentToServer(t *testing.T) {
	ctx := context.Background()

	start := time.Date(2020, time.May, 1, 0, 0, 0, 0, time.UTC)
	expectedEvent := machine.Event{
		EventType: "raw_state",
		Android: machine.Android{
			GetProp:               "",
			DumpsysBattery:        "",
			DumpsysThermalService: "",
		},
		Host: machine.Host{
			Name:      "my-test-bot-001",
			StartTime: start,
		},
	}

	eventSink := &sinkMocks.Sink{}
	eventSink.On("Send", testutils.AnyContext, expectedEvent).Return(nil)

	desc := machine.NewDescription(ctx)
	desc.AttachedDevice = machine.AttachedDeviceAdb

	// Create a Machine instance.
	m := &Machine{
		httpSink:                   eventSink,
		startTime:                  start,
		description:                desc,
		adb:                        adb.New(),
		interrogateTimer:           metrics2.GetFloat64SummaryMetric("bot_config_machine_interrogate_timer", map[string]string{"machine": machineID}),
		interrogateAndSendFailures: metrics2.GetCounter("test_machine_monitor_interrogate_and_send_errors", map[string]string{"machine": "my-test-bot-001"}),
		MachineID:                  "my-test-bot-001",
	}

	ctx = executil.WithFakeTests(ctx,
		"Test_FakeExe_AdbFail",
		"Test_FakeExe_AdbFail",
	)

	err := m.interrogateAndSend(ctx)
	require.NoError(t, err)
	eventSink.AssertExpectations(t)
}

func TestStartInterrogation_TriggerInterrogationChannel_InterrogationIsDone(t *testing.T) {
	ctx := context.Background()

	// Successful calls for a single loop of interrogating an ADB device.
	ctx = executil.WithFakeTests(ctx,
		"Test_FakeExe_AdbGetState_Success",
		"Test_FakeExe_AdbShellGetUptime_Success",
		"Test_FakeExe_AdbShellGetProp_Success",
		"Test_FakeExe_RawDumpSys_Success",
		"Test_FakeExe_RawDumpSys_Success",
	)
	cancelCtx, cancel := context.WithCancel(ctx)

	// Other tests confirm the value being sent is valid, in this case we just
	// want to cancel the context so the startInterrogateLoop exits.
	httpSink := &sinkMocks.Sink{}
	httpSink.On("Send", testutils.AnyContext, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		cancel()
	}).Return(nil)

	desc := machine.NewDescription(ctx)
	desc.AttachedDevice = machine.AttachedDeviceAdb

	// Create a Machine instance.
	triggerInterrogationCh := make(chan bool, 1)
	m := &Machine{
		httpSink:               httpSink,
		description:            desc,
		adb:                    adb.New(),
		interrogateTimer:       metrics2.GetFloat64SummaryMetric("bot_config_machine_interrogate_timer", map[string]string{"machine": machineID}),
		MachineID:              "my-test-bot-001",
		triggerInterrogationCh: triggerInterrogationCh,
	}

	// trigger an interrogation right away.
	triggerInterrogationCh <- true

	m.startInterrogateLoop(cancelCtx)

	// The test will fail by timeout if startInterrogateLoop fails to exit on
	// context cancellation.
}

func Test_FakeExe_AdbShellGetUptime_Success(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "shell", "cat", "/proc/uptime"}, args)

	fmt.Print(adbShellGetUptimeSuccess)
	os.Exit(0)
}

func Test_FakeExe_AdbState_Success(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "get-state"}, args)

	fmt.Print(adbGetStateSuccess)
	os.Exit(0)
}

func Test_FakeExe_AdbShellGetProp_Success(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "shell", "getprop"}, args)

	fmt.Print(adbShellGetPropSuccess)
	os.Exit(0)
}

func Test_FakeExe_RawDumpSys_Success(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	fmt.Print(adbShellDumpSysBattery)
	os.Exit(0)
}

func Test_FakeExe_ADBUptime_ReturnsPlaceholder(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}
	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "shell", "cat", "/proc/uptime"}, args)

	fmt.Print(adbUptimePlaceholder)
	os.Exit(0)
}

func Test_FakeExe_AdbShellGetProp_ReturnsPlaceholder(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "shell", "getprop"}, args)

	fmt.Print(getPropPlaceholder)
	os.Exit(0)
}

func Test_FakeExe_RawDumpSysBattery_ReturnsPlaceholder(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "shell", "dumpsys", "battery"}, args)

	fmt.Print(dumpSysBatteryPlaceholder)
	os.Exit(0)
}

func Test_FakeExe_RawDumpSysThermal_ReturnsPlaceholder(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}
	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "shell", "dumpsys", "thermalservice"}, args)

	fmt.Print(dumpSysThermalPlaceholder)
	os.Exit(0)
}

func Test_FakeExe_IDeviceInfo_ReturnsDeviceType(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}
	require.Equal(t, []string{"ideviceinfo", "-k", "ProductType"}, executil.OriginalArgs())

	fmt.Fprintln(os.Stderr, iOSDeviceTypePlaceholder)
	os.Exit(0)
}

func Test_FakeExe_IDeviceInfo_ReturnsOSVersion(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}
	require.Equal(t, []string{"ideviceinfo", "-k", "ProductVersion"}, executil.OriginalArgs())

	fmt.Fprintln(os.Stderr, iOSVersionPlaceholder)
	os.Exit(0)
}

func Test_FakeExe_IDeviceInfo_ReturnsGoodBatteryLevel(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}
	require.Equal(t, []string{"ideviceinfo", "--domain", "com.apple.mobile.battery", "-k", "BatteryCurrentCapacity"}, executil.OriginalArgs())

	fmt.Fprintln(os.Stderr, "33")
	os.Exit(0)
}

func Test_FakeExe_IDeviceDiagnosticsReboot_Success(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}
	require.Equal(t, []string{"idevicediagnostics", "restart"}, executil.OriginalArgs())
	os.Exit(0)
}

func Test_FakeExe_SSHLSBRelease_ReturnsPlaceholder(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
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
	if !executil.IsCallingFakeCommand() {
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
	if !executil.IsCallingFakeCommand() {
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
	if !executil.IsCallingFakeCommand() {
		return
	}

	os.Exit(1)
}

func Test_FakeExe_AdbGetState_Success(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"adb", "get-state"}, args)
	_, _ = fmt.Fprintf(os.Stderr, "device")

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

	_, _ = fmt.Fprintf(os.Stderr, "error: no devices/emulators found")

	os.Exit(127)
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

func Test_FakeExe_SSHReboot_Success(t *testing.T) {
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Contains(t, args, "ssh")
	require.Contains(t, args, testUserIP)
	require.Contains(t, args, "reboot")

	// Force exit so we don't get PASS in the output.
	os.Exit(0)
}

// setupLocalServerWithCallback sets up a local HTTP server where the provided
// 'cb' function will be used to serve all requests. A *url.URL is returned with
// the scheme and host configured to point at the local HTTP server.
func setupLocalServerWithCallback(t *testing.T, cb http.HandlerFunc) (*url.URL, *bool, *http.Client) {
	t.Helper()
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cb(w, r)
		called = true
	}))
	t.Cleanup(func() {
		ts.Close()
	})
	u, err := url.Parse(ts.URL)
	u.Path = urlExpansionRegex.ReplaceAllLiteralString(rpc.MachineDescriptionURL, machineID)
	require.NoError(t, err)

	httpClient := httputils.DefaultClientConfig().With2xxOnly().WithoutRetries().Client()
	return u, &called, httpClient
}

func TestRetrieveDescription_EndpointReturnsError_DescriptionIsNotUpdated(t *testing.T) {
	u, called, client := setupLocalServerWithCallback(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	})

	m := &Machine{
		client:                client,
		machineDescriptionURL: u.String(),
	}
	err := m.retrieveDescription(context.Background())
	require.Error(t, err)
	require.True(t, *called)
	require.Equal(t, machine.Description{}, m.description)
}

func TestRetrieveDescription_EndpointReturnsInvalidJSON_DescriptionIsNotUpdated(t *testing.T) {
	u, called, client := setupLocalServerWithCallback(t, func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprint(w, "} not valid json {")
		require.NoError(t, err)
	})

	m := &Machine{
		client:                         client,
		machineDescriptionURL:          u.String(),
		descriptionWatchArrivalCounter: metrics2.GetCounter("bot_config_machine_description_watch_arrival", map[string]string{"machine": machineID}),
	}
	m.descriptionWatchArrivalCounter.Reset()

	err := m.retrieveDescription(context.Background())
	require.Error(t, err)
	require.True(t, *called)
	require.Equal(t, machine.Description{}, m.description)
	require.Equal(t, int64(0), m.descriptionWatchArrivalCounter.Get())
}

func TestRetrieveDescription_EndpointReturnsNewDescription_DescriptionIsUpdated(t *testing.T) {
	desc := machine.Description{}
	var capturedRequest *http.Request
	u, called, client := setupLocalServerWithCallback(t, func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r
		err := json.NewEncoder(w).Encode(desc)
		require.NoError(t, err)
	})

	m := &Machine{
		client:                         client,
		machineDescriptionURL:          u.String(),
		descriptionWatchArrivalCounter: metrics2.GetCounter("bot_config_machine_description_watch_arrival", map[string]string{"machine": machineID}),
	}
	m.descriptionWatchArrivalCounter.Reset()

	err := m.retrieveDescription(context.Background())
	require.NoError(t, err)
	require.Equal(t, u.Path, capturedRequest.URL.Path)
	require.True(t, *called)
	require.Equal(t, desc, m.description)
	require.Equal(t, int64(1), m.descriptionWatchArrivalCounter.Get())
}

func TestStartDescriptionWatch_ChannelIsClosed_FunctionExits(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context so startDescriptionWatch will return.
	cancel()
	ch := make(chan interface{})
	var readOnlyCh <-chan interface{} = ch
	changeSource := &mocks.Source{}
	changeSource.On("Start", testutils.AnyContext).Return(readOnlyCh)

	m := &Machine{
		sseChangeSource: changeSource,
	}
	m.startDescriptionWatch(ctx)
	close(ch)

	// Test will never exit on failure.
}

func getMachineWithHomeDir(t *testing.T) *Machine {
	return &Machine{
		MachineID: machineID,
		homeDir:   t.TempDir(),
		description: machine.Description{
			AttachedDevice: machine.AttachedDeviceNone,
		},
		interrogateTimer: metrics2.GetFloat64SummaryMetric("just_a_test"),
	}
}

func writeQuarantineFile(m *Machine, t *testing.T) string {
	// Write quarantine file.
	quarantineFile := filepath.Join(m.homeDir, fmt.Sprintf("%s.force_quarantine", machineID))
	err := os.WriteFile(quarantineFile, []byte("test"), 0666)
	require.NoError(t, err)

	// Test the test, confirm the file exists.
	_, err = os.Stat(quarantineFile)
	require.NoError(t, err)
	return quarantineFile
}

func TestMachineCheckForForcedQuarantine_FileExists_FileIsDeletedAndReturnsTrue(t *testing.T) {
	m := getMachineWithHomeDir(t)

	filename := writeQuarantineFile(m, t)
	require.True(t, m.checkForForcedQuarantine())

	// Confirm the file was removed.
	_, err := os.Stat(filepath.Join(m.homeDir, filename))
	require.True(t, os.IsNotExist(err))
}

func TestMachineCheckForForcedQuarantine_FileDoesNotExists_ReturnsFalse(t *testing.T) {
	m := getMachineWithHomeDir(t)
	require.False(t, m.checkForForcedQuarantine())
}

func TestInterrogate_ForceQuarantineFileDoesNotExists_ReturnsEventWithForceQuarantineTrue(t *testing.T) {
	m := getMachineWithHomeDir(t)
	event, err := m.interrogate(context.Background())
	require.NoError(t, err)
	require.False(t, event.ForcedQuarantine)
}

func TestInterrogate_ForceQuarantineFileExists_ReturnsEventWithForceQuarantineTrue(t *testing.T) {
	m := getMachineWithHomeDir(t)
	_ = writeQuarantineFile(m, t)
	event, err := m.interrogate(context.Background())
	require.NoError(t, err)
	require.True(t, event.ForcedQuarantine)
}

func TestIsAvailable_NilMachine_ReturnsFalse(t *testing.T) {
	var m *Machine
	require.False(t, m.IsAvailable())
}

func TestIsAvailable_AvailableMachine_ReturnsTrue(t *testing.T) {
	m := &Machine{
		description: machine.Description{
			MaintenanceMode: "",
			IsQuarantined:   false,
			Recovering:      "",
		},
	}
	require.True(t, m.IsAvailable())
}
