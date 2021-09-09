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
)

const (
	// For example values, see adb_test.go
	getPropPlaceholder        = "Placeholder get prop response"
	dumpSysBatteryPlaceholder = "Placeholder dumpsys battery response"
	dumpSysThermalPlaceholder = "Placeholder dumpsys thermal response"
	// This formatting matters because it is processed in adb.go
	adbUptimePlaceholder = "123.4 567.8"
)

func TestTryInterrogatingAndroidDevice_DeviceAttached_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_Uptime_ReturnsPlaceholder",
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
		"Test_FakeExe_Uptime_ReturnsPlaceholder",
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

func TestInterrogate_NoDeviceAttached_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_ExitCodeOne", // No android device
	)

	m := &Machine{
		adb:              adb.New(),
		MachineID:        "some-machine",
		Hostname:         "some-hostname",
		KubernetesImage:  "deprecated",
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
			Name:            "some-machine",
			PodName:         "some-hostname",
			KubernetesImage: "deprecated",
			Version:         "some-version",
			StartTime:       time.Date(2021, time.September, 2, 2, 2, 2, 2, time.UTC),
		},
	}, actual)
}

func TestInterrogate_AndroidDeviceAttached_Success(t *testing.T) {
	unittest.SmallTest(t)
	ctx := executil.FakeTestsContext(
		"Test_FakeExe_Uptime_ReturnsPlaceholder",
		"Test_FakeExe_AdbShellGetProp_ReturnsPlaceholder",
		"Test_FakeExe_RawDumpSysBattery_ReturnsPlaceholder",
		"Test_FakeExe_RawDumpSysThermal_ReturnsPlaceholder",
	)

	m := &Machine{
		adb:              adb.New(),
		MachineID:        "some-machine",
		Hostname:         "some-hostname",
		KubernetesImage:  "deprecated",
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
			Name:            "some-machine",
			PodName:         "some-hostname",
			KubernetesImage: "deprecated",
			Version:         "some-version",
			StartTime:       time.Date(2021, time.September, 2, 2, 2, 2, 2, time.UTC),
		},
		Android: machine.Android{
			GetProp:               getPropPlaceholder,
			DumpsysBattery:        dumpSysBatteryPlaceholder,
			DumpsysThermalService: dumpSysThermalPlaceholder,
			Uptime:                123 * time.Second,
		},
	}, actual)
}

func Test_FakeExe_Uptime_ReturnsPlaceholder(t *testing.T) {
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

func Test_FakeExe_ExitCodeOne(t *testing.T) {
	unittest.FakeExeTest(t)
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
		return
	}

	os.Exit(1)
}
