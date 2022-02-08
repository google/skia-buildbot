package source

import (
	"context"

	"go.skia.org/infra/machine/go/machine"
)

// Source provides a channel of machine.Events, implemented by using PubSub
// events sent by each machine.
//
// Note that machines should only send updates if state or dimensions has
// actually changed.
type Source interface {
	// Start the process of receiving events.
	Start(ctx context.Context) (<-chan machine.Event, error)
}
