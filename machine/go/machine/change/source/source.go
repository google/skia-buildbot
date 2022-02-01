package source

import (
	"context"
)

// Source provides a channel of events that arrive every time a machine's
// Description had been updated.
type Source interface {
	// Start the process of receiving events.
	Start(ctx context.Context) (<-chan interface{}, error)
}
