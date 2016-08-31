// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package delegation contains low-level API for working with delegation tokens.
//
// It provides convenient wrappers around Auth Service API for creating the
// tokens and related proto messages.
//
// If you don't know what it is, or why you would use it, you probably don't
// need it.
//
// Prefer the high-level API in server/auth package, in particular
// `MintDelegationToken` and `auth.GetRPCTransport(ctx, auth.AsUser)`.
package delegation
