package processor

import (
	"context"

	"go.skia.org/infra/machine/go/state/types"
)

// Processor does the work of taking an incoming event and updating the Machine
// State based on that event.
type Processor interface {
	Process(ctx context.Context, current types.MachineState, event types.MachineUpdateEvent) types.MachineState
}

// ProcessorFunc is a utility type that allows using a function has a Processor.
type ProcessorFunc func(ctx context.Context, current types.MachineState, event types.MachineUpdateEvent) types.MachineState

// Process implements the Processor interface.
func (p ProcessorFunc) Process(ctx context.Context, current types.MachineState, event types.MachineUpdateEvent) types.MachineState {
	return p(ctx, current, event)
}

// Confirm the ProcessorFunc implements Processor.
var _ Processor = ProcessorFunc(nil)
