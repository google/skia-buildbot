// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package secrets provides an interface for a simple secret store: you ask it
// for a secret (a byte blob, identifies by some key), and it returns it
// to you (current version, as well as a bunch of previous versions). Caller are
// supposed to use the secret for an operation and then forget it (e.g. do not
// try to store it elsewhere).
//
// Secure storage, retrieval and rotation of secrets is outside of the scope of
// this interface: it's the responsibility of the implementation.
package secrets
