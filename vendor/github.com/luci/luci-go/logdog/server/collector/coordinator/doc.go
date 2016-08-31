// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package coordinator implements a minimal interface to the Coordinator service
// that is sufficient for Collector usage.
//
// The interface also serves as an API abstraction boundary between the current
// Coordinator service definition and the Collector's logic.
//
// Cache
//
// Coordinator methods are called very heavily during Collector operation. In
// production, the Coordinator instance should be wrapped in a Cache structure
// to locally cache Coordinator's known state.
//
// The cache is responsible for two things: Firstly, it coalesces multiple
// pending requests for the same stream state into a single Coordinator request.
// Secondly, it maintains a cache of completed responses to short-circuit the
// Coordinator.
//
// Stream state is stored internally as a Promise. This Promise is evaluated by
// querying the Coordinator. This interface is hidden to callers.
package coordinator
