package machine

import "time"

// SwarmingDimensions is for de/serializing swarming dimensions:
//
// https://chromium.googlesource.com/infra/luci/luci-py.git/+doc/master/appengine/swarming/doc/Magic-Values.md#bot-dimensions
type SwarmingDimensions map[string][]string

// Well known swarming dimensions.
const (
	DimID          = "id"
	DimOS          = "os"
	DimQuarantined = "quarantined"
	DimDeviceType  = "device_type"
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
)

// AllModes is a slice of all Mode* consts. Used when generating TypeScript
// definitions.
var AllModes = []Mode{ModeAvailable, ModeMaintenance}

// Annotation is used to record the most recent user change to Description. This
// will be in addition to the normal auditlog of user actions:
// https://pkg.go.dev/go.skia.org/infra/go/auditlog?tab=doc
type Annotation struct {
	Message   string
	User      string
	Timestamp time.Time
}

// Description is the current state of a single machine.
type Description struct {
	Mode       Mode
	Annotation Annotation
	Dimensions SwarmingDimensions
	PodName    string

	// KubernetesImage is the kubernetes image name.
	KubernetesImage string

	// ScheduledForDeletion will be a non-empty string and equal to PodName if
	// the pod should be deleted.
	ScheduledForDeletion string

	// PowerCycle is true if the machine needs to be power-cycled.
	PowerCycle bool

	LastUpdated         time.Time
	Battery             int                // Charge as an integer percent, e.g. 50% = 50.
	Temperature         map[string]float64 // In Celsius.
	RunningSwarmingTask bool
}

// NewDescription returns a new Description instance.
func NewDescription() Description {
	return Description{
		Mode:        ModeAvailable,
		Dimensions:  SwarmingDimensions{},
		LastUpdated: time.Now(),
	}
}

// Copy returns a deep copy of Description.
func (d Description) Copy() Description {
	ret := d
	ret.Dimensions = SwarmingDimensions{}
	for k, values := range d.Dimensions {
		newValues := make([]string, len(values))
		copy(newValues, values)
		ret.Dimensions[k] = newValues
	}
	ret.Temperature = map[string]float64{}
	for k, v := range d.Temperature {
		ret.Temperature[k] = v
	}
	return ret
}

// EventType is the type of update we got from the machine.
type EventType string

const (
	// EventTypeRawState means the raw state from bot_config has been updated.
	EventTypeRawState EventType = "raw_state"
)

// Android contains the raw results from interrogating an Android device.
type Android struct {
	GetProp               string `json:"getprop"`
	DumpsysBattery        string `json:"dumpsys_battery"`
	DumpsysThermalService string `json:"dumpsys_thermal_service"`
}

// Host is information about the host machine.
type Host struct {
	// Name is the machine id, from SWARMING_BOT_ID environment variable or hostname().
	Name string `json:"name"`

	// PodName is the kubernetes pod name.
	PodName string `json:"pod_name"`

	// KubernetesImage is the container image being run.
	KubernetesImage string `json:"image"`
}

// Event is the information a machine should send via Source when
// its local state has changed.
type Event struct {
	EventType           EventType `json:"type"`
	Android             Android   `json:"android"`
	Host                Host      `json:"host"`
	RunningSwarmingTask bool      `json:"running_swarming_task"`
}

// NewEvent returns a new Event instance.
func NewEvent() Event {
	return Event{
		EventType: EventTypeRawState,
		Android:   Android{},
		Host:      Host{},
	}
}
