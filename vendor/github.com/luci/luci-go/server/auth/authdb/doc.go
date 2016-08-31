// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package authdb contains definition of Authentication Database (aka AuthDB).
//
// Authentication Database represents all data used when authorizing incoming
// requests and handling authentication related tasks: user groups, IP
// whitelists, OAuth client ID whitelist, etc.
//
// This package defines a general interface and few its implementations.
package authdb
