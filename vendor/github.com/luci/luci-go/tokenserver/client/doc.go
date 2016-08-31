// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package client implements pRPC client for The Token Server.
//
// It is a thin wrapper around regular pRPC client, that knows how to use
// private keys and certificates to properly prepare requests for the token
// server.
package client
