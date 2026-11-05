// Copyright 2023 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Generate the go code from the protocol buffer definitions.
//
//go:generate bazelisk run --config=mayberemote //:protoc -- --go_opt=module=go.skia.org/infra/perf/go/sheriffconfig/proto/v1 --go_out=. -I . ./sheriff_config.proto
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w sheriff_config.pb.go
package v1
