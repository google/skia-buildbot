// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

//go:generate cproto -import-path=minter

// Package minter contains the main API of the token server.
//
// It is publicly accessible API used to mint various kinds of tokens.
package minter
