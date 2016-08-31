// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"github.com/maruel/subcommands"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/cli"
)

var cmdFmt = &subcommands.Command{
	UsageLine: `fmt subcommand [arguments]`,
	ShortDesc: "converts a message to/from flagpb and JSON formats",
	LongDesc:  "Converts a message to/from flagpb and JSON formats.",
	CommandRun: func() subcommands.CommandRun {
		c := &fmtRun{}
		return c
	},
}

type fmtRun struct {
	cmdRun
}

func (r *fmtRun) Run(a subcommands.Application, args []string) int {
	app := &cli.Application{
		Name: "fmt",
		Context: func(context.Context) context.Context {
			return cli.GetContext(a, r)
		},
		Title: "Converts a message formats.",
		Commands: []*subcommands.Command{
			cmdJ2F,
			cmdF2J,
		},
	}
	return subcommands.Run(app, args)
}
