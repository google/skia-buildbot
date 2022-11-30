package source

import (
	"context"

	"go.skia.org/infra/machine/go/machine"
)

const (
	// Metrics names to be used by all implementations.

	// ReceiveSuccessMetricName of a counter to be incremented on every successful receive.
	ReceiveSuccessMetricName = "machine_source_receive_success"

	// ReceiveFailureMetricName of a counter to be incremented on every failed receive.
	ReceiveFailureMetricName = "machine_source_receive_failure"
)

// Source provides a channel of machine.Events as sent from machines.
//
// Note that machines should only send updates if state or dimensions has
// actually changed.
type Source interface {
	// Start the process of receiving events.
	Start(ctx context.Context) (<-chan machine.Event, error)
}
