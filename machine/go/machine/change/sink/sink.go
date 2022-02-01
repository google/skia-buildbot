package sink

import (
	"context"
)

// Sink notifies a machine that it's Description has changed.
//
type Sink interface {
	// Send a machine.Description from the server to the machine.
	Send(ctx context.Context, machineID string) error
}
