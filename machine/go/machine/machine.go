package machine

import (
	"context"
	"time"

	"go.skia.org/infra/go/now"
)

// SwarmingDimensions is for de/serializing swarming dimensions:
//
// https://chromium.googlesource.com/infra/luci/luci-py.git/+doc/master/appengine/swarming/doc/Magic-Values.md#bot-dimensions
type SwarmingDimensions map[string][]string

// Copy returns a deep copy of the dimensions map.
func (s SwarmingDimensions) Copy() SwarmingDimensions {
	n := make(SwarmingDimensions, len(s))
	for k, v := range s {
		copyValues := make([]string, len(v))
		copy(copyValues, v)
		n[k] = copyValues
	}
	return n
}

// Well known swarming dimensions.
const (
	DimID             = "id"
	DimOS             = "os"
	DimQuarantined    = "quarantined"
	DimDeviceType     = "device_type"
	DimAndroidDevices = "android_devices"
)

// Mode is the mode we want the machine to be in. Note that this is the desired
// state, it might not be the actual state, for example if we put a machine in
// maintenance mode it will only get there after it finishes running the current
// task.
type Mode string

const (
	// ModeAvailable means the machine should be available to run tasks (not in
	// maintenance mode). Note that the machine may still not be running tasks
	// if the Processor decides the machine should be quarantined, for example,
	// for having an overheated device.
	ModeAvailable Mode = "available"

	// ModeMaintenance means the machine is in maintenance mode and should not
	// run tasks.
	ModeMaintenance Mode = "maintenance"

	// ModeRecovery means the machine is cooling down and/or recharging its battery
	// and is unavailable to run tests.
	ModeRecovery Mode = "recovery"
)

// AllModes is a slice of all Mode* consts. Used when generating TypeScript
// definitions.
var AllModes = []Mode{ModeAvailable, ModeMaintenance, ModeRecovery}

// Annotation represents a timestamped message.
type Annotation struct {
	Message   string
	User      string
	Timestamp time.Time
}

// Description is the current state of a single machine.
type Description struct {
	Mode Mode

	// Annotation is used to record the most recent non-user change to Description.
	// For example, if the device battery is too low, this will be set automatically.
	// This will be in addition to the normal auditlog of user actions:
	// https://pkg.go.dev/go.skia.org/infra/go/auditlog?tab=doc
	Annotation Annotation

	// Note is a user authored message on the state of a machine.
	Note Annotation

	// KubernetesImage is the kubernetes image name.
	KubernetesImage string

	// Version of test_machine_monitor being run.
	Version string

	// ScheduledForDeletion will be a non-empty string and equal to PodName if
	// the pod should be deleted.
	ScheduledForDeletion string

	// PowerCycle is true if the machine needs to be power-cycled.
	PowerCycle bool

	LastUpdated         time.Time
	Battery             int                // Charge as an integer percent, e.g. 50% = 50.
	Temperature         map[string]float64 // In Celsius.
	RunningSwarmingTask bool
	LaunchedSwarming    bool      // True if test_machine_monitor launched Swarming.
	RecoveryStart       time.Time // When did the machine start being in recovery mode.
	DeviceUptime        int32     // Seconds
	// SSHUserIP, for example, "root@skia-sparky360-03" indicates we should connect to the
	// given ChromeOS device at that username and ip/hostname.
	SSHUserIP string

	// PodName and KubernetesImage are deprecated as we do not intend to run on k8s any more.
	Dimensions SwarmingDimensions
	PodName    string
}

// NewDescription returns a new Description instance. It describes an available machine with no
// known dimensions.
func NewDescription(ctx context.Context) Description {
	return Description{
		Mode:        ModeAvailable,
		Dimensions:  SwarmingDimensions{},
		LastUpdated: now.Now(ctx),
	}
}

// Copy returns a deep copy of Description.
func (d Description) Copy() Description {
	ret := d
	ret.Dimensions = d.Dimensions.Copy()
	ret.Temperature = map[string]float64{}
	for k, v := range d.Temperature {
		ret.Temperature[k] = v
	}
	return ret
}

// EventType is the type of update we got from the machine.
type EventType string

const (
	// EventTypeRawState means the raw state from test_machine_monitor has been
	// updated.
	EventTypeRawState EventType = "raw_state"
)

// Android contains the raw results from interrogating an Android device.
type Android struct {
	GetProp               string        `json:"getprop"`
	DumpsysBattery        string        `json:"dumpsys_battery"`
	DumpsysThermalService string        `json:"dumpsys_thermal_service"`
	Uptime                time.Duration `json:"uptime"`
}

// Host is information about the host machine.
type Host struct {
	// Name is the machine id, from SWARMING_BOT_ID environment variable or hostname().
	Name string `json:"name"`

	// PodName is the kubernetes pod name.
	PodName string `json:"pod_name"`

	// KubernetesImage is the container image being run.
	KubernetesImage string `json:"image"`

	// Version of test_machine_monitor being run.
	Version string `json:"version"`

	// StartTim is when the test_machine_monitor started running.
	StartTime time.Time `json:"start_time"`
}

// ChromeOS encapsulates the information reported by a ChromeOS machine.
type ChromeOS struct {
	Channel        string        `json:"channel"`
	Milestone      string        `json:"milestone"`
	ReleaseVersion string        `json:"release_version"`
	Uptime         time.Duration `json:"uptime"`
}

type IOS struct {
}

// Event is the information a machine should send via Source when
// its local state has changed.
type Event struct {
	EventType           EventType `json:"type"`
	Android             Android   `json:"android"`
	ChromeOS            ChromeOS  `json:"chromeos"`
	IOS                 IOS       `json:"ios"`
	Host                Host      `json:"host"`
	RunningSwarmingTask bool      `json:"running_swarming_task"`

	// LaunchedSwarming is true if test_machine_monitor launched Swarming.
	LaunchedSwarming bool `json:"launched_swarming"`
}

// NewEvent returns a new Event instance.
func NewEvent() Event {
	return Event{
		EventType: EventTypeRawState,
		Android:   Android{},
		Host:      Host{},
	}
}
