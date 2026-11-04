// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Generate the go code from the protocol buffer definitions.
//go:generate bazelisk run --config=mayberemote //:protoc -- --go_opt=module=go.skia.org/infra/perf/go/autobisection/proto/v1 --go_out=. --go-grpc_opt=module=go.skia.org/infra/perf/go/autobisection/proto/v1 --go-grpc_out=.  -I . ./autobisection_service.proto
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w autobisection_service.pb.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w autobisection_service_grpc.pb.go

package v1
