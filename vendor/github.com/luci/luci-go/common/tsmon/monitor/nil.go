// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"github.com/luci/luci-go/common/tsmon/types"
	"golang.org/x/net/context"
)

type nilMonitor struct{}

// NewNilMonitor returns a Monitor that does nothing.
func NewNilMonitor() Monitor {
	return &nilMonitor{}
}

func (m *nilMonitor) ChunkSize() int {
	return 0
}

func (m *nilMonitor) Send(ctx context.Context, cells []types.Cell) error {
	return nil
}
