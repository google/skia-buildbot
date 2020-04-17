// Package machine is for interacting with the machine state server. See //machine.
package machine

import (
	"context"
	"sync"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machineserver/config"
)

type Machine struct {
	store *store.StoreImpl

	// mutex protects dims.
	mutex sync.Mutex

	// dimensions are the dimensions the machine state server wants us to report
	// to swarming.
	dimensions map[string][]string
}

func New(ctx context.Context, instanceConfig config.InstanceConfig) (*Machine, error) {
	store, err := store.New(ctx, false, instanceConfig)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to build store instance.")
	}
	return &Machine{
		dimensions: map[string][]string{},
		store:      store,
	}, nil
}

func (m *Machine) Start(ctx context.Context) error {

	// Start a loop that scans for local devices and sends pubsub events with all the data every 30s.

	// Also start a second loop that does a firestore onsnapshot watcher that gets the dims we should
	// be reporting to swarming.
}

func (m *Machine) Dims() map[string][]string {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.dimensions
}
