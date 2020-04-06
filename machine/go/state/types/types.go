package types

import "time"

// SwarmingState is for de/serializing swarming state:
type SwarmingState struct {
	// ID is the name of the machine, aka the swarming bot id.
	ID string `json:"id"`

	// State is the swarming state dictionary
	// https://chromium.googlesource.com/infra/luci/luci-py.git/+/master/appengine/swarming/doc/Magic-Values.md#bot-states
	State map[string]interface{} `json:"state"`
}

// SwarmingDimensions is for de/serializing swarming dimensions:
//
// Note we don't need to sidecar the swarming bot ID like we do in SwarmingState
// because 'id' is a required dimension.
// https://chromium.googlesource.com/infra/luci/luci-py.git/+/master/appengine/swarming/doc/Magic-Values.md#bot-dimensions
type SwarmingDimensions map[string][]interface{}

// ModeType is the mode we want the machine to be in. Note that this is the
// desired state, it might not be the actual state, for example if we put a
// machine in maintenance mode it will only get there after it finishes running
// the current task.
type ModeType int

const (
	// ModeAvailable means the machine should be available to run tasks (not in
	// maintenance mode). Note that the machine may still not be running tasks
	// if the Processor decides the machine should be quarantined, for example,
	// for having an overheated device.
	ModeAvailable ModeType = 0

	// ModeMaintenance means the machine is in maintenance mode and should not
	// run tasks.
	ModeMaintenance ModeType = 1
)

// MachineState is the current state of a single machine.
type MachineState struct {
	Mode        ModeType
	Dimensions  SwarmingDimensions `json:"dimensions"`
	State       SwarmingState      `json:"state"`
	LastUpdated time.Time
}

// MachineUpdateEventType is the type of update we got from the machine.
type MachineUpdateEventType int

const (
	// MachineUpdateEventTypeDimensions means the dimensions have been updated.
	MachineUpdateEventTypeDimensions = 0

	// MachineUpdateEventTypeState means the state has been updated.
	MachineUpdateEventTypeState = 1
)

// MachineUpdateEvent is the information a machine should send via Source when
// it's local state has changed.
type MachineUpdateEvent struct {
	Type       MachineUpdateEventType
	Dimensions SwarmingDimensions
	State      SwarmingState
}
