// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package infra

import (
	"github.com/urfave/cli/v2"
)

// Command returns a cli.Command instance which represents the "infra" command.
func Command() *cli.Command {
	return &cli.Command{
		Name:        "infra",
		Description: "Subcommands for interacting with Skia infrastructure.",
		Usage:       "infra <subcommand>",
		Subcommands: []*cli.Command{
			deploymentStatusCommand(),
		},
	}
}
