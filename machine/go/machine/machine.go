package machine

import "time"

// SwarmingState is for de/serializing swarming state.
//
// https://chromium.googlesource.com/infra/luci/luci-py.git/+/master/appengine/swarming/doc/Magic-Values.md#bot-states
type SwarmingState struct {
	// ID is the name of the machine, aka the swarming bot id.
	ID string `json:"id"`

	Maintenance string `json:"maintenance,omitempty"`

	Quarantined string `json:"quarantined,omitempty"`

	SkRack string `json:"sk_rack"`
}

// SwarmingDimensions is for de/serializing swarming dimensions:
//
// Note we don't need to sidecar the swarming bot ID like we do in SwarmingState
// because 'id' is a required dimension.
// https://chromium.googlesource.com/infra/luci/luci-py.git/+/master/appengine/swarming/doc/Magic-Values.md#bot-dimensions
type SwarmingDimensions map[string][]string

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
	Mode        Mode
	Annotation  Annotation
	Dimensions  SwarmingDimensions
	State       SwarmingState
	LastUpdated time.Time
}

// EventType is the type of update we got from the machine.
type EventType string

const (
	// EventTypeDimensions means the dimensions have been updated.
	EventTypeDimensions EventType = "dimensions"

	// EventTypeState means the state has been updated.
	EventTypeState EventType = "state"
)

// Event is the information a machine should send via Source when
// its local state has changed.
type Event struct {
	EventType  EventType          `json:"type"`
	Dimensions SwarmingDimensions `json:"dimensions"`
	State      SwarmingState      `json:"state"`
}
