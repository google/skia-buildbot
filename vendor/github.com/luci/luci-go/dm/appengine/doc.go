// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package appengine provides the appengine service implementation for DM.
//
// This contains the following subpackages:
//   model - These objects are the datastore model objects for DM.
//   mutate - Tumble mutations for DM, a.k.a. DM's state machine. Each mutation
//     represents a single node in DM's state machine.
//   deps - The dependency management pRPC service.
//   frontend - The deployable appengine app. For Technical Reasons (tm), almost
//     zero code lives here, it just calls through to code in deps.
//   distributor - Definition of the Distributor interface, and implementations
//     (such as swarming_v1).
//
// For more information on DM itself, check out https://github.com/luci/luci-go/wiki/Design-Documents
package appengine
