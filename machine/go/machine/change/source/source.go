package source

import (
	"context"
)

// MetricName used for all successful receives by a Source implementation.
const MetricName = "machine_change_receive_success"

// Source provides a channel of events that arrive every time a machine's
// Description had been updated.
type Source interface {
	// Start the process of receiving events.
	Start(ctx context.Context) <-chan interface{}
}
