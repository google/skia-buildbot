// Package sink is for sending machine.Events that are eventually picked up by
// 'source'.
package sink

import (
	"context"

	"go.skia.org/infra/machine/go/machine"
)

const (
	// Metrics names to be used by all implementations.

	// SendSuccessMetricName of a counter to be incremented on every successful send.
	SendSuccessMetricName = "machine_sink_send_success"

	// SendFailureMetricName of a counter to be incremented on every failed send.
	SendFailureMetricName = "machine_sink_send_failure"
)

// Sink is for sending machine.Events that are received by source.Source.
type Sink interface {
	// Send the event. Returns when sent, not received.
	Send(context.Context, machine.Event) error
}
