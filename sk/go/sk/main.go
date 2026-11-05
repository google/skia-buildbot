// Copyright 2021 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"github.com/urfave/cli/v2"

	"go.skia.org/infra/sk/go/agent"
	"go.skia.org/infra/sk/go/asset"
	release_branch "go.skia.org/infra/sk/go/release-branch"
	"go.skia.org/infra/sk/go/try"
)

func main() {
	app := &cli.App{
		Name:        "sk",
		Description: `sk provides developer workflow tools for Skia.`,
		Commands: append([]*cli.Command{
			agent.Command(),
			asset.Command(),
			release_branch.Command(),
		}, try.Command()...),
		Usage: "sk <subcommand>",
	}
	app.RunAndExitOnError()
}
