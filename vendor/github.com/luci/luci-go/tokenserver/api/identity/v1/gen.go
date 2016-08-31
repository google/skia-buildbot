// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

//go:generate cproto -import-path=identity

// Package identity contains the an API to retrieve callers identity.
//
// It exercises exact same authentication paths as regular services. Useful for
// debugging access tokens.
package identity
