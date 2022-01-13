// Package store is for storing and retrieving machine.Descriptions.
package store

import (
	"context"

	"go.skia.org/infra/machine/go/machine"
)

// UpdateCallback is the callback that Store.Update() takes to update a single
// machine.Description. We use a callback because we want to compare the old
// state to decide the new state, along with other bits of info we can include
// in a closure, such as an incoming event. See also processor.Process.
type UpdateCallback func(machine.Description) machine.Description

// Store and retrieve machine.Descriptions.
type Store interface {
	// Update the machine with the given machineID using the given callback
	// function.
	//
	// updateCallback may be called more than once (e.g. transaction retries).
	Update(ctx context.Context, machineID string, updateCallback UpdateCallback) error

	// Get returns the Description for the given machine.
	Get(ctx context.Context, machineID string) (machine.Description, error)

	// Watch returns a channel that will produce a machine.Description every time
	// the description for machineID changes.
	Watch(ctx context.Context, machineID string) <-chan machine.Description

	// WatchForPowerCycle returns a channel that will produce the name of a
	// machine that needs to be power-cycled. Before a machineID is sent on the
	// channel the PowerCycle value is set back to false. If rack is set then
	// only machines whose id contains the rack value will be returned.
	WatchForPowerCycle(ctx context.Context, rack string) <-chan string

	// ListPowerCycle returns a list of machine names that need powercycling.
	ListPowerCycle(ctx context.Context) ([]string, error)

	// List returns a slice containing all known machines.
	List(ctx context.Context) ([]machine.Description, error)

	// Delete removes a machine from the database.
	Delete(ctx context.Context, machineID string) error
}
