// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/tsmon/distribution"
	"github.com/luci/luci-go/common/tsmon/field"
	"github.com/luci/luci-go/common/tsmon/target"
	"github.com/luci/luci-go/common/tsmon/types"
	"golang.org/x/net/context"
)

type inMemoryStore struct {
	defaultTarget     types.Target
	defaultTargetLock sync.RWMutex

	data     map[string]*metricData
	dataLock sync.RWMutex
}

type cellKey struct {
	fieldValuesHash, targetHash uint64
}

type metricData struct {
	types.MetricInfo
	types.MetricMetadata

	cells map[cellKey][]*types.CellData
	lock  sync.Mutex
}

func (m *metricData) get(fieldVals []interface{}, t types.Target, resetTime time.Time) (*types.CellData, error) {
	fieldVals, err := field.Canonicalize(m.Fields, fieldVals)
	if err != nil {
		return nil, err
	}

	key := cellKey{fieldValuesHash: field.Hash(fieldVals)}
	if t != nil {
		key.targetHash = t.Hash()
	}

	cells, ok := m.cells[key]
	if ok {
		for _, cell := range cells {
			if reflect.DeepEqual(fieldVals, cell.FieldVals) &&
				reflect.DeepEqual(t, cell.Target) {
				return cell, nil
			}
		}
	}

	cell := &types.CellData{fieldVals, t, resetTime, nil}
	m.cells[key] = append(cells, cell)
	return cell, nil
}

// NewInMemory creates a new metric store that holds metric data in this
// process' memory.
func NewInMemory(defaultTarget types.Target) Store {
	return &inMemoryStore{
		defaultTarget: defaultTarget,
		data:          map[string]*metricData{},
	}
}

// Register does nothing.
func (s *inMemoryStore) Register(m types.Metric) {}

// Unregister removes the metric (along with all its values) from the store.
func (s *inMemoryStore) Unregister(h types.Metric) {
	s.dataLock.Lock()
	defer s.dataLock.Unlock()

	delete(s.data, h.Info().Name)
}

func (s *inMemoryStore) getOrCreateData(m types.Metric) *metricData {
	s.dataLock.RLock()
	d, ok := s.data[m.Info().Name]
	s.dataLock.RUnlock()
	if ok {
		return d
	}

	s.dataLock.Lock()
	defer s.dataLock.Unlock()

	// Check again in case another goroutine got the lock before us.
	if d, ok := s.data[m.Info().Name]; ok {
		return d
	}

	d = &metricData{
		MetricInfo: m.Info(),
		cells:      map[cellKey][]*types.CellData{},
	}

	s.data[m.Info().Name] = d
	return d
}

func (s *inMemoryStore) DefaultTarget() types.Target {
	s.defaultTargetLock.RLock()
	defer s.defaultTargetLock.RUnlock()

	return s.defaultTarget.Clone()
}

func (s *inMemoryStore) SetDefaultTarget(t types.Target) {
	s.defaultTargetLock.Lock()
	defer s.defaultTargetLock.Unlock()

	s.defaultTarget = t
}

// Get returns the value for a given metric cell.
func (s *inMemoryStore) Get(ctx context.Context, h types.Metric, resetTime time.Time, fieldVals []interface{}) (value interface{}, err error) {
	if resetTime.IsZero() {
		resetTime = clock.Now(ctx)
	}

	m := s.getOrCreateData(h)
	m.lock.Lock()
	defer m.lock.Unlock()

	c, err := m.get(fieldVals, target.Get(ctx), resetTime)
	if err != nil {
		return nil, err
	}

	return c.Value, nil
}

// Set writes the value into the given metric cell.
func (s *inMemoryStore) Set(ctx context.Context, h types.Metric, resetTime time.Time, fieldVals []interface{}, value interface{}) error {
	if resetTime.IsZero() {
		resetTime = clock.Now(ctx)
	}
	return s.set(h, resetTime, fieldVals, target.Get(ctx), value)
}

func isLessThan(a, b interface{}) bool {
	if a == nil || b == nil {
		return false
	}
	switch a.(type) {
	case int64:
		return a.(int64) < b.(int64)
	case float64:
		return a.(float64) < b.(float64)
	}
	return false
}

func (s *inMemoryStore) set(h types.Metric, resetTime time.Time, fieldVals []interface{}, t types.Target, value interface{}) error {
	m := s.getOrCreateData(h)
	m.lock.Lock()
	defer m.lock.Unlock()

	c, err := m.get(fieldVals, t, resetTime)
	if err != nil {
		return err
	}

	if m.ValueType.IsCumulative() && isLessThan(value, c.Value) {
		return fmt.Errorf("attempted to set cumulative metric %s to %v, which is lower than the previous value %v",
			h.Info().Name, value, c.Value)
	}

	c.Value = value
	return nil
}

// Incr increments the value in a given metric cell by the given delta.
func (s *inMemoryStore) Incr(ctx context.Context, h types.Metric, resetTime time.Time, fieldVals []interface{}, delta interface{}) error {
	if resetTime.IsZero() {
		resetTime = clock.Now(ctx)
	}
	return s.incr(h, resetTime, fieldVals, target.Get(ctx), delta)
}

func (s *inMemoryStore) incr(h types.Metric, resetTime time.Time, fieldVals []interface{}, t types.Target, delta interface{}) error {
	m := s.getOrCreateData(h)
	m.lock.Lock()
	defer m.lock.Unlock()

	c, err := m.get(fieldVals, t, resetTime)
	if err != nil {
		return err
	}

	switch m.ValueType {
	case types.CumulativeDistributionType:
		d, ok := delta.(float64)
		if !ok {
			return fmt.Errorf("Incr got a delta of unsupported type (%v)", delta)
		}
		v, ok := c.Value.(*distribution.Distribution)
		if !ok {
			v = distribution.New(h.(types.DistributionMetric).Bucketer())
			c.Value = v
		}
		v.Add(float64(d))

	case types.CumulativeIntType:
		if v, ok := c.Value.(int64); ok {
			c.Value = v + delta.(int64)
		} else {
			c.Value = delta.(int64)
		}

	case types.CumulativeFloatType:
		if v, ok := c.Value.(float64); ok {
			c.Value = v + delta.(float64)
		} else {
			c.Value = delta.(float64)
		}

	default:
		return fmt.Errorf("attempted to increment non-cumulative metric %s by %v",
			m.Name, delta)
	}

	return nil
}

// GetAll efficiently returns all cells in the store.
func (s *inMemoryStore) GetAll(ctx context.Context) []types.Cell {
	s.dataLock.Lock()
	defer s.dataLock.Unlock()

	defaultTarget := s.DefaultTarget()

	ret := []types.Cell{}
	for _, m := range s.data {
		m.lock.Lock()
		for _, cells := range m.cells {
			for _, cell := range cells {
				// Add the default target to the cell if it doesn't have one set.
				cellCopy := *cell
				if cellCopy.Target == nil {
					cellCopy.Target = defaultTarget
				}
				ret = append(ret, types.Cell{m.MetricInfo, m.MetricMetadata, cellCopy})
			}
		}
		m.lock.Unlock()
	}
	return ret
}

func (s *inMemoryStore) Reset(ctx context.Context, h types.Metric) {
	m := s.getOrCreateData(h)

	m.lock.Lock()
	m.cells = make(map[cellKey][]*types.CellData)
	m.lock.Unlock()
}
