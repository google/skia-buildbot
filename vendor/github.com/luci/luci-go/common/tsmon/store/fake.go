// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package store

import (
	"time"

	"github.com/luci/luci-go/common/tsmon/types"
	"golang.org/x/net/context"
)

// Fake is a fake Store.
type Fake struct {
	Cells []types.Cell
	DT    types.Target
}

// Register does nothing.
func (s *Fake) Register(types.Metric) {}

// Unregister does nothing.
func (s *Fake) Unregister(types.Metric) {}

// DefaultTarget returns DT.
func (s *Fake) DefaultTarget() types.Target { return s.DT }

// SetDefaultTarget does nothing.
func (s *Fake) SetDefaultTarget(types.Target) {}

// Get does nothing.
func (s *Fake) Get(context.Context, types.Metric, time.Time, []interface{}) (interface{}, error) {
	return nil, nil
}

// Set does nothing.
func (s *Fake) Set(context.Context, types.Metric, time.Time, []interface{}, interface{}) error {
	return nil
}

// Incr does nothing.
func (s *Fake) Incr(context.Context, types.Metric, time.Time, []interface{}, interface{}) error {
	return nil
}

// GetAll returns the pre-set list of cells.
func (s *Fake) GetAll(context.Context) []types.Cell { return s.Cells }

// ResetForUnittest does nothing.
func (s *Fake) ResetForUnittest() {}

// Reset does nothing.
func (s *Fake) Reset(context.Context, types.Metric) {}
