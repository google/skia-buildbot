// Package sink is for sending machine.Events that are eventually picked up by
// 'source'.
package sink

import (
	"context"

	"go.skia.org/infra/machine/go/machine"
)

// Sink is for sending machine.Events that are received by source.Source.
type Sink interface {

	// Send the event. Returns when sent, not received.
	Send(context.Context, machine.Event) error
}
