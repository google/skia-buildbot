// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package monitor contains the code for sending metric data to monitoring
// endpoints.
package monitor

import (
	"github.com/luci/luci-go/common/tsmon/types"
	"golang.org/x/net/context"
)

// A Monitor can send a collection of metrics somewhere.
type Monitor interface {
	// ChunkSize returns the maximum number of metrics this Monitor can Send in
	// one go.  If there are more metrics, they will be broken up into chunks and
	// Send will be called multiple times.  A ChunkSize of 0 disables chunking.
	ChunkSize() int

	Send(ctx context.Context, cells []types.Cell) error
}
