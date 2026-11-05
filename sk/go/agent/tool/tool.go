// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tool

import (
	"github.com/urfave/cli/v2"
	"go.skia.org/infra/go/skerr"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:            "tool",
		Usage:           "Run an AI agent tool",
		SkipFlagParsing: true,
		Action: func(c *cli.Context) error {
			// Derive the subcommands from MCP server tools. We do this inside
			// of Action to prevent slow startup and unexpected behavior when
			// other commands are used, since communication with the MCP server
			// may be slow or fail altogether.
			subcommands, err := createCommandsForMCPTools(c.Context)
			if err != nil {
				return skerr.Wrapf(err, "failed to retrieve MCP server tools")
			}
			app := &cli.App{
				Name:            "sk agent tool",
				Usage:           "Run an AI agent tool",
				Commands:        subcommands,
				Reader:          c.App.Reader,
				Writer:          c.App.Writer,
				ErrWriter:       c.App.ErrWriter,
				HideHelpCommand: true,
			}
			return app.RunContext(c.Context, append([]string{"sk agent tool"}, c.Args().Slice()...))
		},
	}
}
