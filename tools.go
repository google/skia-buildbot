//go:build tools
// +build tools

package main

// This file exists to ensure that tool dependencies (eg. goimports) which are
// not directly imported in our code actually get included in the go.mod file.
// For more information see the discussion on:
// https://github.com/golang/go/issues/25922

import (
	_ "github.com/golang/protobuf/protoc-gen-go"
	_ "github.com/google/go-licenses/v2"
	_ "github.com/kisielk/errcheck"
	_ "github.com/skia-dev/protoc-gen-twirp_typescript"
	_ "github.com/temporalio/cli/cmd/temporal"
	_ "github.com/temporalio/ui-server/v2/server"
	_ "github.com/twitchtv/twirp/protoc-gen-twirp"
	_ "github.com/vektra/mockery/v2"
	_ "go.temporal.io/server/cmd/server"
	_ "golang.org/x/tools/cmd/goimports"
)
