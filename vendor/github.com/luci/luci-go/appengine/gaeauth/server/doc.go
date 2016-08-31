// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package server implements authentication for inbound HTTP requests on GAE.
// It provides adapters for GAE Users and OAuth2 APIs to make them usable by
// server/auth package.
//
// It also provides GAE-specific implementation of some other interface used
// by server/auth package, such as SessionStore.
package server
