// Package processor does the work of taking incoming events from machines and
// updating the machine state using that information.
package processor

import (
	"context"

	"go.skia.org/infra/machine/go/machine"
)

// Processor does the work of taking an incoming event and updating the Machine
// State based on that event.
type Processor interface {
	Process(ctx context.Context, current machine.Description, event machine.Event) machine.Description
}

// ProcessorFunc is a utility type that allows using a function as a Processor.
type ProcessorFunc func(ctx context.Context, current machine.Description, event machine.Event) machine.Description

// Process implements the Processor interface.
func (p ProcessorFunc) Process(ctx context.Context, current machine.Description, event machine.Event) machine.Description {
	return p(ctx, current, event)
}

// Confirm the ProcessorFunc implements Processor.
var _ Processor = ProcessorFunc(nil)
