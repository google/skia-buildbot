// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"github.com/luci/luci-go/common/tsmon/types"
	"golang.org/x/net/context"
)

// Fake is a fake Monitor.
type Fake struct {
	CS    int
	Cells [][]types.Cell
}

// ChunkSize returns the fake value.
func (m *Fake) ChunkSize() int {
	return m.CS
}

// Send appends the cells to Cells.
func (m *Fake) Send(c context.Context, cells []types.Cell) error {
	m.Cells = append(m.Cells, cells)
	return nil
}
