package source

import (
	"context"
	"sync"

	"go.skia.org/infra/go/skerr"
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

// MultiSource wraps multiple Sources into a single Source.
type MultiSource struct {
	sources []Source
}

// NewMultiSource returns a new MultiSource from the given Sources.
func NewMultiSource(sources ...Source) Source {
	return &MultiSource{
		sources: sources,
	}
}

// Start implements Source.
func (m *MultiSource) Start(ctx context.Context) (<-chan machine.Event, error) {
	outgoing := make(chan machine.Event, 100)

	var wg sync.WaitGroup
	wg.Add(len(m.sources))
	for _, s := range m.sources {
		ch, err := s.Start(ctx)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		go func(ch <-chan machine.Event) {
			defer wg.Done()
			for {
				value, ok := <-ch
				if !ok {
					return
				}
				outgoing <- value
			}
		}(ch)
	}

	go func() {
		wg.Wait()
		close(outgoing)
	}()

	return outgoing, nil
}
