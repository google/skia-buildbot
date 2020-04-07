package store

import (
	"context"

	"go.skia.org/infra/machine/go/machine"
)

// Store and retrieve machine.Description.
type Store interface {
	// Get the current state.
	Get(ctx context.Context, machineID string) (machine.Description, error)

	// Put the current state.
	Put(ctx context.Context, machineID string, state machine.Description)

	// TODO(jcgregorio) This will obviously have to expand to support the needs
	// of the web UI.
}
