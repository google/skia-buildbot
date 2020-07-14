package rpc_types

import (
	"go.skia.org/infra/autoroll/go/manual"
	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/autoroll/go/strategy"
)

//go:generate go run ../go2ts/main.go -o ../../modules/rpc_types.ts

// AutoRollStatus combines roller status with modes and strategies.
type AutoRollStatus struct {
	*status.AutoRollStatus
	Config         *roller.AutoRollerConfig    `json:"config"`
	ManualRequests []*manual.ManualRollRequest `json:"manualRequests"`
	Mode           *modes.ModeChange           `json:"mode"`
	Strategy       *strategy.StrategyChange    `json:"strategy"`
}

// AutoRollMiniStatus is returned by the /json/ministatus endpoint.
type AutoRollMiniStatus struct {
	*status.AutoRollMiniStatus
	ChildName  string `json:"childName,omitempty"`
	Mode       string `json:"mode"`
	ParentName string `json:"parentName,omitempty"`
}

// AutoRollMiniStatuses is returned by the /json/all endpoint.
type AutoRollMiniStatuses map[string]*AutoRollMiniStatus

// AutoRollModeChangeRequest is a request to change the autoroller's mode.
type AutoRollModeChangeRequest struct {
	Message string `json:"message"`
	Mode    string `json:"mode"`
}

// AutoRollStrategyChangeRequest is a request to change the autoroller's
// strategy.
type AutoRollStrategyChangeRequest struct {
	Message  string `json:"message"`
	Strategy string `json:"strategy"`
}
