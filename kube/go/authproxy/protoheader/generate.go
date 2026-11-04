// Copyright 2022 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package protoheader

//go:generate bazelisk run --config=mayberemote //:protoc -- --go_opt=paths=source_relative --go_out=. ./header.proto
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w header.pb.go
