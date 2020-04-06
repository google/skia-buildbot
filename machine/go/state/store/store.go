package store

import (
	"context"

	"go.skia.org/infra/machine/go/state/types"
)

// Store and retrieve types.MachineState.
type Store interface {
	// Get the current state.
	Get(ctx context.Context, machineID string) (types.MachineState, error)

	// Put the current state.
	Put(ctx context.Context, machineID string, state types.MachineState)

	// TODO(jcgregorio) This will obviously have to expand to support the needs
	// of the web UI.
}
