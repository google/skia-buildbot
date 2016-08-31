// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package main hosts the Cloud Endpoints API generator utility.
//
// This utility is intended to be used with `go generate` to automatically
// generate Cloud Endpoints Go stubs. It is closely tired to the
// `google-api-go-generator` tool (
// https://github.com/google/google-api-go-client).
//
// `apigen` detects a methods of operation by parsing its target's `app.yaml`
// file. It configures and executes a Cloud Endpoints frontend server for the
// target application, then runs `google-api-go-generator` against it. The
// resulting APIs are aggregated in a Go package destination.
//
// The generated APIs are also patched as they are installed in order to add
// LUCI-Go-conforming structure.
//
// For more information, see the `example/...` subdirectory, which contains
// example AppEngine and Managed VM services and their generated stubs.
package main
