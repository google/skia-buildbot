package processor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/machine"
)

const (
	myTestVersion = "2021-07-22-jcgregorio-78bcc725fef1e29b518291469b8ad8f0cc3b21e4"
)

func TestParseAndroidProperties_HappyPath(t *testing.T) {
	unittest.SmallTest(t)

	const adbResponseHappyPath = `
[ro.product.manufacturer]: [asus]
[ro.product.model]: [Nexus 7]
[ro.product.name]: [razor]
`
	want := map[string]string{
		"ro.product.manufacturer": "asus",
		"ro.product.model":        "Nexus 7",
		"ro.product.name":         "razor",
	}
	got := parseAndroidProperties(adbResponseHappyPath)
	assert.Equal(t, want, got)
}

func TestParseAndroidProperties_EmptyStringGivesEmptyMap(t *testing.T) {
	unittest.SmallTest(t)

	assert.Empty(t, parseAndroidProperties(""))
}

func TestDimensionsFromAndroidProperties_Success(t *testing.T) {
	unittest.SmallTest(t)

	adbResponse := strings.Join([]string{
		"[ro.product.manufacturer]: [Google]",      // Ignored
		"[ro.product.model]: [Pixel 3a]",           // Ignored
		"[ro.build.id]: [QQ2A.200305.002]",         // device_os
		"[ro.product.brand]: [google]",             // device_os_flavor
		"[ro.build.type]: [user]",                  // device_os_type
		"[ro.product.board]: []",                   // Ignore empty values.
		"[ro.product.device]: [4560MMX_sprout]",    // device_type
		"[ro.build.product]: [sargo]",              // device_type
		"[ro.product.system.brand]: [google]",      // device_os_flavor (dup should be ignored)
		"[ro.product.system.brand]: [aosp]",        // device_os_flavor (should be converted to "android")
		"[ro.product.name]: [aosp_sunfish_hwasan]", // android_hwasan_build
	}, "\n")

	dimensions := parseAndroidProperties(adbResponse)
	got := dimensionsFromAndroidProperties(dimensions)

	expected := map[string][]string{
		"android_devices":      {"1"},
		"android_hwasan_build": {"1"},
		"device_os":            {"Q", "QQ2A.200305.002"},
		"device_os_flavor":     {"google", "android"},
		"device_os_type":       {"user"},
		machine.DimDeviceType:  {"4560MMX_sprout", "sargo"},
		machine.DimOS:          {"Android"},
	}
	assert.Equal(t, expected, got)
}

func TestDimensionsFromAndroidProperties_AppendIncrementalBuildToDeviceOS(t *testing.T) {
	unittest.SmallTest(t)

	adbResponse := strings.Join([]string{
		"[ro.build.id]: [QQ2A.200305.002]",          // device_os
		"[ro.build.version.incremental]: [6254899]", // device_os additional data.
	}, "\n")

	dimensions := parseAndroidProperties(adbResponse)
	got := dimensionsFromAndroidProperties(dimensions)

	expected := map[string][]string{
		"android_devices": {"1"},
		"device_os":       {"Q", "QQ2A.200305.002", "QQ2A.200305.002_6254899"},
		machine.DimOS:     {"Android"},
	}
	assert.Equal(t, expected, got)
}

func TestDimensionsFromAndroidProperties_EmptyFromEmpty(t *testing.T) {
	unittest.SmallTest(t)

	dimensions := parseAndroidProperties("")
	assert.Empty(t, dimensionsFromAndroidProperties(dimensions))
}

func newProcessorForTest() *ProcessorImpl {
	p := New(context.Background())
	p.eventsProcessedCount.Reset()
	p.unknownEventTypeCount.Reset()
	return p
}

func TestProcess_DetectBadEventType(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()
	event := machine.Event{
		EventType: machine.EventType(""),
	}
	previous := machine.Description{}
	p := newProcessorForTest()
	_ = p.Process(ctx, previous, event)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(1), p.unknownEventTypeCount.Get())
}

func TestProcess_SwarmingTaskIsRunning(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()
	event := machine.Event{
		EventType:           machine.EventTypeRawState,
		RunningSwarmingTask: true,
	}
	previous := machine.Description{}
	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	assert.True(t, next.RunningSwarmingTask)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(0), p.unknownEventTypeCount.Get())
}

func TestProcess_LaunchedSwarmingIsTrueInEvent_LaunchedSwarmingIsTrueInDescription(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()
	event := machine.Event{
		EventType:        machine.EventTypeRawState,
		LaunchedSwarming: true,
	}
	previous := machine.Description{}
	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	assert.True(t, next.LaunchedSwarming)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(0), p.unknownEventTypeCount.Get())
}

func TestProcess_NewDeviceAttached(t *testing.T) {
	unittest.SmallTest(t)

	stateTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	bootUpTime := time.Date(2021, time.September, 1, 10, 1, 0, 0, time.UTC)
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)

	ctx := now.TimeTravelingContext(stateTime)

	// The current machine has nothing attached.
	previous := machine.NewDescription(ctx)
	previous.AttachedDevice = machine.AttachedDeviceAdb
	require.Empty(t, previous.Dimensions)
	const uptime = int32(5)

	// An event arrives with the attachment of an Android device.
	props := strings.Join([]string{
		"[ro.build.id]: [QQ2A.200305.002]",  // device_os
		"[ro.product.brand]: [google]",      // device_os_flavor
		"[ro.build.type]: [user]",           // device_os_type
		"[ro.build.product]: [sargo]",       // device_type
		"[ro.product.system.brand]: [aosp]", // device_os_flavor
	}, "\n")
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Android: machine.Android{
			GetProp: props,
			Uptime:  time.Duration(int64(uptime) * int64(time.Second)),
		},
		Host: machine.Host{
			Name:      "skia-rpi2-0001",
			Version:   myTestVersion,
			StartTime: bootUpTime,
		},
	}

	ctx.SetTime(serverTime)
	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(0), p.unknownEventTypeCount.Get())

	// The Android device should be reflected in the returned Dimensions.
	assert.Equal(t, machine.Description{
		AttachedDevice: machine.AttachedDeviceAdb,
		Mode:           machine.ModeAvailable,
		LastUpdated:    serverTime,
		Dimensions: machine.SwarmingDimensions{
			"android_devices":     []string{"1"},
			"device_os":           []string{"Q", "QQ2A.200305.002"},
			"device_os_flavor":    []string{"google", "android"},
			"device_os_type":      []string{"user"},
			machine.DimDeviceType: []string{"sargo"},
			machine.DimOS:         []string{"Android"},
			machine.DimID:         []string{"skia-rpi2-0001"},
		},
		SuppliedDimensions: machine.SwarmingDimensions{},
		Battery:            machine.BadBatteryLevel,
		Version:            myTestVersion,
		DeviceUptime:       5,
	}, next)
}

func TestProcess_DetectNotInsideDocker(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	// The current machine has nothing attached.
	previous := machine.NewDescription(ctx)
	require.Empty(t, previous.Dimensions)

	// An event arrives with the attachment of an Android device.
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Android:   machine.Android{},
		Host: machine.Host{
			Name: "skia-rpi-0001",
		},
	}

	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(0), p.unknownEventTypeCount.Get())

	// The Android device should be reflected in the returned Dimensions.
	expected := machine.SwarmingDimensions{
		machine.DimID: []string{"skia-rpi-0001"},
	}
	assert.Equal(t, expected, next.Dimensions)
	assert.Equal(t, machine.ModeAvailable, next.Mode)
}

func TestProcess_DeviceGoingMissingMeansQuarantine(t *testing.T) {
	unittest.SmallTest(t)

	stateTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	bootUpTime := time.Date(2021, time.September, 1, 10, 1, 0, 0, time.UTC)
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)

	ctx := now.TimeTravelingContext(stateTime)

	// The current machine has a device attached.
	previous := machine.NewDescription(ctx)
	previous.AttachedDevice = machine.AttachedDeviceAdb
	previous.Dimensions = machine.SwarmingDimensions{
		"android_devices":     []string{"1"},
		"device_os":           []string{"Q", "QQ2A.200305.002"},
		"device_os_flavor":    []string{"google", "android"},
		"device_os_type":      []string{"user"},
		machine.DimDeviceType: []string{"sargo"},
		machine.DimOS:         []string{"Android"},
		machine.DimID:         []string{"skia-rpi2-0001"},
	}

	// An event arrives without any device info.
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name:      "skia-rpi2-0001",
			StartTime: bootUpTime,
		},
	}

	// The dimensions should not change, except for the addition of the
	// quarantine message, which tells swarming to quarantine this machine.
	expectedDims := previous.Dimensions.Copy()
	expectedDims[machine.DimQuarantined] = []string{`Device ["sargo"] has gone missing`}

	ctx.SetTime(serverTime)
	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(0), p.unknownEventTypeCount.Get())

	assert.Equal(t, machine.Description{
		AttachedDevice:     machine.AttachedDeviceAdb,
		Mode:               machine.ModeAvailable,
		Dimensions:         expectedDims,
		SuppliedDimensions: machine.SwarmingDimensions{},
		LastUpdated:        serverTime,
	}, next)
}

func TestProcess_DoNotQuarantineDevicesInMaintenanceMode(t *testing.T) {
	unittest.SmallTest(t)

	stateTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	bootUpTime := time.Date(2021, time.September, 1, 10, 1, 0, 0, time.UTC)
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)

	ctx := now.TimeTravelingContext(stateTime)

	// The current machine has a device attached.
	previous := machine.NewDescription(ctx)
	previous.Dimensions = machine.SwarmingDimensions{
		"android_devices":     []string{"1"},
		"device_os":           []string{"Q", "QQ2A.200305.002"},
		"device_os_flavor":    []string{"google", "android"},
		"device_os_type":      []string{"user"},
		machine.DimDeviceType: []string{"sargo"},
		machine.DimOS:         []string{"Android"},
		machine.DimID:         []string{"skia-rpi2-0001"},
	}
	previous.Mode = machine.ModeMaintenance

	// An event arrives with the device still attached.
	props := strings.Join([]string{
		"[ro.build.id]: [QQ2A.200305.002]",  // device_os
		"[ro.product.brand]: [google]",      // device_os_flavor
		"[ro.build.type]: [user]",           // device_os_type
		"[ro.build.product]: [sargo]",       // device_type
		"[ro.product.system.brand]: [aosp]", // device_os_flavor
	}, "\n")
	// An event arrives without any device info.
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name:      "skia-rpi2-0001",
			StartTime: bootUpTime,
		},
		Android: machine.Android{
			GetProp: props,
		},
	}
	// The dimensions should not change.
	expected := previous.Dimensions.Copy()

	ctx.SetTime(serverTime)
	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(0), p.unknownEventTypeCount.Get())

	assert.Equal(t, expected, next.Dimensions)
	assert.Equal(t, machine.ModeMaintenance, next.Mode)
	assert.Equal(t, int64(0), metrics2.GetInt64Metric("machine_processor_device_quarantined", next.Dimensions.AsMetricsTags()).Get())
}

func TestProcess_RemoveMachineFromQuarantineIfDeviceReturns(t *testing.T) {
	unittest.SmallTest(t)

	stateTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	bootUpTime := time.Date(2021, time.September, 1, 10, 1, 0, 0, time.UTC)
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)

	ctx := now.TimeTravelingContext(stateTime)

	// The current machine has been quarantined because the device went missing.
	previous := machine.NewDescription(ctx)
	previous.AttachedDevice = machine.AttachedDeviceAdb
	previous.Dimensions = machine.SwarmingDimensions{
		"android_devices":      []string{"1"},
		"device_os":            []string{"Q", "QQ2A.200305.002"},
		"device_os_flavor":     []string{"google", "android"},
		"device_os_type":       []string{"user"},
		machine.DimDeviceType:  []string{"sargo"},
		machine.DimOS:          []string{"Android"},
		machine.DimQuarantined: []string{"Device [\"sargo\"] has gone missing"},
		machine.DimID:          []string{"skia-rpi2-0001"},
	}

	// An event arrives with the device restored.
	props := strings.Join([]string{
		"[ro.build.id]: [QQ2A.200305.002]",  // device_os
		"[ro.product.brand]: [google]",      // device_os_flavor
		"[ro.build.type]: [user]",           // device_os_type
		"[ro.build.product]: [sargo]",       // device_type
		"[ro.product.system.brand]: [aosp]", // device_os_flavor
	}, "\n")
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Android: machine.Android{
			GetProp: props,
			Uptime:  10,
		},
		Host: machine.Host{
			StartTime: bootUpTime,
			Name:      "skia-rpi2-0001",
		},
	}

	expectedDims := previous.Dimensions.Copy()
	// The machine should no longer be quarantined via dimensions.
	delete(expectedDims, machine.DimQuarantined)

	ctx.SetTime(serverTime)
	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(0), p.unknownEventTypeCount.Get())

	assert.Equal(t, machine.Description{
		AttachedDevice:     machine.AttachedDeviceAdb,
		Mode:               machine.ModeAvailable,
		Dimensions:         expectedDims,
		SuppliedDimensions: machine.SwarmingDimensions{},
		LastUpdated:        serverTime,
		Battery:            machine.BadBatteryLevel,
	}, next)

	assert.Equal(t, int64(0), metrics2.GetInt64Metric("machine_processor_device_quarantined", next.Dimensions.AsMetricsTags()).Get())
}

func TestProcess_RecoveryModeIfDeviceBatteryTooLow(t *testing.T) {
	unittest.SmallTest(t)

	stateTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	bootUpTime := time.Date(2021, time.September, 1, 10, 1, 0, 0, time.UTC)
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)

	ctx := now.TimeTravelingContext(stateTime)

	previous := machine.NewDescription(ctx)
	previous.AttachedDevice = machine.AttachedDeviceAdb
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name:      "skia-rpi2-0001",
			StartTime: bootUpTime,
		},
		Android: machine.Android{
			Uptime: 10,
			DumpsysBattery: `Current Battery Service state:
  AC powered: true
  USB powered: false
  Wireless powered: false
  Max charging current: 1500000
  Max charging voltage: 5000000
  Charge counter: 2448973
  status: 2
  health: 2
  present: true
  level: 9
  scale: 100
  voltage: 4248`,
		},
	}

	ctx.SetTime(serverTime)
	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	assert.Equal(t, machine.Description{
		AttachedDevice: machine.AttachedDeviceAdb,
		Mode:           machine.ModeRecovery,
		Annotation: machine.Annotation{
			Message:   "Battery low. ",
			User:      machineUserName,
			Timestamp: serverTime,
		},
		Dimensions: machine.SwarmingDimensions{
			machine.DimID: []string{"skia-rpi2-0001"},
		},
		SuppliedDimensions: machine.SwarmingDimensions{},
		Battery:            9,
		RecoveryStart:      serverTime,
		LastUpdated:        serverTime,
	}, next)

	assert.Equal(t, int64(9), metrics2.GetInt64Metric("machine_processor_device_battery_level", map[string]string{"machine": "skia-rpi2-0001"}).Get())
	assert.Equal(t, int64(1), metrics2.GetInt64Metric("machine_processor_device_maintenance", next.Dimensions.AsMetricsTags()).Get())
	assert.Equal(t, int64(0), metrics2.GetInt64Metric("machine_processor_device_time_in_recovery_mode_s", next.Dimensions.AsMetricsTags()).Get())
}

func TestProcess_DeviceStillInRecoveryMode_MetricReportsTimeInRecovery(t *testing.T) {
	unittest.SmallTest(t)

	startRecoveryTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	bootUpTime := time.Date(2021, time.September, 1, 10, 1, 0, 0, time.UTC)
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)

	ctx := now.TimeTravelingContext(serverTime)

	previous := machine.NewDescription(ctx)
	previous.RecoveryStart = startRecoveryTime
	previous.Mode = machine.ModeRecovery

	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name:      "skia-rpi2-0001",
			StartTime: bootUpTime,
		},
		Android: machine.Android{
			Uptime: 10,
			DumpsysBattery: `Current Battery Service state:
  AC powered: true
  USB powered: false
  Wireless powered: false
  Max charging current: 1500000
  Max charging voltage: 5000000
  Charge counter: 2448973
  status: 2
  health: 2
  present: true
  level: 9
  scale: 100
  voltage: 4248`,
		},
	}

	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)

	assert.Equal(t, int64(serverTime.Sub(startRecoveryTime).Seconds()), metrics2.GetInt64Metric("machine_processor_device_time_in_recovery_mode_s", next.Dimensions.AsMetricsTags()).Get())
}

func TestProcess_RecoveryModeIfDeviceTooHot(t *testing.T) {
	unittest.SmallTest(t)

	stateTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	bootUpTime := time.Date(2021, time.September, 1, 10, 1, 0, 0, time.UTC)
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)

	ctx := now.TimeTravelingContext(stateTime)

	previous := machine.NewDescription(ctx)
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name:      "skia-rpi2-0001",
			StartTime: bootUpTime,
		},
		Android: machine.Android{
			Uptime: 10,
			DumpsysThermalService: `IsStatusOverride: false
ThermalEventListeners:
	callbacks: 3
	killed: false
	broadcasts count: -1
ThermalStatusListeners:
	callbacks: 1
	killed: false
	broadcasts count: -1
Thermal Status: 0
Cached temperatures:
	Temperature{mValue=32.401, mType=3, mName=mb-therm-monitor, mStatus=0}
	Temperature{mValue=46.100002, mType=0, mName=cpu0-silver-usr, mStatus=0}
	Temperature{mValue=44.800003, mType=0, mName=cpu1-silver-usr, mStatus=0}
	Temperature{mValue=45.100002, mType=0, mName=cpu2-silver-usr, mStatus=0}
	Temperature{mValue=40.600002, mType=1, mName=gpu0-usr, mStatus=0}
	Temperature{mValue=40.300003, mType=1, mName=gpu1-usr, mStatus=0}
	Temperature{mValue=44.100002, mType=0, mName=cpu3-silver-usr, mStatus=0}
	Temperature{mValue=45.4, mType=0, mName=cpu4-silver-usr, mStatus=0}
	Temperature{mValue=45.4, mType=0, mName=cpu5-silver-usr, mStatus=0}
	Temperature{mValue=30.000002, mType=2, mName=battery, mStatus=0}
	Temperature{mValue=48.300003, mType=0, mName=cpu1-gold-usr, mStatus=0}
	Temperature{mValue=46.7, mType=0, mName=cpu0-gold-usr, mStatus=0}
	Temperature{mValue=27.522001, mType=4, mName=usbc-therm-monitor, mStatus=0}
HAL Ready: true
HAL connection:
	ThermalHAL 2.0 connected: yes
Current temperatures from HAL:
	Temperature{mValue=28.000002, mType=2, mName=battery, mStatus=0}
	Temperature{mValue=33.800003, mType=0, mName=cpu0-gold-usr, mStatus=0}
	Temperature{mValue=33.800003, mType=0, mName=cpu0-silver-usr, mStatus=0}
	Temperature{mValue=33.5, mType=0, mName=cpu1-gold-usr, mStatus=0}
	Temperature{mValue=44.1, mType=0, mName=cpu1-silver-usr, mStatus=0}
	Temperature{mValue=43.8, mType=0, mName=cpu2-silver-usr, mStatus=0}
	Temperature{mValue=33.5, mType=0, mName=cpu3-silver-usr, mStatus=0}
	Temperature{mValue=33.5, mType=0, mName=cpu4-silver-usr, mStatus=0}
	Temperature{mValue=33.5, mType=0, mName=cpu5-silver-usr, mStatus=0}
	Temperature{mValue=32.9, mType=1, mName=gpu0-usr, mStatus=0}
	Temperature{mValue=32.9, mType=1, mName=gpu1-usr, mStatus=0}
	Temperature{mValue=30.147001, mType=3, mName=mb-therm-monitor, mStatus=0}
	Temperature{mValue=26.926, mType=4, mName=usbc-therm-monitor, mStatus=0}
Current cooling devices from HAL:
	CoolingDevice{mValue=0, mType=1, mName=battery}
	CoolingDevice{mValue=0, mType=2, mName=thermal-cpufreq-0}
	CoolingDevice{mValue=0, mType=2, mName=thermal-cpufreq-6}
	CoolingDevice{mValue=0, mType=3, mName=thermal-devfreq-0}`,
		},
	}

	ctx.SetTime(serverTime)
	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	assert.Equal(t, "Too hot. ", next.Annotation.Message)
	assert.Equal(t, machineUserName, next.Annotation.User)
	assert.Equal(t, machine.ModeRecovery, next.Mode)
	assert.Empty(t, next.Dimensions[machine.DimQuarantined])

	assert.Equal(t, float64(44.1), metrics2.GetFloat64Metric("machine_processor_device_temperature_c", map[string]string{"machine": "skia-rpi2-0001", "sensor": "cpu1-silver-usr"}).Get())
	assert.Equal(t, int64(1), metrics2.GetInt64Metric("machine_processor_device_maintenance", next.Dimensions.AsMetricsTags()).Get())
	assert.Equal(t, int64(0), metrics2.GetInt64Metric("machine_processor_device_time_in_recovery_mode_s", next.Dimensions.AsMetricsTags()).Get())

}

func TestProcess_HandleTempsInMilliCentgrade(t *testing.T) {

	unittest.SmallTest(t)

	stateTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	bootUpTime := time.Date(2021, time.September, 1, 10, 1, 0, 0, time.UTC)
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)

	ctx := now.TimeTravelingContext(stateTime)

	const dumpsysBattery = `IsStatusOverride: false
ThermalEventListeners:
	callbacks: 8
	killed: false
	broadcasts count: -1
ThermalStatusListeners:
	callbacks: 2
	killed: false
	broadcasts count: -1
Thermal Status: 0
Cached temperatures:
	Temperature{mValue=1300.0, mType=6, mName=smpl_gm, mStatus=0}
	Temperature{mValue=55.000004, mType=0, mName=LITTLE, mStatus=0}
	Temperature{mValue=0.0, mType=6, mName=critical-battery-cell, mStatus=0}
	Temperature{mValue=11900.0, mType=7, mName=ocp_gpu, mStatus=0}
	Temperature{mValue=10400.0, mType=7, mName=ocp_tpu, mStatus=0}
	Temperature{mValue=20.623001, mType=-1, mName=neutral_therm, mStatus=0}
	Temperature{mValue=0.0, mType=4, mName=VIRTUAL-USB-UI, mStatus=0}
	Temperature{mValue=8900.0, mType=7, mName=soft_ocp_gpu, mStatus=0}
	Temperature{mValue=8400.0, mType=7, mName=soft_ocp_tpu, mStatus=0}
	Temperature{mValue=21.423, mType=-1, mName=disp_therm, mStatus=0}
	Temperature{mValue=-1.8060001, mType=-1, mName=USB2-MINUS-QI, mStatus=0}
	Temperature{mValue=0.0, mType=-1, mName=FLASH_LED_REDUCE, mStatus=0}
	Temperature{mValue=19.324453, mType=-1, mName=VIRTUAL-QUIET-BATT, mStatus=0}
	Temperature{mValue=6900.0, mType=7, mName=soft_ocp_cpu1, mStatus=0}
	Temperature{mValue=8900.0, mType=7, mName=soft_ocp_cpu2, mStatus=0}
	Temperature{mValue=0.0, mType=6, mName=battery_cycle, mStatus=0}
	Temperature{mValue=20.723001, mType=-1, mName=quiet_therm, mStatus=0}
	Temperature{mValue=22.771002, mType=-1, mName=VIRTUAL-SKIN-CHARGE, mStatus=0}
	Temperature{mValue=4900.0, mType=7, mName=batoilo, mStatus=0}
	Temperature{mValue=20.2, mType=2, mName=battery, mStatus=0}
	Temperature{mValue=61.000004, mType=0, mName=BIG, mStatus=0}
	Temperature{mValue=36.0, mType=1, mName=G3D, mStatus=0}
	Temperature{mValue=52.000004, mType=0, mName=MID, mStatus=0}
	Temperature{mValue=37.0, mType=9, mName=TPU, mStatus=0}
	Temperature{mValue=19.0, mType=8, mName=soc, mStatus=0}
	Temperature{mValue=20.473001, mType=-1, mName=usb_pwr_therm2, mStatus=0}
	Temperature{mValue=1050.0, mType=6, mName=vdroop1, mStatus=0}
	Temperature{mValue=1250.0, mType=6, mName=vdroop2, mStatus=0}
	Temperature{mValue=22.279001, mType=-1, mName=qi_therm, mStatus=0}
	Temperature{mValue=-0.05, mType=-1, mName=USB2-MINUS-USB, mStatus=0}
	Temperature{mValue=6900.0, mType=7, mName=ocp_cpu1, mStatus=0}
	Temperature{mValue=11900.0, mType=7, mName=ocp_cpu2, mStatus=0}
	Temperature{mValue=22.771002, mType=5, mName=cellular-emergency, mStatus=0}
	Temperature{mValue=22.841002, mType=-1, mName=gnss_tcxo_therm, mStatus=0}
	Temperature{mValue=22.771002, mType=3, mName=VIRTUAL-SKIN, mStatus=0}
	Temperature{mValue=20.523, mType=-1, mName=usb_pwr_therm, mStatus=0}
	Temperature{mValue=0.0, mType=4, mName=VIRTUAL-USB-THROTTLING, mStatus=0}
	Temperature{mValue=20.15738, mType=-1, mName=VIRTUAL-QI-BATT, mStatus=0}
	Temperature{mValue=18.2005, mType=-1, mName=VIRTUAL-QI-GNSS, mStatus=0}
	Temperature{mValue=22.771002, mType=-1, mName=VIRTUAL-USB2-DISP, mStatus=0}
HAL Ready: true
HAL connection:
	ThermalHAL 2.0 connected: yes
Current temperatures from HAL:
	Temperature{mValue=8900.0, mType=7, mName=soft_ocp_gpu, mStatus=0}
	Temperature{mValue=8400.0, mType=7, mName=soft_ocp_tpu, mStatus=0}
	Temperature{mValue=25.000002, mType=9, mName=TPU, mStatus=0}
	Temperature{mValue=8900.0, mType=7, mName=soft_ocp_cpu2, mStatus=0}
	Temperature{mValue=10400.0, mType=7, mName=ocp_tpu, mStatus=0}
	Temperature{mValue=1300.0, mType=6, mName=smpl_gm, mStatus=0}
	Temperature{mValue=1250.0, mType=6, mName=vdroop2, mStatus=0}
	Temperature{mValue=4900.0, mType=7, mName=batoilo, mStatus=0}
	Temperature{mValue=0.0, mType=8, mName=soc, mStatus=0}
	Temperature{mValue=0.0, mType=-1, mName=FLASH_LED_REDUCE, mStatus=0}
	Temperature{mValue=0.0, mType=6, mName=critical-battery-cell, mStatus=0}
	Temperature{mValue=26.000002, mType=1, mName=G3D, mStatus=0}
	Temperature{mValue=26.000002, mType=0, mName=BIG, mStatus=0}
	Temperature{mValue=27.000002, mType=0, mName=LITTLE, mStatus=0}
	Temperature{mValue=0.0, mType=4, mName=VIRTUAL-USB-THROTTLING, mStatus=0}
	Temperature{mValue=-0.117000006, mType=-1, mName=USB2-MINUS-USB, mStatus=0}
	Temperature{mValue=22.912, mType=-1, mName=neutral_therm, mStatus=0}
	Temperature{mValue=24.83324, mType=5, mName=cellular-emergency, mStatus=0}
	Temperature{mValue=27.000002, mType=0, mName=MID, mStatus=0}
	Temperature{mValue=-0.82000005, mType=-1, mName=USB2-MINUS-QI, mStatus=0}
	Temperature{mValue=24.83324, mType=-1, mName=VIRTUAL-SKIN-CHARGE, mStatus=0}
	Temperature{mValue=24.83324, mType=3, mName=VIRTUAL-SKIN, mStatus=0}
	Temperature{mValue=21.542654, mType=-1, mName=VIRTUAL-QUIET-BATT, mStatus=0}
	Temperature{mValue=1.0, mType=6, mName=battery_cycle, mStatus=0}
	Temperature{mValue=24.83324, mType=-1, mName=VIRTUAL-USB2-DISP, mStatus=0}
	Temperature{mValue=6900.0, mType=7, mName=soft_ocp_cpu1, mStatus=0}
	Temperature{mValue=21.000002, mType=-1, mName=rf2_therm, mStatus=0}
	Temperature{mValue=11900.0, mType=7, mName=ocp_gpu, mStatus=0}
	Temperature{mValue=6900.0, mType=7, mName=ocp_cpu1, mStatus=0}
	Temperature{mValue=18.844501, mType=-1, mName=VIRTUAL-QI-GNSS, mStatus=0}
	Temperature{mValue=0.0, mType=4, mName=VIRTUAL-USB-UI, mStatus=0}
	Temperature{mValue=21.000002, mType=-1, mName=rf1_therm, mStatus=0}
	Temperature{mValue=11900.0, mType=7, mName=ocp_cpu2, mStatus=0}
	Temperature{mValue=23.427002, mType=-1, mName=disp_therm, mStatus=0}
	Temperature{mValue=1050.0, mType=6, mName=vdroop1, mStatus=0}
	Temperature{mValue=23.661001, mType=-1, mName=qi_therm, mStatus=0}
	Temperature{mValue=23.239, mType=-1, mName=gnss_tcxo_therm, mStatus=0}
	Temperature{mValue=22.771002, mType=-1, mName=quiet_therm, mStatus=0}
	Temperature{mValue=21.94342, mType=-1, mName=VIRTUAL-QI-BATT, mStatus=0}
	Temperature{mValue=22.841002, mType=-1, mName=usb_pwr_therm2, mStatus=0}
	Temperature{mValue=22.958, mType=-1, mName=usb_pwr_therm, mStatus=0}
	Temperature{mValue=22.1, mType=2, mName=battery, mStatus=0}
Current cooling devices from HAL:`

	previous := machine.NewDescription(ctx)
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name:      "skia-rpi2-0001",
			StartTime: bootUpTime,
		},
		Android: machine.Android{
			Uptime:                10,
			DumpsysThermalService: dumpsysBattery,
		},
	}

	ctx.SetTime(serverTime)
	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	assert.Equal(t, machine.ModeAvailable, next.Mode)
	assert.Empty(t, next.Dimensions[machine.DimQuarantined])

	assert.Equal(t, float64(11.9), metrics2.GetFloat64Metric("machine_processor_device_temperature_c", map[string]string{"machine": "skia-rpi2-0001", "sensor": "ocp_cpu2"}).Get())
	assert.Equal(t, int64(0), metrics2.GetInt64Metric("machine_processor_device_maintenance", next.Dimensions.AsMetricsTags()).Get())
	assert.Equal(t, int64(0), metrics2.GetInt64Metric("machine_processor_device_time_in_recovery_mode_s", next.Dimensions.AsMetricsTags()).Get())
}

func TestProcess_RecoveryModeIfDeviceTooHotAndBatteryIsTooLow(t *testing.T) {
	unittest.SmallTest(t)

	stateTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	bootUpTime := time.Date(2021, time.September, 1, 10, 1, 0, 0, time.UTC)
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)

	ctx := now.TimeTravelingContext(stateTime)

	previous := machine.NewDescription(ctx)
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name:      "skia-rpi2-0001",
			StartTime: bootUpTime,
		},
		Android: machine.Android{
			Uptime: 10,
			DumpsysBattery: `Current Battery Service state:
  AC powered: true
  USB powered: false
  Wireless powered: false
  Max charging current: 1500000
  Max charging voltage: 5000000
  Charge counter: 2448973
  status: 2
  health: 2
  present: true
  level: 9
  scale: 100
  voltage: 4248`,
			DumpsysThermalService: `IsStatusOverride: false
ThermalEventListeners:
	callbacks: 3
	killed: false
	broadcasts count: -1
ThermalStatusListeners:
	callbacks: 1
	killed: false
	broadcasts count: -1
Thermal Status: 0
Cached temperatures:
	Temperature{mValue=32.401, mType=3, mName=mb-therm-monitor, mStatus=0}
	Temperature{mValue=46.100002, mType=0, mName=cpu0-silver-usr, mStatus=0}
	Temperature{mValue=44.800003, mType=0, mName=cpu1-silver-usr, mStatus=0}
	Temperature{mValue=45.100002, mType=0, mName=cpu2-silver-usr, mStatus=0}
	Temperature{mValue=40.600002, mType=1, mName=gpu0-usr, mStatus=0}
	Temperature{mValue=40.300003, mType=1, mName=gpu1-usr, mStatus=0}
	Temperature{mValue=44.100002, mType=0, mName=cpu3-silver-usr, mStatus=0}
	Temperature{mValue=45.4, mType=0, mName=cpu4-silver-usr, mStatus=0}
	Temperature{mValue=45.4, mType=0, mName=cpu5-silver-usr, mStatus=0}
	Temperature{mValue=30.000002, mType=2, mName=battery, mStatus=0}
	Temperature{mValue=48.300003, mType=0, mName=cpu1-gold-usr, mStatus=0}
	Temperature{mValue=46.7, mType=0, mName=cpu0-gold-usr, mStatus=0}
	Temperature{mValue=27.522001, mType=4, mName=usbc-therm-monitor, mStatus=0}
HAL Ready: true
HAL connection:
	ThermalHAL 2.0 connected: yes
Current temperatures from HAL:
	Temperature{mValue=28.000002, mType=2, mName=battery, mStatus=0}
	Temperature{mValue=33.800003, mType=0, mName=cpu0-gold-usr, mStatus=0}
	Temperature{mValue=33.800003, mType=0, mName=cpu0-silver-usr, mStatus=0}
	Temperature{mValue=33.5, mType=0, mName=cpu1-gold-usr, mStatus=0}
	Temperature{mValue=44.1, mType=0, mName=cpu1-silver-usr, mStatus=0}
	Temperature{mValue=43.8, mType=0, mName=cpu2-silver-usr, mStatus=0}
	Temperature{mValue=33.5, mType=0, mName=cpu3-silver-usr, mStatus=0}
	Temperature{mValue=33.5, mType=0, mName=cpu4-silver-usr, mStatus=0}
	Temperature{mValue=33.5, mType=0, mName=cpu5-silver-usr, mStatus=0}
	Temperature{mValue=32.9, mType=1, mName=gpu0-usr, mStatus=0}
	Temperature{mValue=32.9, mType=1, mName=gpu1-usr, mStatus=0}
	Temperature{mValue=30.147001, mType=3, mName=mb-therm-monitor, mStatus=0}
	Temperature{mValue=26.926, mType=4, mName=usbc-therm-monitor, mStatus=0}
Current cooling devices from HAL:
	CoolingDevice{mValue=0, mType=1, mName=battery}
	CoolingDevice{mValue=0, mType=2, mName=thermal-cpufreq-0}
	CoolingDevice{mValue=0, mType=2, mName=thermal-cpufreq-6}
	CoolingDevice{mValue=0, mType=3, mName=thermal-devfreq-0}`,
		},
	}

	ctx.SetTime(serverTime)
	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	assert.Equal(t, "Battery low. Too hot. ", next.Annotation.Message)
	assert.Equal(t, machineUserName, next.Annotation.User)
	assert.Equal(t, machine.ModeRecovery, next.Mode)
	assert.Empty(t, next.Dimensions[machine.DimQuarantined])

	assert.Equal(t, float64(44.1), metrics2.GetFloat64Metric("machine_processor_device_temperature_c", map[string]string{"machine": "skia-rpi2-0001", "sensor": "cpu1-silver-usr"}).Get())
	assert.Equal(t, int64(1), metrics2.GetInt64Metric("machine_processor_device_maintenance", next.Dimensions.AsMetricsTags()).Get())
	assert.Equal(t, int64(0), metrics2.GetInt64Metric("machine_processor_device_time_in_recovery_mode_s", next.Dimensions.AsMetricsTags()).Get())
}

func TestProcess_DoNotGoIntoMaintenanceModeIfDeviceBatteryIsChargedEnough(t *testing.T) {
	unittest.SmallTest(t)

	stateTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	bootUpTime := time.Date(2021, time.September, 1, 10, 1, 0, 0, time.UTC)
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)

	ctx := now.TimeTravelingContext(stateTime)

	previous := machine.NewDescription(ctx)
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name:      "skia-rpi2-0001",
			StartTime: bootUpTime,
		},
		Android: machine.Android{
			Uptime: 10,
			DumpsysBattery: `Current Battery Service state:
  AC powered: true
  USB powered: false
  Wireless powered: false
  Max charging current: 1500000
  Max charging voltage: 5000000
  Charge counter: 2448973
  status: 2
  health: 2
  present: true
  level: 95
  scale: 100
  voltage: 4248`,
		},
	}

	ctx.SetTime(serverTime)
	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	assert.Empty(t, next.Dimensions[machine.DimQuarantined])
	assert.Equal(t, machine.ModeAvailable, next.Mode)
	assert.Equal(t, 95, next.Battery)
	assert.Equal(t, int64(0), metrics2.GetInt64Metric("machine_processor_device_time_in_recovery_mode_s", next.Dimensions.AsMetricsTags()).Get())
}

func TestProcess_LeaveRecoveryModeIfDeviceBatteryIsChargedEnough(t *testing.T) {
	unittest.SmallTest(t)

	stateTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	bootUpTime := time.Date(2021, time.September, 1, 10, 1, 0, 0, time.UTC)
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)

	ctx := now.TimeTravelingContext(stateTime)

	previous := machine.NewDescription(ctx)
	previous.Mode = machine.ModeRecovery
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name:      "skia-rpi2-0001",
			StartTime: bootUpTime,
		},
		Android: machine.Android{
			Uptime: 10,
			DumpsysBattery: `Current Battery Service state:
  AC powered: true
  USB powered: false
  Wireless powered: false
  Max charging current: 1500000
  Max charging voltage: 5000000
  Charge counter: 2448973
  status: 2
  health: 2
  present: true
  level: 95
  scale: 100
  voltage: 4248`,
		},
	}

	ctx.SetTime(serverTime)
	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	assert.Empty(t, next.Dimensions[machine.DimQuarantined])
	assert.Equal(t, machine.ModeAvailable, next.Mode)
	assert.Equal(t, 95, next.Battery)
}

func TestProcess_ChromeOSDeviceAttached_UnquarantineAndMergeDimensions(t *testing.T) {
	unittest.SmallTest(t)

	stateTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	eventTime := time.Date(2021, time.September, 1, 10, 1, 0, 0, time.UTC)
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)

	previous := machine.Description{
		Mode:                machine.ModeAvailable,
		LastUpdated:         stateTime,
		RunningSwarmingTask: false,
		LaunchedSwarming:    true,
		DeviceUptime:        0,
		SSHUserIP:           "root@my-chromebook",
		SuppliedDimensions: machine.SwarmingDimensions{
			"cpu": []string{"arm"},
			"gpu": []string{"MaliT654"},
		},
		Dimensions: machine.SwarmingDimensions{
			"id":                   []string{"skia-rpi2-0001"},
			"os":                   []string{"Debian", "Debian-11", "Debian-11.0", "Linux"},
			"cpu":                  []string{"x86", "x86_64"},
			"gpu":                  []string{"none"},
			machine.DimQuarantined: []string{"Device root@my-chromebook has gone missing"},
		},
	}
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name:      "skia-rpi2-0001",
			StartTime: eventTime,
		},
		LaunchedSwarming: true,
		ChromeOS: machine.ChromeOS{
			Channel:        "stable-channel",
			Milestone:      "89",
			ReleaseVersion: "13729.56.0",
			Uptime:         123 * time.Second,
		},
	}

	ctx := now.TimeTravelingContext(serverTime)
	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	assert.Equal(t, machine.Description{
		Mode:                machine.ModeAvailable,
		LastUpdated:         serverTime,
		RunningSwarmingTask: false,
		LaunchedSwarming:    true,
		DeviceUptime:        123,
		SSHUserIP:           "root@my-chromebook",
		SuppliedDimensions: machine.SwarmingDimensions{
			"cpu": []string{"arm"},
			"gpu": []string{"MaliT654"},
		},
		Dimensions: machine.SwarmingDimensions{
			"id":                 []string{"skia-rpi2-0001"},
			machine.DimOS:        []string{"ChromeOS"},       // overwritten
			"cpu":                []string{"arm"},            // overwritten
			"gpu":                []string{"MaliT654"},       // overwritten
			"chromeos_channel":   []string{"stable-channel"}, // added
			"chromeos_milestone": []string{"89"},             // added
			"release_version":    []string{"13729.56.0"},     // added
			// quarantined was removed.
		},
	}, next)
}

func TestProcess_ChromeOSDeviceSpecifiedButNotAttached_Quarantined(t *testing.T) {
	unittest.SmallTest(t)

	stateTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	eventTime := time.Date(2021, time.September, 1, 10, 1, 0, 0, time.UTC)
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)

	previous := machine.Description{
		Mode:        machine.ModeAvailable,
		LastUpdated: stateTime,
		SSHUserIP:   "root@my-chromebook",
		SuppliedDimensions: machine.SwarmingDimensions{
			"cpu": []string{"arm"},
			"gpu": []string{"MaliT654"},
		},
		Dimensions: machine.SwarmingDimensions{
			"id":  []string{"skia-rpi2-0001"},
			"os":  []string{"Debian", "Debian-11", "Debian-11.0", "Linux"},
			"cpu": []string{"x86", "x86_64"},
			"gpu": []string{"none"},
		},
	}
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name:      "skia-rpi2-0001",
			StartTime: eventTime,
		},
	}

	ctx := now.TimeTravelingContext(serverTime)
	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	assert.Equal(t, machine.Description{
		Mode:        machine.ModeAvailable,
		LastUpdated: serverTime,
		SSHUserIP:   "root@my-chromebook",
		SuppliedDimensions: machine.SwarmingDimensions{
			"cpu": []string{"arm"},
			"gpu": []string{"MaliT654"},
		},
		Dimensions: machine.SwarmingDimensions{
			"id":                   []string{"skia-rpi2-0001"},
			"os":                   []string{"Debian", "Debian-11", "Debian-11.0", "Linux"},
			"cpu":                  []string{"x86", "x86_64"},
			"gpu":                  []string{"none"},
			machine.DimQuarantined: []string{"Device root@my-chromebook has gone missing"},
		},
	}, next)
}

func TestProcess_ChromeOSDeviceDisconnected_QuarantinedSet(t *testing.T) {
	unittest.SmallTest(t)

	stateTime := time.Date(2021, time.September, 1, 10, 0, 0, 0, time.UTC)
	eventTime := time.Date(2021, time.September, 1, 10, 1, 0, 0, time.UTC)
	serverTime := time.Date(2021, time.September, 1, 10, 1, 5, 0, time.UTC)

	previous := machine.Description{
		Mode:        machine.ModeAvailable,
		LastUpdated: stateTime,
		SSHUserIP:   "root@my-chromebook",
		SuppliedDimensions: machine.SwarmingDimensions{
			"cpu": []string{"arm"},
			"gpu": []string{"MaliT654"},
		},
		Dimensions: machine.SwarmingDimensions{
			"id":                 []string{"skia-rpi2-0001"},
			machine.DimOS:        []string{"ChromeOS"},
			"cpu":                []string{"arm"},
			"gpu":                []string{"MaliT654"},
			"chromeos_channel":   []string{"stable-channel"},
			"chromeos_milestone": []string{"89"},
			"release_version":    []string{"13729.56.0"},
		},
	}
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name:      "skia-rpi2-0001",
			StartTime: eventTime,
		},
	}

	ctx := now.TimeTravelingContext(serverTime)
	p := newProcessorForTest()
	next := p.Process(ctx, previous, event)
	assert.Equal(t, machine.Description{
		Mode:        machine.ModeAvailable,
		LastUpdated: serverTime,
		SSHUserIP:   "root@my-chromebook",
		SuppliedDimensions: machine.SwarmingDimensions{
			"cpu": []string{"arm"},
			"gpu": []string{"MaliT654"},
		},
		Dimensions: machine.SwarmingDimensions{
			"id":                   []string{"skia-rpi2-0001"},
			machine.DimOS:          []string{"ChromeOS"},
			"cpu":                  []string{"arm"},
			"gpu":                  []string{"MaliT654"},
			"chromeos_channel":     []string{"stable-channel"},
			"chromeos_milestone":   []string{"89"},
			"release_version":      []string{"13729.56.0"},
			machine.DimQuarantined: []string{"Device root@my-chromebook has gone missing"},
		},
	}, next)
}

func TestBatteryFromAndroidDumpSys_Success(t *testing.T) {
	unittest.SmallTest(t)
	battery, ok := batteryFromAndroidDumpSys(`Current Battery Service state:
  level: 94
  scale: 100
  `)
	assert.True(t, ok)
	assert.Equal(t, 94, battery)
}

// It turns out that hammerhead devices separate dumpsys lines with \r\n instead
// of \n, so test for that.
func TestBatteryFromAndroidDumpSys_Hammerhead_Success(t *testing.T) {
	unittest.SmallTest(t)
	battery, ok := batteryFromAndroidDumpSys("Current Battery Service state:\r\n  AC powered: false\r\n  USB powered: true\r\n  Wireless powered: false\r\n  Max charging current: 0\r\n  status: 5\r\n  health: 2\r\n  present: true\r\n  level: 45\r\n  scale: 100\r\n  voltage: 4189\r\n  temperature: 180\r\n  technology: Li-ion\r\n")
	assert.True(t, ok)
	assert.Equal(t, 45, battery)
}

func TestBatteryFromAndroidDumpSys_FalseOnEmptyString(t *testing.T) {
	unittest.SmallTest(t)
	_, ok := batteryFromAndroidDumpSys("")
	assert.False(t, ok)
}

func TestBatteryFromAndroidDumpSys_FalseIfNoLevel(t *testing.T) {
	unittest.SmallTest(t)
	_, ok := batteryFromAndroidDumpSys(`Current Battery Service state:
  scale: 100
  `)
	assert.False(t, ok)
}
func TestBatteryFromAndroidDumpSys_FalseIfNoScale(t *testing.T) {
	unittest.SmallTest(t)
	_, ok := batteryFromAndroidDumpSys(`Current Battery Service state:
  level: 94
  `)
	assert.False(t, ok)
}

func TestBatteryFromAndroidDumpSys_FailOnBadScale(t *testing.T) {
	unittest.SmallTest(t)
	_, ok := batteryFromAndroidDumpSys(`Current Battery Service state:
  level: 94
  scale: 0
  `)
	assert.False(t, ok)
}

func TestTemperatureFromAndroid_FindTempInThermalServiceOutput(t *testing.T) {
	unittest.SmallTest(t)
	thermalServiceOutput := `IsStatusOverride: false
ThermalEventListeners:
	callbacks: 1
	killed: false
	broadcasts count: -1
ThermalStatusListeners:
	callbacks: 1
	killed: false
	broadcasts count: -1
Thermal Status: 0
Cached temperatures:
 Temperature{mValue=-99.9, mType=6, mName=TYPE_POWER_AMPLIFIER, mStatus=0}
	Temperature{mValue=25.3, mType=4, mName=TYPE_SKIN, mStatus=0}
	Temperature{mValue=24.0, mType=1, mName=TYPE_CPU, mStatus=0}
	Temperature{mValue=24.4, mType=3, mName=TYPE_BATTERY, mStatus=0}
	Temperature{mValue=24.2, mType=5, mName=TYPE_USB_PORT, mStatus=0}
HAL Ready: true
HAL connection:
	Sdhms connected: yes
Current temperatures from HAL:
	Temperature{mValue=24.0, mType=1, mName=TYPE_CPU, mStatus=0}
	Temperature{mValue=24.4, mType=3, mName=TYPE_BATTERY, mStatus=0}
	Temperature{mValue=25.3, mType=4, mName=TYPE_SKIN, mStatus=0}
	Temperature{mValue=24.2, mType=5, mName=TYPE_USB_PORT, mStatus=0}
	Temperature{mValue=-99.9, mType=6, mName=TYPE_POWER_AMPLIFIER, mStatus=0}
Current cooling devices from HAL:
	CoolingDevice{mValue=0, mType=2, mName=TYPE_CPU}
	CoolingDevice{mValue=0, mType=3, mName=TYPE_GPU}
	CoolingDevice{mValue=0, mType=1, mName=TYPE_BATTERY}
	CoolingDevice{mValue=1, mType=4, mName=TYPE_MODEM}`
	a := machine.Android{
		DumpsysThermalService: thermalServiceOutput,
	}
	temp, ok := temperatureFromAndroid(a)
	assert.True(t, ok)
	assert.Equal(t, map[string]float64{
		"TYPE_CPU":      24.0,
		"TYPE_BATTERY":  24.4,
		"TYPE_SKIN":     25.3,
		"TYPE_USB_PORT": 24.2,
	}, temp)
}

func TestTemperatureFromAndroid_ReturnFalseIfNoOutputFromThermalOrBatteryService(t *testing.T) {
	unittest.SmallTest(t)
	a := machine.Android{}
	_, ok := temperatureFromAndroid(a)
	assert.False(t, ok)
}

func TestTemperatureFromAndroid_FindTempInBatteryServiceOutput(t *testing.T) {
	unittest.SmallTest(t)
	batteryOutput := `Current Battery Service state:
	AC powered: true
	USB powered: false
	Wireless powered: false
	Max charging current: 1500000
	Max charging voltage: 5000000
	Charge counter: 2448973
	status: 2
	health: 2
	present: true
	level: 94
	scale: 100
	voltage: 4248
	temperature: 281
	technology: Li-ion
  `
	a := machine.Android{
		DumpsysBattery: batteryOutput,
	}
	temp, ok := temperatureFromAndroid(a)
	assert.True(t, ok)
	assert.Equal(t, map[string]float64{batteryTemperatureKey: 28.1}, temp)
}
