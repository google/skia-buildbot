// Package store is for storing and retrieving machine.Descriptions.
package store

import (
	"context"

	"go.skia.org/infra/machine/go/machine"
)

// TxCallback is the callback that Store.Update() takes to update a single
// machine.Description. We use a callback because we want to compare the old
// state to decide the new state, along with other bits of info we can include
// in a closure, such as an incoming event. See also processor.Process.
type TxCallback func(machine.Description) machine.Description

// Store and retrieve machine.Descriptions.
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

	// List returns a slice containing all known machines.
	List(ctx context.Context) ([]machine.Description, error)
}
