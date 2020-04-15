package store

import (
	"context"

	"go.skia.org/infra/machine/go/machine"
)

type TxCallback func(machine.Description) machine.Description

// Store and retrieve machine.Description.
type Store interface {
	// Update the machine with the given machineID using the given callback
	// function.
	//
	// txCallback will be called inside a firestore transaction and may be
	// called more than once.
	Update(ctx context.Context, machineID string, txCallback TxCallback) error

	// Watch returns a channel that will produce a machine.Description every time
	// the description for machineID changes.
	Watch(ctx context.Context, machineID string) <-chan machine.Description
}
