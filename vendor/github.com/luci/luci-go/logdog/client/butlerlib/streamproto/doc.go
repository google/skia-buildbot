// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package streamproto describes the protocol primitives used by LogDog/Butler
// for stream negotiation.
//
// A LogDog Butler client wishing to create a new LogDog stream can use the
// Flags type to configure/send the stream.
//
// Internally, LogDog represents the Flags properties with the Properties type.
package streamproto
