package rpc

import (
	"time"

	"go.skia.org/infra/machine/go/machine"
)

type SupplyChromeOSRequest struct {
	SSHUserIP          string
	SuppliedDimensions machine.SwarmingDimensions
}

type SetNoteRequest struct {
	Message string
	// User and Timestamp will be added by the server
}

// FrontendDescription is the frontend representation of machine.Description. See that struct
// for details on the fields.
type FrontendDescription struct {
	Mode                machine.Mode
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

type ListMachinesResponse []FrontendDescription

func ToListMachinesResponse(descriptions []machine.Description) []FrontendDescription {
	rv := make([]FrontendDescription, 0, len(descriptions))
	for _, d := range descriptions {
		rv = append(rv, FrontendDescription{
			Mode:                d.Mode,
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
		})
	}
	return rv
}
