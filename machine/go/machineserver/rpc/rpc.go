package rpc

import "go.skia.org/infra/machine/go/machine"

type SupplyChromeOSRequest struct {
	SSHUserIP          string
	SuppliedDimensions machine.SwarmingDimensions
}
