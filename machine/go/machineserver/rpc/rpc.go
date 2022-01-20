package rpc

import (
	"time"

	"go.skia.org/infra/machine/go/machine"
)

// URL paths.
const (
	MachineDescriptionURL = "/json/v1/machine/description/{id:.+}"
	PowerCycleListURL     = "/json/v1/powercycle/list"
	PowerCycleCompleteURL = "/json/v1/powercycle/complete/{id:.+}"
)

type SupplyChromeOSRequest struct {
	SSHUserIP          string
	SuppliedDimensions machine.SwarmingDimensions
}

type SetNoteRequest struct {
	Message string
	// User and Timestamp will be added by the server
}

type SetAttachedDevice struct {
	AttachedDevice machine.AttachedDevice
}

// FrontendDescription is the frontend representation of machine.Description.
// See that struct for details on the fields.
type FrontendDescription struct {
	Mode                machine.Mode
	AttachedDevice      machine.AttachedDevice
	Annotation          machine.Annotation
	Note                machine.Annotation
	Version             string
	PowerCycle          bool
	LastUpdated         time.Time
	Battery             int
	Temperature         map[string]float64
	RunningSwarmingTask bool
	LaunchedSwarming    bool
	DeviceUptime        int32
	SSHUserIP           string
	Dimensions          machine.SwarmingDimensions
}

// ListMachinesResponse is the full list of all known machines.
type ListMachinesResponse []FrontendDescription

// ListPowerCycleResponse is the list of machine ids that need powercycling.
type ListPowerCycleResponse []string

// ToListPowerCycleResponse converts the response from store.ListPowerCycle to a
// ListPowerCycleResponse.
func ToListPowerCycleResponse(machineIDs []string) ListPowerCycleResponse {
	return machineIDs
}

// ToFrontendDescription converts a machine.Description into a FrontendDescription.
func ToFrontendDescription(d machine.Description) FrontendDescription {
	return FrontendDescription{
		Mode:                d.Mode,
		AttachedDevice:      d.AttachedDevice,
		Annotation:          d.Annotation,
		Note:                d.Note,
		Version:             d.Version,
		PowerCycle:          d.PowerCycle,
		LastUpdated:         d.LastUpdated,
		Battery:             d.Battery,
		Temperature:         d.Temperature,
		RunningSwarmingTask: d.RunningSwarmingTask,
		LaunchedSwarming:    d.LaunchedSwarming,
		DeviceUptime:        d.DeviceUptime,
		SSHUserIP:           d.SSHUserIP,
		Dimensions:          d.Dimensions,
	}
}

// ToListMachinesResponse converts []machine.Description into []FrontendDescription.
func ToListMachinesResponse(descriptions []machine.Description) []FrontendDescription {
	rv := make([]FrontendDescription, 0, len(descriptions))
	for _, d := range descriptions {
		rv = append(rv, ToFrontendDescription(d))
	}
	return rv
}
