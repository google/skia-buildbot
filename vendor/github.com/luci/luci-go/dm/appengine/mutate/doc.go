// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package mutate includes the main logic of DM's state machine. The package
// is a series of "github.com/luci/luci-go/tumble".Mutation implementations.
// Each mutation operates on a single entity group in DM's datastore model,
// advancing the state machine for the dependency graph by one edge.
package mutate
