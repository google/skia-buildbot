// Package sink is for sending machine.Events that eventually picked up by
// 'source'.
package sink

import (
	"context"

	"go.skia.org/infra/machine/go/machine"
)

// Sink is for sending machine.Events that eventually picked up by 'source'.
type Sink interface {
	Send(context.Context, machine.Event) error
}
