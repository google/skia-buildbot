package rpc

import (
	"go.skia.org/infra/machine/go/machine"
)

// URL paths.
const (
	APIPrefix = "/json/v1"

	MachineDescriptionRelativeURL           = "/machine/description/{id:.+}"
	MachineEventRelativeURL                 = "/machine/event/"
	PowerCycleCompleteRelativeURL           = "/powercycle/complete/{id:.+}"
	PowerCycleListRelativeURL               = "/powercycle/list"
	PowerCycleStateUpdateRelativeURL        = "/powercycle/state/update"
	SSEMachineDescriptionUpdatedRelativeURL = "/machine/sse/description/updated"

	MachineDescriptionURL           = APIPrefix + MachineDescriptionRelativeURL
	MachineEventURL                 = APIPrefix + MachineEventRelativeURL
	PowerCycleCompleteURL           = APIPrefix + PowerCycleCompleteRelativeURL
	PowerCycleListURL               = APIPrefix + PowerCycleListRelativeURL
	PowerCycleStateUpdateURL        = APIPrefix + PowerCycleStateUpdateRelativeURL
	SSEMachineDescriptionUpdatedURL = APIPrefix + SSEMachineDescriptionUpdatedRelativeURL
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

type PowerCycleStateForMachine struct {
	MachineID       string
	PowerCycleState machine.PowerCycleState
}

type UpdatePowerCycleStateRequest struct {
	Machines []PowerCycleStateForMachine
}

// ListMachinesResponse is the full list of all known machines.
type ListMachinesResponse []machine.Description

// ListPowerCycleResponse is the list of machine ids that need powercycling.
type ListPowerCycleResponse []string

// ToListPowerCycleResponse converts the response from store.ListPowerCycle to a
// ListPowerCycleResponse.
func ToListPowerCycleResponse(machineIDs []string) ListPowerCycleResponse {
	return machineIDs
}
