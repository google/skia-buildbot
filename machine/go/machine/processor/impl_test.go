package processor

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		"[ro.product.manufacturer]: [Google]", // Ignored
		"[ro.product.model]: [Pixel 3a]",      // Ignored
		"[ro.build.id]: [QQ2A.200305.002]",    // device_os
		"[ro.product.brand]: [google]",        // device_os_flavor
		"[ro.build.type]: [user]",             // device_os_type
		"[ro.product.device]: [sargo]",        // device_type
		"[ro.build.product]: [sargo]",         // device_type (dup should be ignored)
		"[ro.product.system.brand]: [google]", // device_os_flavor (dup should be ignored)
		"[ro.product.system.brand]: [aosp]",   // device_os_flavor (should be converted to "android")
	}, "\n")

	dimensions := parseAndroidProperties(adbResponse)
	got := dimensionsFromAndroidProperties(dimensions)

	expected := map[string][]string{
		"android_devices":     {"1"},
		"device_os":           {"Q", "QQ2A.200305.002"},
		"device_os_flavor":    {"google", "android"},
		"device_os_type":      {"user"},
		machine.DimDeviceType: {"sargo"},
		machine.DimOS:         {"Android"},
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

func TestProcess_NewDeviceAttached(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	// The current machine has nothing attached.
	previous := machine.NewDescription()
	require.Empty(t, previous.Dimensions)

	// An event arrives with the attachment of an Android device.
	props := strings.Join([]string{
		"[ro.build.id]: [QQ2A.200305.002]",  // device_os
		"[ro.product.brand]: [google]",      // device_os_flavor
		"[ro.build.type]: [user]",           // device_os_type
		"[ro.product.device]: [sargo]",      // device_type
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

func TestProcess_QuarantineDevicesInMaintenanceMode(t *testing.T) {
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

	// An event arrives without any device info.
	event := machine.Event{
		EventType: machine.EventTypeRawState,
		Host: machine.Host{
			Name: "skia-rpi2-0001",
		},
	}

	p := newProcessorForTest(t)
	next := p.Process(ctx, previous, event)
	require.Equal(t, int64(1), p.eventsProcessedCount.Get())
	require.Equal(t, int64(0), p.unknownEventTypeCount.Get())

	// The dimensions should not change, except for the addition of the
	// quarantine message.
	expected := previous.Dimensions
	expected[machine.DimQuarantined] = []string{"Device is quarantined for maintenance"}
	assert.Equal(t, expected, next.Dimensions)
	assert.Equal(t, machine.ModeMaintenance, next.Mode)
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
		"[ro.product.device]: [sargo]",      // device_type
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
}
