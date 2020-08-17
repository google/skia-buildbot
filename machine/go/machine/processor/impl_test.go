package processor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/machine"
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
		"[ro.product.manufacturer]: [Google]",   // Ignored
		"[ro.product.model]: [Pixel 3a]",        // Ignored
		"[ro.build.id]: [QQ2A.200305.002]",      // device_os
		"[ro.product.brand]: [google]",          // device_os_flavor
		"[ro.build.type]: [user]",               // device_os_type
		"[ro.product.board]: []",                // Ignore empty values.
		"[ro.product.device]: [4560MMX_sprout]", // device_type
		"[ro.build.product]: [sargo]",           // device_type
		"[ro.product.system.brand]: [google]",   // device_os_flavor (dup should be ignored)
		"[ro.product.system.brand]: [aosp]",     // device_os_flavor (should be converted to "android")
	}, "\n")

	dimensions := parseAndroidProperties(adbResponse)
	got := dimensionsFromAndroidProperties(dimensions)

	expected := map[string][]string{
		"android_devices":     {"1"},
		"device_os":           {"Q", "QQ2A.200305.002"},
		"device_os_flavor":    {"google", "android"},
		"device_os_type":      {"user"},
		machine.DimDeviceType: {"4560MMX_sprout", "sargo"},
		machine.DimOS:         {"Android"},
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

func newProcessorForTest(t *testing.T) *ProcessorImpl {
	p := New(context.Background())
	p.eventsProcessedCount.Reset()
	p.unknownEventTypeCount.Reset()
	return p
}

func TestProcess_DetectBadEventType(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()
	event := machine.Event{
		EventType: machine.EventType(-1),
	}
	previous := machine.Description{}
	p := newProcessorForTest(t)
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
	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	assert.True(t, next.RunningSwarmingTask)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(0), p.unknownEventTypeCount.Get())
}

func TestProcess_NewDeviceAttached(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	// The current machine has nothing attached.
	previous := machine.NewDescription()
	require.Empty(t, previous.Dimensions)
	uptime := int32(5)

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
			Name:            "skia-rpi2-0001",
			PodName:         "rpi-swarming-12345-987",
			KubernetesImage: "gcr.io/skia-public/rpi-swarming-client:2020-05-09T19_28_20Z-jcgregorio-4fef3ca-clean",
		},
	}

	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(0), p.unknownEventTypeCount.Get())

	// The Android device should be reflected in the returned Dimensions.
	expected := machine.SwarmingDimensions{
		"android_devices":     []string{"1"},
		"device_os":           []string{"Q", "QQ2A.200305.002"},
		"device_os_flavor":    []string{"google", "android"},
		"device_os_type":      []string{"user"},
		machine.DimDeviceType: []string{"sargo"},
		machine.DimOS:         []string{"Android"},
		machine.DimID:         []string{"skia-rpi2-0001"},
		"inside_docker":       []string{"1", "containerd"},
	}
	assert.Equal(t, expected, next.Dimensions)
	assert.Equal(t, machine.ModeAvailable, next.Mode)
	assert.Equal(t, event.Host.PodName, next.PodName)
	assert.Equal(t, event.Host.KubernetesImage, next.KubernetesImage)
	assert.Equal(t, uptime, next.DeviceUptime)
}

func TestProcess_DetectInsideDocker(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	// The current machine has nothing attached.
	previous := machine.NewDescription()
	require.Empty(t, previous.Dimensions)

	// An event arrives with the attachment of an Android device.
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Android:   machine.Android{},
		Host: machine.Host{
			Name: "skia-rpi2-0001",
		},
	}

	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(0), p.unknownEventTypeCount.Get())

	// The Android device should be reflected in the returned Dimensions.
	expected := machine.SwarmingDimensions{
		machine.DimID:   []string{"skia-rpi2-0001"},
		"inside_docker": []string{"1", "containerd"},
	}
	assert.Equal(t, expected, next.Dimensions)
	assert.Equal(t, machine.ModeAvailable, next.Mode)
}

func TestProcess_DetectNotInsideDocker(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	// The current machine has nothing attached.
	previous := machine.NewDescription()
	require.Empty(t, previous.Dimensions)

	// An event arrives with the attachment of an Android device.
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Android:   machine.Android{},
		Host: machine.Host{
			Name: "skia-rpi-0001",
		},
	}

	p := newProcessorForTest(t)
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

func TestProcess_ClearScheduledForDelectionOnPodNameChange(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	// The current machine has nothing attached.
	previous := machine.NewDescription()
	require.Empty(t, previous.Dimensions)
	previous.ScheduledForDeletion = "foo"
	previous.PodName = "foo"

	// An event arrives with the attachment of an Android device.
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Android:   machine.Android{},
		Host: machine.Host{
			Name:    "skia-rpi-0001",
			PodName: "bar",
		},
	}

	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(0), p.unknownEventTypeCount.Get())

	// The Android should no longer be scheduled for deletion.
	assert.Equal(t, "", next.ScheduledForDeletion)
}

func TestProcess_DeviceGoingMissingMeansQuarantine(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	// The current machine has a device attached.
	previous := machine.NewDescription()
	previous.Dimensions = machine.SwarmingDimensions{
		"android_devices":     []string{"1"},
		"device_os":           []string{"Q", "QQ2A.200305.002"},
		"device_os_flavor":    []string{"google", "android"},
		"device_os_type":      []string{"user"},
		machine.DimDeviceType: []string{"sargo"},
		machine.DimOS:         []string{"Android"},
		machine.DimID:         []string{"skia-rpi2-0001"},
		"inside_docker":       []string{"1", "containerd"},
	}

	// An event arrives without any device info.
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Android: machine.Android{
			GetProp: "",
		},
		Host: machine.Host{
			Name: "skia-rpi2-0001",
		},
	}

	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(0), p.unknownEventTypeCount.Get())

	// The dimensions should not change, except for the addition of the
	// quarantine message, which tells swarming to quarantine this machine.
	expected := previous.Dimensions
	expected[machine.DimQuarantined] = []string{"Device [\"sargo\"] has gone missing"}
	assert.Equal(t, expected, next.Dimensions)
	assert.Equal(t, machine.ModeAvailable, next.Mode)
}

func TestProcess_DoNotQuarantineDevicesInMaintenanceMode(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	// The current machine has a device attached.
	previous := machine.NewDescription()
	previous.Dimensions = machine.SwarmingDimensions{
		"android_devices":     []string{"1"},
		"device_os":           []string{"Q", "QQ2A.200305.002"},
		"device_os_flavor":    []string{"google", "android"},
		"device_os_type":      []string{"user"},
		machine.DimDeviceType: []string{"sargo"},
		machine.DimOS:         []string{"Android"},
		machine.DimID:         []string{"skia-rpi2-0001"},
		"inside_docker":       []string{"1", "containerd"},
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
			Name: "skia-rpi2-0001",
		},
		Android: machine.Android{
			GetProp: props,
		},
	}

	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(0), p.unknownEventTypeCount.Get())

	// The dimensions should not change.
	expected := previous.Dimensions
	assert.Equal(t, expected, next.Dimensions)
	assert.Equal(t, machine.ModeMaintenance, next.Mode)
	assert.Equal(t, int64(0), metrics2.GetInt64Metric("machine_processor_device_quarantined", map[string]string{"machine": "skia-rpi2-0001"}).Get())
}

func TestProcess_RemoveMachineFromQuarantineIfDeviceReturns(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	// The current machine has been quarantined because the device went missing.
	previous := machine.NewDescription()
	previous.Dimensions = machine.SwarmingDimensions{
		"android_devices":      []string{"1"},
		"device_os":            []string{"Q", "QQ2A.200305.002"},
		"device_os_flavor":     []string{"google", "android"},
		"device_os_type":       []string{"user"},
		machine.DimDeviceType:  []string{"sargo"},
		machine.DimOS:          []string{"Android"},
		machine.DimQuarantined: []string{"Device [\"sargo\"] has gone missing"},
		machine.DimID:          []string{"skia-rpi2-0001"},
		"inside_docker":        []string{"1", "containerd"},
	}

	// An event arrives tith the device restored.
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
		},
		Host: machine.Host{
			Name: "skia-rpi2-0001",
		},
	}

	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(0), p.unknownEventTypeCount.Get())

	// The machine should no longer be quarantined.
	expected := machine.SwarmingDimensions{
		"android_devices":     []string{"1"},
		"device_os":           []string{"Q", "QQ2A.200305.002"},
		"device_os_flavor":    []string{"google", "android"},
		"device_os_type":      []string{"user"},
		machine.DimDeviceType: []string{"sargo"},
		machine.DimOS:         []string{"Android"},
		machine.DimID:         []string{"skia-rpi2-0001"},
		"inside_docker":       []string{"1", "containerd"},
	}
	assert.Equal(t, expected, next.Dimensions)
	assert.Equal(t, int64(0), metrics2.GetInt64Metric("machine_processor_device_quarantined", map[string]string{"machine": "skia-rpi2-0001"}).Get())
}

func TestProcess_RecoveryModeIfDeviceBatteryTooLow(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	previous := machine.NewDescription()
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name: "skia-rpi2-0001",
		},
		Android: machine.Android{
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

	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	assert.Equal(t, "Battery low. ", next.Annotation.Message)
	assert.Equal(t, machineUserName, next.Annotation.User)
	assert.Equal(t, 9, next.Battery)
	assert.Equal(t, machine.ModeRecovery, next.Mode)
	assert.Empty(t, next.Dimensions[machine.DimQuarantined])
	assert.Equal(t, int64(9), metrics2.GetInt64Metric("machine_processor_device_battery_level", map[string]string{"machine": "skia-rpi2-0001"}).Get())
	assert.Equal(t, int64(1), metrics2.GetInt64Metric("machine_processor_device_maintenance", map[string]string{"machine": "skia-rpi2-0001"}).Get())
}

func TestProcess_ScheduleForDeletionIfPodIsTooOld(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	previous := machine.NewDescription()
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name:      "skia-rpi2-0001",
			StartTime: time.Now().Add(-2 * maxPodLifetime),
			PodName:   "rpi-swarming-123",
		},
		Android: machine.Android{},
	}

	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	assert.Equal(t, next.ScheduledForDeletion, next.PodName)
	assert.NotEmpty(t, next.PodName)
	assert.Equal(t, next.Annotation.User, machineUserName)
	assert.Equal(t, next.Annotation.Message, "Pod too old, requested update for \"rpi-swarming-123\"")
}

func TestProcess_DoNoScheduleForDeletionIfPodIsntTooOld(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	previous := machine.NewDescription()
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name:      "skia-rpi2-0001",
			StartTime: time.Now().Add(-1 * maxPodLifetime / 2),
			PodName:   "rpi-swarming-123",
		},
		Android: machine.Android{},
	}

	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	assert.Empty(t, next.ScheduledForDeletion)
	assert.NotEmpty(t, next.PodName)
}

func TestProcess_RecoveryModeIfDeviceTooHot(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	previous := machine.NewDescription()
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name: "skia-rpi2-0001",
		},
		Android: machine.Android{
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

	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	assert.Equal(t, "Too hot. ", next.Annotation.Message)
	assert.Equal(t, machineUserName, next.Annotation.User)
	assert.Equal(t, machine.ModeRecovery, next.Mode)
	assert.Empty(t, next.Dimensions[machine.DimQuarantined])

	assert.Equal(t, float64(44.1), metrics2.GetFloat64Metric("machine_processor_device_temperature_c", map[string]string{"machine": "skia-rpi2-0001", "sensor": "cpu1-silver-usr"}).Get())
	assert.Equal(t, int64(1), metrics2.GetInt64Metric("machine_processor_device_maintenance", map[string]string{"machine": "skia-rpi2-0001"}).Get())
}

func TestProcess_RecoveryModeIfDeviceTooHotAndBatterIsTooLow(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	previous := machine.NewDescription()
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name: "skia-rpi2-0001",
		},
		Android: machine.Android{
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

	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	assert.Equal(t, "Battery low. Too hot. ", next.Annotation.Message)
	assert.Equal(t, machineUserName, next.Annotation.User)
	assert.Equal(t, machine.ModeRecovery, next.Mode)
	assert.Empty(t, next.Dimensions[machine.DimQuarantined])

	assert.Equal(t, float64(44.1), metrics2.GetFloat64Metric("machine_processor_device_temperature_c", map[string]string{"machine": "skia-rpi2-0001", "sensor": "cpu1-silver-usr"}).Get())
	assert.Equal(t, int64(1), metrics2.GetInt64Metric("machine_processor_device_maintenance", map[string]string{"machine": "skia-rpi2-0001"}).Get())
}

func TestProcess_DoNotGoIntoMaintenanceModeIfDeviceBatteryIsChargedEnough(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	previous := machine.NewDescription()
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name: "skia-rpi2-0001",
		},
		Android: machine.Android{
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

	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	assert.Empty(t, next.Dimensions[machine.DimQuarantined])
	assert.Equal(t, machine.ModeAvailable, next.Mode)
	assert.Equal(t, 95, next.Battery)
}

func TestProcess_LeaveRecoveryModeIfDeviceBatteryIsChargedEnough(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	previous := machine.NewDescription()
	previous.Mode = machine.ModeRecovery
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name: "skia-rpi2-0001",
		},
		Android: machine.Android{
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

	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	assert.Empty(t, next.Dimensions[machine.DimQuarantined])
	assert.Equal(t, machine.ModeAvailable, next.Mode)
	assert.Equal(t, 95, next.Battery)
}

func TestProcess_ReportBadBatteryIfDumpsysBatteryIsMissing(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	previous := machine.NewDescription()
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name: "skia-rpi2-0001",
		},
		Android: machine.Android{
			DumpsysBattery: "",
		},
	}

	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	assert.Empty(t, next.Dimensions[machine.DimQuarantined])
	assert.Equal(t, badBatteryLevel, next.Battery)
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

func TestSanitizeKubernetesImageName_SuccessForExpectedValue(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		"gcr.io/skia-public/rpi-swarming-client:2020-05-09T19_28_20Z-jcgregorio-4fef3ca-clean",
		sanitizeKubernetesImageName("image: gcr.io/skia-public/rpi-swarming-client:2020-05-09T19_28_20Z-jcgregorio-4fef3ca-clean\n"),
	)
}

func TestSanitizeKubernetesImageName_PassesEmptyStringThrough(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, "", sanitizeKubernetesImageName(""))
}

func TestSanitizeKubernetesImageName_PassesStringWithoutImagePrefix(t *testing.T) {
	unittest.SmallTest(t)
	const imageName = "gcr.io/skia-public/rpi-swarming-client:2020-05-09T19_28_20Z-jcgregorio-4fef3ca-clean"
	assert.Equal(t, imageName, sanitizeKubernetesImageName(imageName))
}
