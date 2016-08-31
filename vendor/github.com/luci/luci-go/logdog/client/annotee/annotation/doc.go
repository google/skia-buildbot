// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package annotation implements a state machine that constructs Milo annotation
// protobufs from a series of annotation commands.
//
// The annotation state is represented by a State object. Annotation strings are
// appended to the State via Append(), causing the State to incorporate that
// annotation and advance. During state advancement, any number of the State's
// Callbacks may be invoked in response to changes that are made.
//
// State is pure (not bound to any I/O). Users of a State should interact with
// it by implementing Callbacks.
package annotation
