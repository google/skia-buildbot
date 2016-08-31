// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tsmon

import (
	"errors"
	"sync"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/tsmon/monitor"
	"github.com/luci/luci-go/common/tsmon/store"
	"github.com/luci/luci-go/common/tsmon/target"
	"github.com/luci/luci-go/common/tsmon/types"
)

// State holds the configuration of the tsmon library.  There is one global
// instance of State, but it can be overridden in a Context by tests.
type State struct {
	S       store.Store
	M       monitor.Monitor
	Flusher *autoFlusher

	RegisteredMetrics     map[string]types.Metric
	RegisteredMetricsLock sync.RWMutex

	CallbacksMutex  sync.RWMutex
	Callbacks       []Callback
	GlobalCallbacks []GlobalCallback
}

// SetStore changes the metric store.  All metrics that were registered with
// the old store will be re-registered on the new store.
func (state *State) SetStore(s store.Store) {
	oldStore := state.S
	if s == oldStore {
		return
	}

	state.RegisteredMetricsLock.RLock()
	defer state.RegisteredMetricsLock.RUnlock()

	// Register metrics on the new store.
	for _, m := range state.RegisteredMetrics {
		s.Register(m)
	}

	state.S = s

	// Unregister metrics from the old store.
	if oldStore != nil {
		for _, m := range state.RegisteredMetrics {
			oldStore.Unregister(m)
		}
	}
}

// ResetCumulativeMetrics resets only cumulative metrics.
func (state *State) ResetCumulativeMetrics(c context.Context) {
	state.RegisteredMetricsLock.RLock()
	defer state.RegisteredMetricsLock.RUnlock()

	for _, m := range state.RegisteredMetrics {
		if m.Info().ValueType.IsCumulative() {
			state.S.Reset(c, m)
		}
	}
}

// RunGlobalCallbacks runs all registered global callbacks that produce global
// metrics.
//
// See RegisterGlobalCallback for more info.
func (state *State) RunGlobalCallbacks(c context.Context) {
	state.CallbacksMutex.RLock()
	defer state.CallbacksMutex.RUnlock()

	for _, gcp := range state.GlobalCallbacks {
		gcp.Callback(c)
	}
}

// ResetGlobalCallbackMetrics resets metrics produced by global callbacks.
//
// See RegisterGlobalCallback for more info.
func (state *State) ResetGlobalCallbackMetrics(c context.Context) {
	state.CallbacksMutex.RLock()
	defer state.CallbacksMutex.RUnlock()

	for _, gcp := range state.GlobalCallbacks {
		for _, m := range gcp.Metrics {
			state.S.Reset(c, m)
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Flush sends all the metrics that are registered in the application.
//
// Uses given monitor if not nil, or the state.M otherwise.
func (state *State) Flush(c context.Context, mon monitor.Monitor) error {
	if mon == nil {
		mon = state.M
	}
	if mon == nil {
		return errors.New("no tsmon Monitor is configured")
	}

	// Run any callbacks that have been registered to populate values in callback
	// metrics.
	state.runCallbacks(c)

	cells := state.S.GetAll(c)
	if len(cells) == 0 {
		return nil
	}

	logging.Debugf(c, "Starting tsmon flush: %d cells", len(cells))
	defer logging.Debugf(c, "Tsmon flush finished")

	// Split up the payload into chunks if there are too many cells.
	chunkSize := mon.ChunkSize()
	if chunkSize == 0 {
		chunkSize = len(cells)
	}

	total := len(cells)
	sent := 0
	for len(cells) > 0 {
		count := minInt(chunkSize, len(cells))
		if err := mon.Send(c, cells[:count]); err != nil {
			logging.Errorf(
				c, "Sent %d cells out of %d, skipping the rest due to error - %s",
				sent, total, err)
			return err
		}
		cells = cells[count:]
		sent += count
	}
	return nil
}

// runCallbacks runs any callbacks that have been registered to populate values
// in callback metrics.
func (state *State) runCallbacks(c context.Context) {
	state.CallbacksMutex.RLock()
	defer state.CallbacksMutex.RUnlock()

	for _, f := range state.Callbacks {
		f(c)
	}
}

// GetState returns the State instance held in the context (if set) or else
// returns the global state.
func GetState(c context.Context) *State {
	if ret := c.Value(stateKey); ret != nil {
		return ret.(*State)
	}
	return globalState
}

// WithState returns a new context holding the given State instance.
func WithState(c context.Context, s *State) context.Context {
	return context.WithValue(c, stateKey, s)
}

// WithFakes returns a new context holding a new State with a fake store and a
// fake monitor.
func WithFakes(c context.Context) (context.Context, *store.Fake, *monitor.Fake) {
	s := &store.Fake{}
	m := &monitor.Fake{}
	return WithState(c, &State{
		S:                 s,
		M:                 m,
		RegisteredMetrics: map[string]types.Metric{},
	}), s, m
}

// WithDummyInMemory returns a new context holding a new State with a new in-
// memory store and a fake monitor.
func WithDummyInMemory(c context.Context) (context.Context, *monitor.Fake) {
	s := store.NewInMemory(&target.Task{ServiceName: proto.String("")})
	m := &monitor.Fake{}
	return WithState(c, &State{
		S:                 s,
		M:                 m,
		RegisteredMetrics: map[string]types.Metric{},
	}), m
}

type key int

var (
	globalState     = NewState()
	stateKey    key = 1
)

// NewState returns a new default State, configured with a nil Store and
// Monitor.
func NewState() *State {
	return &State{
		S:                 store.NewNilStore(),
		M:                 monitor.NewNilMonitor(),
		RegisteredMetrics: map[string]types.Metric{},
	}
}
